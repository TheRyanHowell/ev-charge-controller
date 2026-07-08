package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"
)

// TariffProvider resolves the electricity tariff used to bill a user's session.
type TariffProvider interface {
	EffectiveTariffForUser(ctx context.Context, userID string) (models.TariffSettings, error)
}

// SessionLifecycleService manages session lifecycle: creation, activation,
// cancellation, and completion of charge sessions.
type SessionLifecycleService struct {
	sessionReader     internal.SessionReader
	sessionWriter     internal.SessionWriter
	vehicleRepo       internal.VehicleRepo
	plugRepo          internal.PlugRepo
	plugCtrl          internal.PlugController
	powerReadingStats internal.PowerReadingStats
	tariffProvider    TariffProvider
	socGenerator      *SOCGenerator
	notifier          *ChargeNotifier
	lock              *sessionLock
}

// SetTariffProvider wires the tariff provider used to freeze a session's cost at
// completion. When unset, completed sessions persist a zero cost.
func (s *SessionLifecycleService) SetTariffProvider(p TariffProvider) {
	s.tariffProvider = p
}

// NewSessionLifecycleService creates a new SessionLifecycleService.
// The session lock is shared with ChargeSessionService and
// SessionMonitoringService to serialize lifecycle mutations with monitoring.
func NewSessionLifecycleService(
	sessionReader internal.SessionReader,
	sessionWriter internal.SessionWriter,
	vehicleRepo internal.VehicleRepo,
	plugRepo internal.PlugRepo,
	plugCtrl internal.PlugController,
	powerReadingStats internal.PowerReadingStats,
	notifier *ChargeNotifier,
	lock *sessionLock,
) *SessionLifecycleService {
	return &SessionLifecycleService{
		sessionReader:     sessionReader,
		sessionWriter:     sessionWriter,
		vehicleRepo:       vehicleRepo,
		plugRepo:          plugRepo,
		plugCtrl:          plugCtrl,
		powerReadingStats: powerReadingStats,
		socGenerator:      NewSOCGenerator(),
		notifier:          notifier,
		lock:              lock,
	}
}

// StartSession creates a charge session and blocks on MQTT power confirmation.
// Creates the pending session in DB first, then blocks on MQTT power confirmation.
// On success, transitions pending→active inline. On failure, cancels the pending
// session and best-effort turns the plug OFF.
func (s *SessionLifecycleService) StartSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent float64) (*models.ChargeSession, error) {
	return s.startSession(ctx, plugID, vehicleID, startPercent, targetPercent, nil, nil)
}

// StartTwoStageSession starts a ready-by two-stage session: charges to holdPercent,
// holds there, then resumes to reach targetPercent by readyByTime. Shares the same
// pending→MQTT-confirm→active flow as StartSession.
func (s *SessionLifecycleService) StartTwoStageSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent, holdPercent float64, readyByTime string) (*models.ChargeSession, error) {
	return s.startSession(ctx, plugID, vehicleID, startPercent, targetPercent, &holdPercent, &readyByTime)
}

