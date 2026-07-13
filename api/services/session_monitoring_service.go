package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"ev-charge-controller/api/chargeestimate"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"
)

// staleReadingThreshold is the max age of a reading before it must be stored
// again regardless of whether the value changed - ensures periodic data points.
const staleReadingThreshold = 30 * time.Minute

// SessionMonitoringService handles energy monitoring: power readings,
// SOC snapshots, and auto-stop when target is reached.
type SessionMonitoringService struct {
	sessionReader    internal.SessionReader
	sessionWriter    internal.SessionWriter
	snapshotRepo     internal.SnapshotReader
	powerReadingRepo internal.PowerReadingReader
	vehicleRepo      internal.VehicleRepo
	plugCtrl         internal.PlugController
	carbonIntensity  internal.CarbonIntensityFetcher
	socGenerator     *SOCGenerator
	socWorker        *SOCWorker
	lock             *sessionLock

	// estimator computes two-stage resume timing. Defaults to
	// chargeestimate.EstimateMinutes; overridable via SetEstimator for tests.
	estimator DurationEstimator
	// forecaster supplies carbon intensity forecasts for carbon-aware two-stage
	// resume timing. Unset by default; without it, carbon-aware-origin holds
	// fall back to plain deadline-guard resume, same as daily-origin holds.
	forecaster internal.CarbonForecaster
}

// SetEstimator injects a custom charge-duration estimator for two-stage resume
// timing. Used in tests to stub duration estimates; production callers may leave
// it unset to use the chargeestimate.EstimateMinutes default.
func (s *SessionMonitoringService) SetEstimator(est DurationEstimator) {
	s.estimator = est
}

// SetForecaster injects the carbon intensity forecaster used to pick a
// balanced (clean + late) resume time for carbon-aware two-stage sessions.
// Wired from server.go alongside ScheduleService's carbon-aware deps.
func (s *SessionMonitoringService) SetForecaster(f internal.CarbonForecaster) {
	s.forecaster = f
}

// NewSessionMonitoringService creates a new SessionMonitoringService.
// The session lock is shared with ChargeSessionService to serialize monitoring
// operations with lifecycle mutations (Stop, StartSession, etc.).
func NewSessionMonitoringService(
	sessionReader internal.SessionReader,
	sessionWriter internal.SessionWriter,
	snapshotRepo internal.SnapshotReader,
	powerReadingRepo internal.PowerReadingReader,
	vehicleRepo internal.VehicleRepo,
	plugCtrl internal.PlugController,
	carbonIntensity internal.CarbonIntensityFetcher,
	socWorker *SOCWorker,
	lock *sessionLock,
) *SessionMonitoringService {
	return &SessionMonitoringService{
		sessionReader:    sessionReader,
		sessionWriter:    sessionWriter,
		snapshotRepo:     snapshotRepo,
		powerReadingRepo: powerReadingRepo,
		vehicleRepo:      vehicleRepo,
		plugCtrl:         plugCtrl,
		carbonIntensity:  carbonIntensity,
		socGenerator:     NewSOCGenerator(),
		socWorker:        socWorker,
		lock:             lock,
	}
}

// GetEnergy returns the last cached MQTT energy reading for the active session's plug.
// Returns (nil, nil) when no active session exists or no MQTT data is available yet.
func (s *SessionMonitoringService) GetEnergy(ctx context.Context) (*tasmota.EnergyData, error) {
	if s.plugCtrl == nil {
		return nil, nil
	}
	session, err := s.sessionReader.GetActive(ctx)
	if err != nil || session == nil || session.PlugID == nil {
		return nil, nil
	}
	return s.plugCtrl.LastEnergy(*session.PlugID), nil
}

// SetPowerState controls the active session's plug power outlet.
// No-op when no active session or no plug assigned.
func (s *SessionMonitoringService) SetPowerState(ctx context.Context, powerOn bool) error {
	if s.plugCtrl == nil {
		return nil
	}
	session, err := s.sessionReader.GetActive(ctx)
	if err != nil {
		return err
	}
	if session == nil || session.PlugID == nil {
		return nil
	}
	return s.plugCtrl.SetPower(ctx, *session.PlugID, powerOn)
}

// AddPowerReading saves a power reading to the database.
func (s *SessionMonitoringService) AddPowerReading(ctx context.Context, reading *models.PowerReading) error {
	return s.powerReadingRepo.CreatePowerReading(ctx, reading)
}