func (s *SessionLifecycleService) startSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent float64, holdPercent *float64, readyByTime *string) (*models.ChargeSession, error) {
	if err := s.validateVehicleExists(ctx, vehicleID); err != nil {
		return nil, err
	}
	if err := s.validatePlugOwnership(ctx, plugID); err != nil {
		return nil, err
	}

	// Capture cached MQTT energy baseline before acquiring mutex.
	startTotalKwh := s.captureEnergyBaseline(ctx, plugID)

	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.ensureNoActiveSessionForPlug(ctx, plugID); err != nil {
		return nil, err
	}

	// Create pending session in DB FIRST so it survives browser refresh or API crash.
	session, err := s.createSessionFromPercent(ctx, plugID, vehicleID, startPercent, targetPercent, startTotalKwh, holdPercent, readyByTime)
	if err != nil {
		return nil, err
	}

	// Block on MQTT power confirmation. If confirmed, transition to active inline.
	// If timeout/failure, cancel the pending session and best-effort cut power.
	// Use session.PlugID (resolved by createSessionFromPercent, may have fallen back).
	if s.plugCtrl != nil && session.PlugID != nil {
		confirmed, confErr := s.plugCtrl.SetPowerAndWait(ctx, *session.PlugID, true, models.PowerConfirmationTimeout)
		if confirmed {
			if actErr := s.sessionWriter.ActivatePending(ctx, session.ID, time.Now()); actErr != nil {
				slog.Warn("StartSession: activation failed after power confirmation", "sessionID", session.ID, "err", actErr)
			} else {
				session.Status = models.SessionStatusActive
				session.StartedAt = &session.CreatedAt
				s.sendChargeStartedNotification(ctx, session)
			}
			return session, nil
		}

		// Power confirmation failed - cancel pending session and cut power.
		slog.Warn("StartSession: power confirmation failed", "sessionID", session.ID, "err", confErr)
		if cancelErr := s.sessionWriter.CancelPending(ctx, session.ID, time.Now()); cancelErr != nil {
			slog.Error("StartSession: failed to cancel pending session after power timeout", "sessionID", session.ID, "err", cancelErr)
		}
		// Best-effort: turn plug OFF in case relay is stuck ON.
		if _, offErr := s.plugCtrl.SetPowerAndWait(ctx, *session.PlugID, false, models.PowerConfirmationBestEffortTimeout); offErr != nil {
			slog.Warn("StartSession: best-effort power-off failed", "plugID", *session.PlugID, "err", offErr)
		}
		return nil, fmt.Errorf("power confirmation failed: %w", confErr)
	}

	return session, nil
}

// ActivatePending transitions a pending session to active.
func (s *SessionLifecycleService) ActivatePending(ctx context.Context, sessionID string) (float64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	session, err := s.sessionReader.FindByID(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	if session == nil {
		return 0, ErrSessionNotFound
	}
	if session.Status != models.SessionStatusPending {
		return 0, nil
	}

	// Capture fresh energy baseline at activation to avoid counting
	// energy accumulated during the pending phase
	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && session.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*session.PlugID)
	}
	if energy != nil {
		startTotalStr := "nil"
		if session.StartTotalKwh != nil {
			startTotalStr = fmt.Sprintf("%f", *session.StartTotalKwh)
		}
		slog.Info("[ACTIVATE] Session", "sessionID", sessionID, "power", energy.Power, "total", energy.Total, "startTotalKwh", startTotalStr)
		var startTotalKwh *float64
		if session.StartTotalKwh != nil && *session.StartTotalKwh > 0 {
			startTotalKwh = &energy.Total
		}
		if startTotalKwh != nil {
			if err := s.sessionWriter.UpdateStartTotalKwh(ctx, sessionID, *startTotalKwh); err != nil {
				return 0, err
			}
		}
	}

	// Atomic state transition with WHERE-clause guard
	if err := s.sessionWriter.ActivatePending(ctx, sessionID, time.Now()); err != nil {
		return 0, err
	}

	// Return the new baseline for the caller to update the session object
	if energy != nil {
		return energy.Total, nil
	}
	return 0, nil
}

// CancelPending transitions a pending session to cancelled.
func (s *SessionLifecycleService) CancelPending(ctx context.Context, sessionID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.sessionWriter.CancelPending(ctx, sessionID, time.Now()); errors.Is(err, repository.ErrSessionWrongState) {
		return ErrSessionNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// CancelPendingIfTimedOut atomically checks for a pending session that has
// exceeded the given timeout and cancels it. The mutex is held for the entire
// operation to prevent races with SaveEnergyReadings and other mutations.
// Power is cut for the plug before returning. Returns true if a session was cancelled.
func (s *SessionLifecycleService) CancelPendingIfTimedOut(ctx context.Context, timeout time.Duration) (bool, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	pendingSession, err := s.sessionReader.GetPending(ctx)
	if err != nil || pendingSession == nil {
		return false, nil
	}

	if time.Since(pendingSession.CreatedAt) <= timeout {
		return false, nil
	}

	// Save plugID before cancelling so we can cut power after the DB write.
	plugID := ""
	if pendingSession.PlugID != nil {
		plugID = *pendingSession.PlugID
	}

	slog.Info("Pending session timed out, cancelling", "session_id", pendingSession.ID, "timeout", timeout)
	if err := s.sessionWriter.CancelPending(ctx, pendingSession.ID, time.Now()); errors.Is(err, repository.ErrSessionWrongState) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if s.plugCtrl != nil && plugID != "" {
		if err := s.plugCtrl.SetPower(ctx, plugID, false); err != nil {
			slog.Warn("CancelPendingIfTimedOut: failed to cut power", "plugID", plugID, "err", err)
		}
	}

	return true, nil
}

// Stop stops the active charge session, calculating endPercent from Tasmota energy.
// If session is pending, it will be cancelled instead of completed.
func (s *SessionLifecycleService) Stop(ctx context.Context, activeSession *models.ChargeSessionView) (*StopResult, error) {
	if activeSession == nil {
		return nil, ErrSessionNotFound
	}

	// Pending sessions are cancelled, not completed
	if activeSession.Status == models.SessionStatusPending {
		return s.cancelPendingSession(ctx, &activeSession.ChargeSession)
	}

	endPercent := s.calculateEndPercent(ctx, activeSession)

	return s.stopWithPercent(ctx, &activeSession.ChargeSession, endPercent)
}

// StopWithPercent stops a specific session by ID.
// For use from handlers and tests where the session hasn't been pre-fetched.
func (s *SessionLifecycleService) StopWithPercent(ctx context.Context, id string, endPercent float64) (*StopResult, error) {
	session, err := s.sessionReader.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	// Pending sessions are cancelled, not completed
	if session.Status == models.SessionStatusPending {
		return s.cancelPendingSession(ctx, session)
	}

	return s.stopWithPercent(ctx, session, endPercent)
}

// stopWithPercent stops a specific session using the already-read session object.
// The caller must have already fetched the session to avoid TOCTOU races.
// reason controls whether a completion notification is sent: only StopAutoComplete fires one.
func (s *SessionLifecycleService) stopWithPercent(ctx context.Context, session *models.ChargeSession, endPercent float64, reason ...models.StopReason) (*StopResult, error) {
	result := &StopResult{Stopped: true}
	_ = reason // StopReason carried for context; notification always fires via stopWithPercent
	endTime := time.Now()
	endKwh, actualEndPercent := s.calculateEndData(ctx, session, endPercent)

	// Compute session stats before acquiring lock
	batteryKwh := endKwh - session.StartKwh
	if batteryKwh < 0 {
		batteryKwh = 0
	}
	efficiency := s.getChargingEfficiency(ctx, session.VehicleID)
	wallKwh := batteryKwh / efficiency
	avgCarbon, co2Grams := s.getSessionCarbon(ctx, session.ID, wallKwh)
	costPence, offPeakKwh := s.computeSessionCost(ctx, session)

	s.lock.Lock()
	if err := s.sessionWriter.UpdateEndWithStats(ctx, session.ID, endTime, endKwh, actualEndPercent, batteryKwh, wallKwh, co2Grams, avgCarbon, costPence, offPeakKwh); err != nil {
		s.lock.Unlock()
		return nil, err
	}
	s.lock.Unlock()

	if s.plugCtrl != nil && session.PlugID != nil {
		confirmed, confErr := s.plugCtrl.SetPowerAndWait(ctx, *session.PlugID, false, models.PowerConfirmationTimeout)
		if !confirmed {
			result.Stopped = false
			result.TasmotaErr = confErr.Error()
			slog.Warn("Stop: power-off not confirmed", "sessionID", session.ID, "err", confErr)
		}
	}

	if updateErr := s.vehicleRepo.UpdatePercents(ctx, session.VehicleID, actualEndPercent, session.TargetPercent); updateErr != nil {
		slog.Warn("failed to update vehicle percents after charge stop", "err", updateErr)
	}

	// Best-effort increment of vehicle lifetime stats
	if incErr := s.vehicleRepo.IncrementLifetimeStats(ctx, session.VehicleID, batteryKwh, wallKwh, co2Grams, costPence, session.CreatedAt); incErr != nil {
		slog.Warn("failed to increment vehicle lifetime stats after charge stop", "err", incErr)
	}

	s.sendChargeCompleteNotification(ctx, session, actualEndPercent)

	return result, nil
}

// getChargingEfficiency returns the charging efficiency for a vehicle.
// Falls back to the default if the vehicle or efficiency can't be found.
func (s *SessionLifecycleService) getChargingEfficiency(ctx context.Context, vehicleID string) float64 {
	vehicle, err := s.vehicleRepo.FindByID(ctx, vehicleID)
	if err != nil || vehicle == nil {
		return models.DefaultChargingEfficiency
	}
	if vehicle.ChargingEfficiency <= 0 {
		return models.DefaultChargingEfficiency
	}
	return vehicle.ChargingEfficiency
}

// getSessionCarbon looks up the average carbon intensity for a session
// and computes the CO2 grams from wall energy.
func (s *SessionLifecycleService) getSessionCarbon(ctx context.Context, sessionID string, wallKwh float64) (*float64, float64) {
	if s.powerReadingStats == nil || wallKwh == 0 {
		return nil, 0
	}
	carbonMap, err := s.powerReadingStats.GetAvgCarbonIntensityForSessions(ctx, []string{sessionID})
	if err != nil {
		slog.Warn("failed to get carbon intensity for session stats", "sessionID", sessionID, "err", err)
		return nil, 0
	}
	avgCarbon, ok := carbonMap[sessionID]
	if !ok || avgCarbon == nil {
		return nil, 0
	}
	return avgCarbon, wallKwh * *avgCarbon
}

// computeSessionCost freezes the session's electricity cost at completion: it
// time-weights the wall-side energy across the user's tariff windows. Returns
// (0, 0) when the tariff, energy baseline, or readings are unavailable.
func (s *SessionLifecycleService) computeSessionCost(ctx context.Context, session *models.ChargeSession) (costPence, offPeakKwh float64) {
	if s.tariffProvider == nil || session.UserID == nil || session.StartTotalKwh == nil {
		return 0, 0
	}
	tariff, err := s.tariffProvider.EffectiveTariffForUser(ctx, *session.UserID)
	if err != nil {
		slog.Warn("failed to resolve tariff for session cost", "sessionID", session.ID, "err", err)
		return 0, 0
	}
	readings, err := s.sessionReader.GetPowerReadings(ctx, session.ID)
	if err != nil {
		slog.Warn("failed to load power readings for session cost", "sessionID", session.ID, "err", err)
		return 0, 0
	}
	return CalculateSessionCost(*session.StartTotalKwh, readings, tariff)
}

// CancelActiveSession cancels an active or conditioning session (e.g. due to plug disconnect).
// DB is updated before power is cut so the poll loop sees the cancelled state immediately.
func (s *SessionLifecycleService) CancelActiveSession(ctx context.Context, session *models.ChargeSession) error {
	if err := s.verifySessionOwnership(ctx, session); err != nil {
		return err
	}
	s.lock.Lock()
	if err := s.sessionWriter.UpdateCancelData(ctx, session.ID, time.Now()); err != nil {
		s.lock.Unlock()
		return err
	}
	s.lock.Unlock()

	// Plug may already be off; best-effort power cut.
	if s.plugCtrl != nil && session.PlugID != nil {
		if err := s.plugCtrl.SetPower(ctx, *session.PlugID, false); err != nil {
			slog.Warn("[CANCEL-ACTIVE] SetPower failed (plug may already be off)", "err", err)
		}
	}

	return nil
}

// cancelPendingSession cancels a pending session by marking it cancelled in DB first,
// then turning off power. DB is updated first so the poll loop sees the session
// as non-active before power is cut, preventing orphaned readings.
func (s *SessionLifecycleService) cancelPendingSession(ctx context.Context, session *models.ChargeSession) (*StopResult, error) {
	result := &StopResult{Stopped: true}

	s.lock.Lock()
	if err := s.sessionWriter.UpdateCancelData(ctx, session.ID, time.Now()); err != nil {
		s.lock.Unlock()
		return nil, err
	}
	s.lock.Unlock()

	// Turn off plug power AFTER DB is updated
	if s.plugCtrl != nil && session.PlugID != nil {
		if err := s.plugCtrl.SetPower(ctx, *session.PlugID, false); err != nil {
			result.Stopped = false
			result.TasmotaErr = err.Error()
		}
	}

	return result, nil
}

// calculateEndData determines end kWh and percent from MQTT energy readings or fallback.
func (s *SessionLifecycleService) calculateEndData(ctx context.Context, session *models.ChargeSession, endPercent float64) (float64, float64) {
	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && session.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*session.PlugID)
	}

	endKwh, actualEndPercent := s.computeEndFromEnergy(ctx, session, energy)
	if endKwh == 0 {
		endKwh = s.computeEndFromPercent(ctx, session, endPercent)
		actualEndPercent = endPercent
	}
	return endKwh, actualEndPercent
}