// GetLastCompleted returns the last completed charge session.
func (s *SessionMonitoringService) GetLastCompleted(ctx context.Context) (*models.ChargeSession, error) {
	return s.sessionReader.GetLastCompleted(ctx)
}

// StoreSOCSnapshot calculates the SOC from Tasmota energy readings and persists it.
func (s *SessionMonitoringService) StoreSOCSnapshot(ctx context.Context, session *models.ChargeSession, energy *tasmota.EnergyData) error {
	vehicle, err := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	if err != nil {
		return err
	}
	if vehicle == nil || vehicle.CapacityKwh <= 0 {
		return nil
	}

	socPercent, lastBlendedKwh, err := s.socGenerator.CalculateSOC(session, energy, vehicle)
	if err != nil {
		return err
	}

	snapshot := s.socGenerator.BuildSnapshot(session.ID, socPercent)

	lastSnapshot, lastErr := s.snapshotRepo.GetLastSOCSnapshot(ctx, session.ID)
	if lastErr != nil {
		slog.Warn("Error fetching last SOC snapshot for dedup check", "err", lastErr)
	}
	if shouldStoreSOCSnapshot(lastSnapshot, snapshot) {
		if err := s.snapshotRepo.CreateSOCSnapshot(ctx, snapshot); err != nil {
			return err
		}
	}

	return s.sessionWriter.UpdateLastBlendedKwh(ctx, session.ID, lastBlendedKwh)
}

// SaveEnergyReadings atomically checks session status and saves a power reading
// against the active session for the given plug - readings must never be
// attributed to another plug's session when several charge concurrently.
// SOC snapshot is offloaded to an async worker to avoid blocking the poll cycle.
// The mutex ensures the session status check and power reading write are serialised
// with Stop, StartSession, and other session lifecycle mutations.
func (s *SessionMonitoringService) SaveEnergyReadings(ctx context.Context, plugID string, energy *tasmota.EnergyData) {
	// Grid carbon intensity is an external HTTP call and is independent of
	// session state, so fetch it BEFORE acquiring the lock. Holding the shared
	// session lock across network I/O would block every lifecycle mutation
	// (Stop, StartSession, …) for the duration of the request.
	carbonIntensity := s.currentCarbonIntensity(ctx)

	session := s.saveReadingLocked(ctx, plugID, energy, carbonIntensity)
	if session == nil {
		return
	}

	// Offload SOC snapshot to async worker
	if session.Status != models.SessionStatusPending && session.StartTotalKwh != nil {
		s.socWorker.Send(socRequest{
			sessionID:      session.ID,
			vehicleID:      session.VehicleID,
			startKwh:       session.StartKwh,
			startTotalKwh:  *session.StartTotalKwh,
			targetKwh:      session.TargetKwh,
			createdAt:      session.CreatedAt,
			startedAt:      session.StartedAt,
			lastBlendedKwh: session.LastBlendedKwh,
			energy:         energy,
		})
	}
}

// currentCarbonIntensity returns the current grid carbon intensity in
// gCO2/kWh, or nil if unavailable. A nil client, fetch error, or nil reading
// are all non-fatal - the power reading is simply stored without it.
func (s *SessionMonitoringService) currentCarbonIntensity(ctx context.Context) *float64 {
	if s.carbonIntensity == nil {
		return nil
	}
	ci, err := s.carbonIntensity.GetCurrent(ctx)
	if err != nil || ci == nil {
		return nil
	}
	v := float64(ci.Actual)
	return &v
}