// computeEndFromEnergy calculates end kWh and percent from Tasmota energy readings.
func (s *SessionLifecycleService) computeEndFromEnergy(ctx context.Context, session *models.ChargeSession, energy *tasmota.EnergyData) (float64, float64) {
	canCompute := energy != nil && energy.Total > 0 && session.StartTotalKwh != nil
	vehicle, vErr := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	hasValidVehicle := vErr == nil && vehicle != nil && vehicle.CapacityKwh > 0
	if !canCompute || !hasValidVehicle {
		return 0, 0
	}

	efficiency := vehicle.ChargingEfficiency
	if efficiency <= 0 {
		efficiency = models.DefaultChargingEfficiency
	}
	sessionEnergyKwh := (energy.Total - *session.StartTotalKwh) * efficiency
	if sessionEnergyKwh <= epsilonKwh {
		return 0, 0
	}

	endKwh := session.StartKwh + sessionEnergyKwh
	endKwh = math.Max(0, math.Min(vehicle.CapacityKwh, endKwh))
	actualEndPercent := math.Max(0, math.Min(100, (endKwh/vehicle.CapacityKwh)*100))
	return endKwh, actualEndPercent
}

// computeEndFromPercent calculates end kWh from the passed percent value.
func (s *SessionLifecycleService) computeEndFromPercent(ctx context.Context, session *models.ChargeSession, endPercent float64) float64 {
	vehicle, vErr := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	if vErr != nil || vehicle == nil || vehicle.CapacityKwh <= 0 {
		return 0
	}
	return vehicle.CapacityKwh * endPercent / 100
}