// saveReadingLocked performs the check-then-act under the shared session lock:
// it confirms a charging/conditioning session is active on the plug and persists
// the power reading, serialising with lifecycle mutations (Stop, StartSession, …).
// It returns the active session for follow-up work (SOC offload) outside the lock,
// or nil if there was nothing to record. No network I/O happens under the lock.
func (s *SessionMonitoringService) saveReadingLocked(ctx context.Context, plugID string, energy *tasmota.EnergyData, carbonIntensity *float64) *models.ChargeSession {
	s.lock.Lock()
	defer s.lock.Unlock()

	session, err := s.sessionReader.GetActiveByPlug(ctx, plugID)
	if err != nil {
		slog.Error("Error getting active session for energy save", "err", err, "plugID", plugID)
		return nil
	}
	if session == nil || (session.Status != models.SessionStatusActive && session.Status != models.SessionStatusConditioning) {
		return nil
	}

	reading := &models.PowerReading{
		ID:                        uuid.New().String(),
		SessionID:                 session.ID,
		EnergyKwh:                 energy.Total,
		Power:                     energy.Power,
		Voltage:                   energy.Voltage,
		Current:                   energy.Current,
		Timestamp:                 time.Now(),
		CarbonIntensityGCo2PerKwh: carbonIntensity,
	}

	lastReading, lastErr := s.powerReadingRepo.GetLastPowerReading(ctx, session.ID)
	if lastErr != nil {
		slog.Warn("Error fetching last power reading for dedup check", "err", lastErr)
	}
	if shouldStorePowerReading(lastReading, reading) {
		if err := s.powerReadingRepo.CreatePowerReading(ctx, reading); err != nil {
			slog.Error("Error saving power reading", "err", err)
		}
	}

	return session
}

// CheckAndStopConditioningSession checks whether a conditioning session has
// tapered to the stop threshold and completes it if so.
func (s *SessionMonitoringService) CheckAndStopConditioningSession(ctx context.Context, stopper sessionStopper) {
	sessions, err := s.sessionReader.ListActive(ctx)
	if err != nil {
		return
	}
	for i := range sessions {
		if sessions[i].Status != models.SessionStatusConditioning {
			continue
		}
		s.stopConditioningSession(ctx, stopper, &sessions[i])
	}
}

// stopConditioningSession runs the CV-taper stop check for a single conditioning session.
func (s *SessionMonitoringService) stopConditioningSession(ctx context.Context, stopper sessionStopper, activeSession *models.ChargeSession) {
	vehicle, err := s.vehicleRepo.FindByID(ctx, activeSession.VehicleID)
	if err != nil || vehicle == nil || vehicle.ChargerOutputW <= 0 {
		slog.Warn("[CONDITIONING] Cannot check threshold: vehicle missing or no charger output", "err", err)
		return
	}

	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && activeSession.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*activeSession.PlugID)
	}
	if energy == nil {
		slog.Info("[CONDITIONING] Cannot check: no MQTT energy data")
		return
	}

	thresholdW := vehicle.ChargerOutputW * models.ConditioningStopThresholdFraction
	slog.Info("[CONDITIONING] Check", "sessionID", activeSession.ID, "powerW", energy.Power, "thresholdW", thresholdW)

	if energy.Power < thresholdW {
		slog.Info("[CONDITIONING] Power below threshold, completing session", "sessionID", activeSession.ID)
		result, err := stopper.stopWithPercent(ctx, activeSession, activeSession.TargetPercent, models.StopAutoComplete)
		if err != nil {
			slog.Error("[CONDITIONING] Error completing session", "err", err)
		} else if !result.Stopped {
			slog.Error("[CONDITIONING] Tasmota stop failed", "tasmotaErr", result.TasmotaErr)
		} else {
			slog.Info("[CONDITIONING] Session completed", "sessionID", activeSession.ID)
		}
	}
}

// CheckAndAutoStopReachingSession checks if the active session has reached its
// target and automatically stops it if so.
type sessionStopper interface {
	stopWithPercent(ctx context.Context, session *models.ChargeSession, endPercent float64, reason ...models.StopReason) (*StopResult, error)
}

func (s *SessionMonitoringService) CheckAndAutoStopReachingSession(ctx context.Context, stopper sessionStopper) {
	sessions, err := s.sessionReader.ListActive(ctx)
	if err != nil {
		slog.Error("[AUTO-STOP] Error listing active sessions", "err", err)
		return
	}
	if len(sessions) == 0 {
		slog.Info("[AUTO-STOP] No active session")
		return
	}
	for i := range sessions {
		// Skip pending/holding/conditioning sessions - they are not charging
		// toward the target right now (each has its own dedicated check).
		if sessions[i].Status != models.SessionStatusActive {
			continue
		}
		s.autoStopReachingSession(ctx, stopper, &sessions[i])
	}
}