// calculateEndPercent determines the final battery percent from MQTT energy readings.
func (s *SessionLifecycleService) calculateEndPercent(ctx context.Context, session *models.ChargeSessionView) float64 {
	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && session.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*session.PlugID)
	}
	if energy != nil && energy.Total > 0 && session.StartTotalKwh != nil {
		vehicle, vErr := s.vehicleRepo.FindByID(ctx, session.VehicleID)
		if vErr == nil && vehicle != nil && vehicle.CapacityKwh > 0 {
			return CalculateEndPercent(&session.ChargeSession, energy, vehicle)
		}
	}

	if session.CurrentPercent != nil {
		return *session.CurrentPercent
	}

	return session.TargetPercent
}

// verifySessionOwnership checks that the session belongs to the current user.
// When no user is in context (background worker), skips the check.
func (s *SessionLifecycleService) verifySessionOwnership(ctx context.Context, session *models.ChargeSession) error {
	userID, ok := internal.UserIDFromContext(ctx)
	if !ok {
		return nil // background worker, no user context
	}
	if session.UserID == nil {
		return ErrSessionNotFound
	}
	if *session.UserID != userID {
		return ErrSessionNotFound
	}
	return nil
}

// validatePlugOwnership checks that the plug exists and belongs to the current user.
// When no user is in context (background workers), only checks plug exists.
func (s *SessionLifecycleService) validatePlugOwnership(ctx context.Context, plugID string) error {
	if plugID == "" {
		return nil
	}
	if s.plugRepo == nil {
		return nil
	}
	plug, err := s.plugRepo.FindByID(ctx, plugID)
	if err != nil {
		return err
	}
	if plug == nil {
		return ErrPlugNotFound
	}
	userID, ok := internal.UserIDFromContext(ctx)
	if !ok {
		return nil
	}
	if plug.UserID != userID {
		return ErrPlugNotFound
	}
	return nil
}