// autoStopReachingSession runs the target-reached check for a single active session.
func (s *SessionMonitoringService) autoStopReachingSession(ctx context.Context, stopper sessionStopper, activeSession *models.ChargeSession) {
	slog.Info("[AUTO-STOP] Session", "sessionID", activeSession.ID, "status", activeSession.Status, "target", activeSession.TargetPercent, "startKwh", activeSession.StartKwh, "targetKwh", activeSession.TargetKwh, "startTotalKwh", activeSession.StartTotalKwh)

	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && activeSession.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*activeSession.PlugID)
	}
	if energy == nil || energy.Total == 0 {
		if energy == nil {
			slog.Info("[AUTO-STOP] Skip: no MQTT energy data")
		} else {
			slog.Info("[AUTO-STOP] Skip: energy.Total=0")
		}
		return
	}
	if activeSession.StartTotalKwh == nil {
		// A session without a baseline can never compute progress, so it would
		// otherwise charge past its target forever. Adopt the current meter
		// total as the baseline (slightly conservative: energy delivered before
		// this tick isn't counted, so the session charges a little longer, but
		// it becomes stoppable).
		s.backfillEnergyBaseline(ctx, activeSession, energy)
		return
	}

	slog.Info("[AUTO-STOP] MQTT energy", "total", energy.Total, "power", energy.Power)

	vehicle, err := s.vehicleRepo.FindByID(ctx, activeSession.VehicleID)
	if err != nil || vehicle == nil || vehicle.CapacityKwh <= 0 {
		slog.Info("[AUTO-STOP] Skip: vehicle lookup failed", "err", err, "vehicle", vehicle != nil, "capacity", vehicle.CapacityKwh)
		return
	}

	slog.Info("[AUTO-STOP] Vehicle", "id", vehicle.ID, "capacity", vehicle.CapacityKwh, "efficiency", vehicle.ChargingEfficiency)

	progress := CalculateProgress(activeSession, energy, vehicle)

	slog.Info("[AUTO-STOP] Progress", "blendedKwh", progress.BlendedKwh, "targetKwh", activeSession.TargetKwh, "currentPercent", progress.CurrentPercent, "targetPercent", activeSession.TargetPercent, "remainingKwh", activeSession.TargetKwh-progress.BlendedKwh, "hasTick", progress.HasTick)

	s.lock.Lock()
	err = s.sessionWriter.UpdateLastBlendedKwh(ctx, activeSession.ID, progress.LastBlendedKwh)
	s.lock.Unlock()
	if err != nil {
		slog.Warn("failed to update last blended kWh for session", "sessionID", activeSession.ID, "err", err)
	}

	// Two-stage (ready-by) session still charging toward its intermediate hold
	// point: compare against holdKwh instead of the real target. Once resumed,
	// HoldPercent is cleared (see ResumeHolding) and the block below runs as normal.
	if activeSession.HoldPercent != nil {
		holdKwh := vehicle.CapacityKwh * *activeSession.HoldPercent / 100
		slog.Info("[AUTO-STOP] Two-stage hold check", "sessionID", activeSession.ID, "blendedKwh", progress.BlendedKwh, "holdKwh", holdKwh, "holdPercent", *activeSession.HoldPercent)
		if progress.BlendedKwh >= holdKwh {
			s.holdSession(ctx, activeSession)
		}
		return
	}

	if progress.BlendedKwh >= activeSession.TargetKwh {
		slog.Info("[AUTO-STOP] TRIGGERED", "sessionID", activeSession.ID, "blendedKwh", progress.BlendedKwh, "targetKwh", activeSession.TargetKwh, "currentPercent", progress.CurrentPercent, "targetPercent", activeSession.TargetPercent)
		// 100% target: enter conditioning phase instead of stopping immediately.
		// The charger stays on until the CV tail current tapers below the threshold.
		if activeSession.TargetPercent == models.MaxPercent {
			s.lock.Lock()
			condErr := s.sessionWriter.UpdateStatus(ctx, activeSession.ID, models.SessionStatusConditioning)
			s.lock.Unlock()
			if condErr != nil {
				slog.Error("[AUTO-STOP] Error transitioning to conditioning", "err", condErr)
			} else {
				slog.Info("[AUTO-STOP] Session transitioned to conditioning", "sessionID", activeSession.ID)
			}
		} else {
			result, err := stopper.stopWithPercent(ctx, activeSession, progress.CurrentPercent, models.StopAutoComplete)
			if err != nil {
				slog.Error("[AUTO-STOP] Error stopping session", "err", err)
			} else if !result.Stopped {
				slog.Error("[AUTO-STOP] Tasmota stop failed", "tasmotaErr", result.TasmotaErr)
			} else {
				slog.Info("[AUTO-STOP] Session stopped successfully", "sessionID", activeSession.ID)
			}
		}
	} else {
		slog.Info("[AUTO-STOP] Not yet at target", "blendedKwh", progress.BlendedKwh, "targetKwh", activeSession.TargetKwh, "gap", activeSession.TargetKwh-progress.BlendedKwh, "currentPercent", progress.CurrentPercent, "targetPercent", activeSession.TargetPercent)
	}
}