// validateVehicleExists checks that the vehicle exists and belongs to the current user.
// When no user is in context (background workers), only checks vehicle exists.
func (s *SessionLifecycleService) validateVehicleExists(ctx context.Context, vehicleID string) error {
	vehicle, err := s.vehicleRepo.FindByID(ctx, vehicleID)
	if err != nil {
		return err
	}
	if vehicle == nil {
		return ErrVehicleNotFound
	}
	userID, ok := internal.UserIDFromContext(ctx)
	if !ok {
		return nil
	}
	if vehicle.UserID == nil || *vehicle.UserID != userID {
		return ErrVehicleNotFound
	}
	return nil
}

// ensureNoActiveSessionForPlug returns ErrActiveSessionExists if that plug already has an active session.
// Falls back to a global active-session check when plugID is empty (legacy/no-plug path).
func (s *SessionLifecycleService) ensureNoActiveSessionForPlug(ctx context.Context, plugID string) error {
	var existingActive *models.ChargeSession
	var err error
	if plugID == "" {
		existingActive, err = s.sessionReader.GetActive(ctx)
	} else {
		existingActive, err = s.sessionReader.GetActiveByPlug(ctx, plugID)
	}
	if err != nil {
		return err
	}
	if existingActive != nil {
		return ErrActiveSessionExists
	}
	return nil
}

// captureEnergyBaseline returns the last cached MQTT energy total for session start tracking.
// Returns nil if no MQTT energy is available yet (session is still created without a baseline).
func (s *SessionLifecycleService) captureEnergyBaseline(_ context.Context, plugID string) *float64 {
	if s.plugCtrl == nil || plugID == "" {
		return nil
	}
	energy := s.plugCtrl.LastEnergy(plugID)
	if energy == nil {
		return nil
	}
	return &energy.Total
}

// createSessionFromPercent creates a new charge session with percent-based start/end values.
// holdPercent and readyByTime are non-nil only for two-stage (ready-by) sessions.
func (s *SessionLifecycleService) createSessionFromPercent(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent float64, startTotalKwh, holdPercent *float64, readyByTime *string) (*models.ChargeSession, error) {
	vehicle, err := s.vehicleRepo.FindByID(ctx, vehicleID)
	if err != nil {
		return nil, err
	}

	session := &models.ChargeSession{
		VehicleID:     vehicleID,
		StartPercent:  startPercent,
		StartKwh:      vehicle.CapacityKwh * startPercent / 100,
		TargetPercent: targetPercent,
		TargetKwh:     vehicle.CapacityKwh * targetPercent / 100,
		Status:        models.SessionStatusPending,
		CreatedAt:     time.Now(),
		StartTotalKwh: startTotalKwh,
		HoldPercent:   holdPercent,
		ReadyByTime:   readyByTime,
	}

	if userID, ok := internal.UserIDFromContext(ctx); ok {
		session.UserID = &userID
	} else if vehicle.UserID != nil {
		session.UserID = vehicle.UserID
	}
	if session.UserID == nil {
		return nil, ErrUserIDRequired
	}
	if plugID != "" {
		session.PlugID = &plugID
	} else if s.plugRepo != nil {
		// Fallback: find first plug for the user
		plugs, err := s.plugRepo.List(ctx, *session.UserID)
		if err == nil && len(plugs) > 0 {
			session.PlugID = &plugs[0].ID
		}
	}
	if session.PlugID == nil {
		return nil, ErrPlugNotFound
	}

	return session, s.sessionWriter.Create(ctx, session)
}