// backfillEnergyBaseline persists a missing wall-side energy baseline for a
// session using the current cumulative meter reading. Sessions can legitimately
// start without a baseline (the plug's MQTT cache was empty at creation, e.g.
// right after an API restart); without this heal they can never compute
// progress and never auto-stop.
func (s *SessionMonitoringService) backfillEnergyBaseline(ctx context.Context, session *models.ChargeSession, energy *tasmota.EnergyData) {
	if energy == nil || energy.Total <= 0 {
		return
	}
	s.lock.Lock()
	err := s.sessionWriter.UpdateStartTotalKwh(ctx, session.ID, energy.Total)
	s.lock.Unlock()
	if err != nil {
		slog.Error("[AUTO-STOP] Baseline backfill failed", "sessionID", session.ID, "err", err)
		return
	}
	slog.Warn("[AUTO-STOP] Backfilled missing energy baseline", "sessionID", session.ID, "startTotalKwh", energy.Total)
}

// CheckAndStopIdleSession auto-completes an active or conditioning session
// whose plug has drawn near-zero power for IdleSessionTimeout, even though
// the session's energy target was never reached. This covers vehicles whose
// BMS stops drawing current before the energy model's blended kWh reaches
// TargetKwh, so neither CheckAndAutoStopReachingSession nor
// CheckAndStopConditioningSession ever arms - without this check such a
// session sits "active" indefinitely until manually stopped.
func (s *SessionMonitoringService) CheckAndStopIdleSession(ctx context.Context, stopper sessionStopper) {
	sessions, err := s.sessionReader.ListActive(ctx)
	if err != nil {
		return
	}
	for i := range sessions {
		status := sessions[i].Status
		if status != models.SessionStatusActive && status != models.SessionStatusConditioning {
			continue
		}
		s.stopIdleSession(ctx, stopper, &sessions[i])
	}
}

// stopIdleSession runs the idle-power stop check for a single active or conditioning session.
func (s *SessionMonitoringService) stopIdleSession(ctx context.Context, stopper sessionStopper, activeSession *models.ChargeSession) {
	if activeSession.StartedAt == nil || time.Since(*activeSession.StartedAt) < models.MinSessionDurationBeforeIdleCheck {
		return
	}

	readings, err := s.powerReadingRepo.GetPowerReadings(ctx, activeSession.ID)
	if err != nil || len(readings) == 0 {
		return
	}

	idleSince, isIdle := idleStreakStart(readings)
	if !isIdle || time.Since(idleSince) < models.IdleSessionTimeout {
		return
	}

	slog.Info("[IDLE-STOP] Power idle beyond timeout, completing session", "sessionID", activeSession.ID, "idleSince", idleSince)
	result, err := stopper.stopWithPercent(ctx, activeSession, models.MaxPercent, models.StopAutoComplete)
	if err != nil {
		slog.Error("[IDLE-STOP] Error completing session", "err", err)
	} else if !result.Stopped {
		slog.Error("[IDLE-STOP] Tasmota stop failed", "tasmotaErr", result.TasmotaErr)
	} else {
		slog.Info("[IDLE-STOP] Session completed", "sessionID", activeSession.ID)
	}
}

// idleStreakStart returns the timestamp of the earliest reading in the
// unbroken run of sub-IdlePowerThresholdW readings ending at the most recent
// reading, and whether the most recent reading is itself idle. readings must
// be ordered oldest-first (as returned by PowerReadingReader.GetPowerReadings).
func idleStreakStart(readings []models.PowerReading) (time.Time, bool) {
	last := readings[len(readings)-1]
	if last.Power >= models.IdlePowerThresholdW {
		return time.Time{}, false
	}

	streakStart := last.Timestamp
	for i := len(readings) - 1; i >= 0 && readings[i].Power < models.IdlePowerThresholdW; i-- {
		streakStart = readings[i].Timestamp
	}
	return streakStart, true
}