// sendChargeCompleteNotification sends a push notification when a charge session completes.
func (s *SessionLifecycleService) sendChargeCompleteNotification(ctx context.Context, session *models.ChargeSession, actualEndPercent float64) {
	s.notifier.NotifyChargeComplete(ctx, session, actualEndPercent)
}

// sendChargeStartedNotification sends a push notification when a charge session starts.
func (s *SessionLifecycleService) sendChargeStartedNotification(ctx context.Context, session *models.ChargeSession) {
	s.notifier.NotifyChargeStarted(ctx, session)
}

// UpdateTarget updates the target percent for an active session.
func (s *SessionLifecycleService) UpdateTarget(ctx context.Context, sessionID string, newTargetPercent float64) error {
	session, err := s.sessionReader.FindByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	if err := s.verifySessionOwnership(ctx, session); err != nil {
		return err
	}
	if session.Status != models.SessionStatusActive {
		return ErrSessionNotActive
	}

	if newTargetPercent < 0 || newTargetPercent > models.MaxPercent {
		return ErrTargetOutOfRange
	}

	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && session.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*session.PlugID)
	}
	if energy == nil || energy.Total == 0 || session.StartTotalKwh == nil {
		if newTargetPercent <= session.StartPercent {
			return ErrTargetBelowStart
		}
	} else {
		vehicle, errFind := s.vehicleRepo.FindByID(ctx, session.VehicleID)
		if errFind != nil || vehicle == nil || vehicle.CapacityKwh <= 0 {
			return ErrVehicleConfigMissing
		}

		progress := CalculateProgress(session, energy, vehicle)

		if newTargetPercent <= progress.CurrentPercent {
			return ErrTargetBelowCurrent
		}
	}

	if err := s.sessionWriter.UpdateTarget(ctx, sessionID, newTargetPercent); errors.Is(err, repository.ErrSessionWrongState) {
		return ErrSessionNotActive
	} else if err != nil {
		return err
	}
	return nil
}

// DeleteSession deletes a completed or cancelled session.
func (s *SessionLifecycleService) DeleteSession(ctx context.Context, id string) error {
	session, err := s.sessionReader.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	if err := s.verifySessionOwnership(ctx, session); err != nil {
		return err
	}
	if session.Status == models.SessionStatusActive || session.Status == models.SessionStatusPending || session.Status == models.SessionStatusConditioning {
		return ErrCannotDeleteActiveSession
	}

	// Decrement vehicle lifetime stats if session has pre-computed stats
	if session.BatteryKwh != nil {
		wallKwh := 0.0
		co2Grams := 0.0
		costPence := 0.0
		if session.WallKwh != nil {
			wallKwh = *session.WallKwh
		}
		if session.Co2Grams != nil {
			co2Grams = *session.Co2Grams
		}
		if session.CostPence != nil {
			costPence = *session.CostPence
		}
		if decErr := s.vehicleRepo.DecrementLifetimeStats(ctx, session.VehicleID, *session.BatteryKwh, wallKwh, co2Grams, costPence); decErr != nil {
			slog.Warn("failed to decrement vehicle lifetime stats on session delete", "err", decErr)
		}
	}

	return s.sessionWriter.Delete(ctx, id)
}