// holdSession powers off the plug and transitions a two-stage session to holding
// once it reaches its intermediate hold point. Only transitions on confirmed
// power-off; an unconfirmed attempt is retried on the next poll tick.
func (s *SessionMonitoringService) holdSession(ctx context.Context, session *models.ChargeSession) {
	if s.plugCtrl == nil || session.PlugID == nil {
		return
	}
	confirmed, err := s.plugCtrl.SetPowerAndWait(ctx, *session.PlugID, false, models.PowerConfirmationTimeout)
	if !confirmed {
		slog.Warn("[AUTO-STOP] Two-stage hold power-off not confirmed, retrying next tick", "sessionID", session.ID, "err", err)
		return
	}

	s.lock.Lock()
	holdErr := s.sessionWriter.UpdateStatus(ctx, session.ID, models.SessionStatusHolding)
	s.lock.Unlock()
	if holdErr != nil {
		slog.Error("[AUTO-STOP] Error transitioning to holding", "err", holdErr)
	} else {
		slog.Info("[AUTO-STOP] Session transitioned to holding", "sessionID", session.ID)
	}
}

// CheckAndResumeHoldingSession resumes a holding two-stage session once it's time
// to charge from HoldPercent to TargetPercent in order to reach ReadyByTime. The
// deadline guard (resume immediately once the last safe moment is reached, or on
// a duration-estimator error) always applies. For carbon-aware-origin sessions
// with a forecaster configured, resuming earlier than the deadline is further
// gated by findBalancedStart, so stage 2 also favors clean, late power the same
// way stage 1's activation did. Daily-origin sessions (and carbon-aware sessions
// without a forecaster wired) keep the plain deadline-guard-only behavior.
func (s *SessionMonitoringService) CheckAndResumeHoldingSession(ctx context.Context) {
	sessions, err := s.sessionReader.ListActive(ctx)
	if err != nil {
		return
	}
	for i := range sessions {
		if sessions[i].Status != models.SessionStatusHolding {
			continue
		}
		s.resumeHoldingSessionIfDue(ctx, &sessions[i])
	}
}

// resumeHoldingSessionIfDue runs the resume decision for a single holding session.
func (s *SessionMonitoringService) resumeHoldingSessionIfDue(ctx context.Context, session *models.ChargeSession) {
	if session.HoldPercent == nil || session.ReadyByTime == nil {
		slog.Warn("[HOLD-RESUME] Holding session missing hold fields", "sessionID", session.ID)
		return
	}

	vehicle, err := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	if err != nil || vehicle == nil {
		slog.Warn("[HOLD-RESUME] Cannot resolve vehicle", "sessionID", session.ID, "err", err)
		return
	}

	now := scheduleNowFunc()
	deadline, err := resolveDeadline(now, *session.ReadyByTime)
	if err != nil {
		slog.Error("[HOLD-RESUME] Invalid ready-by time", "sessionID", session.ID, "err", err)
		return
	}

	est := s.estimator
	if est == nil {
		est = chargeestimate.EstimateMinutes
	}
	d, estErr := est(vehicle, *session.HoldPercent, session.TargetPercent)
	if estErr != nil {
		slog.Warn("[HOLD-RESUME] estimator error, resuming now", "sessionID", session.ID, "err", estErr)
		s.resumeHoldingSession(ctx, session)
		return
	}

	latestStart := deadline.Add(-time.Duration(d) * time.Minute)
	if !now.Before(latestStart) {
		slog.Info("[HOLD-RESUME] deadline guard, resuming now", "sessionID", session.ID, "latestStart", latestStart)
		s.resumeHoldingSession(ctx, session)
		return
	}

	if !session.CarbonAwareHold || s.forecaster == nil {
		// Daily-origin (or carbon-aware without a forecaster wired): plain
		// deadline guard only, not yet due - keep holding.
		return
	}

	buckets, ferr := s.forecaster.GetForecast(ctx, now, deadline)
	if ferr != nil || len(buckets) == 0 {
		slog.Warn("[HOLD-RESUME] forecast unavailable, deferring", "sessionID", session.ID, "err", ferr)
		return
	}

	optimalStart := findBalancedStart(buckets, now, latestStart, deadline, time.Duration(d)*time.Minute)
	currentBucket := alignToHalfHour(now.UTC())
	if optimalStart.IsZero() || !optimalStart.After(currentBucket) {
		s.resumeHoldingSession(ctx, session)
	}
	// else: a better-balanced window exists later - keep holding.
}

// EstimateResumeTime mirrors CheckAndResumeHoldingSession's decision logic for
// a currently-holding carbon-aware two-stage session, without any side
// effects (no power changes, no status writes). Returns ok=false when no
// confident estimate can be made - not holding, missing hold fields,
// daily-origin (no forecast optimization applies), no forecaster, or the
// forecast is unavailable.
func (s *SessionMonitoringService) EstimateResumeTime(ctx context.Context, session *models.ChargeSession) (string, bool) {
	if session == nil || session.Status != models.SessionStatusHolding || !session.CarbonAwareHold {
		return "", false
	}
	if session.HoldPercent == nil || session.ReadyByTime == nil {
		return "", false
	}
	if s.forecaster == nil {
		return "", false
	}

	vehicle, err := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	if err != nil || vehicle == nil {
		return "", false
	}

	now := scheduleNowFunc()
	deadline, err := resolveDeadline(now, *session.ReadyByTime)
	if err != nil {
		return "", false
	}

	est := s.estimator
	if est == nil {
		est = chargeestimate.EstimateMinutes
	}
	d, estErr := est(vehicle, *session.HoldPercent, session.TargetPercent)
	if estErr != nil {
		return "", false
	}

	latestStart := deadline.Add(-time.Duration(d) * time.Minute)
	if !now.Before(latestStart) {
		return formatTime(latestStart.In(now.Location())), true
	}

	buckets, ferr := s.forecaster.GetForecast(ctx, now, deadline)
	if ferr != nil || len(buckets) == 0 {
		return "", false
	}

	optimalStart := findBalancedStart(buckets, now, latestStart, deadline, time.Duration(d)*time.Minute)
	if optimalStart.IsZero() {
		return "", false
	}

	return formatTime(optimalStart.In(now.Location())), true
}

// resumeHoldingSession powers the plug back on and transitions a holding
// session back to active. Only transitions on confirmed power-on; an
// unconfirmed attempt is retried on the next poll tick.
func (s *SessionMonitoringService) resumeHoldingSession(ctx context.Context, session *models.ChargeSession) {
	if s.plugCtrl == nil || session.PlugID == nil {
		return
	}
	confirmed, confErr := s.plugCtrl.SetPowerAndWait(ctx, *session.PlugID, true, models.PowerConfirmationTimeout)
	if !confirmed {
		slog.Warn("[HOLD-RESUME] Power-on not confirmed, retrying next tick", "sessionID", session.ID, "err", confErr)
		return
	}

	s.lock.Lock()
	resumeErr := s.sessionWriter.ResumeHolding(ctx, session.ID)
	s.lock.Unlock()
	if resumeErr != nil {
		slog.Error("[HOLD-RESUME] Error resuming session", "err", resumeErr)
	} else {
		slog.Info("[HOLD-RESUME] Session resumed", "sessionID", session.ID)
	}
}

// shouldStorePowerReading returns true when the reading differs from the last
// stored reading or the last reading is older than staleReadingThreshold.
func shouldStorePowerReading(last *models.PowerReading, next *models.PowerReading) bool {
	if last == nil {
		return true
	}
	if time.Since(last.Timestamp) >= staleReadingThreshold {
		return true
	}
	if last.Power != next.Power || last.Voltage != next.Voltage || last.Current != next.Current {
		return true
	}
	return carbonIntensityDiffers(last.CarbonIntensityGCo2PerKwh, next.CarbonIntensityGCo2PerKwh)
}

// shouldStoreSOCSnapshot returns true when the snapshot SOC differs from the
// last stored snapshot or the last snapshot is older than staleReadingThreshold.
func shouldStoreSOCSnapshot(last *models.SOCSnapshot, next *models.SOCSnapshot) bool {
	if last == nil {
		return true
	}
	if time.Since(last.Timestamp) >= staleReadingThreshold {
		return true
	}
	return last.SocPercent != next.SocPercent
}

func carbonIntensityDiffers(a, b *float64) bool {
	if (a == nil) != (b == nil) {
		return true
	}
	if a == nil {
		return false
	}
	return *a != *b
}
