package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"
)

var (
	ErrSessionNotFound            = errors.New("charge session not found")
	ErrActiveSessionExists        = errors.New("an active charge session already exists")
	ErrCannotDeleteActiveSession  = errors.New("cannot delete an active or pending session")
	ErrSessionNotActive           = errors.New("session is not active")
	ErrTargetOutOfRange           = errors.New("charge target must be between 0 and 100")
	ErrTargetBelowStart           = errors.New("charge target must be higher than the starting battery level")
	ErrVehicleConfigMissing       = errors.New("vehicle configuration is missing - please reselect your vehicle in Settings")
	ErrTargetBelowCurrent         = errors.New("charge target must be higher than the current battery level")
	ErrUserIDRequired             = errors.New("user ID is required")
)

// epsilonKwh is the noise floor for Tasmota's cumulative energy counter.
// Values below this threshold are considered indistinguishable from rounding noise.
const epsilonKwh = 0.002

// CalcBlendedKwh computes the current battery-side Kwh using wall-side Tasmota
// energy with interpolation fallback. Applies charging efficiency to convert
// wall energy to battery energy (batteryKwh = wallKwh * efficiency).
// Returns the blended Kwh and whether Tasmota has meaningfully ticked.
func CalcBlendedKwh(startKwh, startTotalKwh float64, energy *tasmota.EnergyData, startTime time.Time, efficiency float64) (blendedKwh float64, hasTick bool) {
	if energy.Power <= 0 {
		slog.Debug("[SOC] CalcBlendedKwh: power, returning startKwh (no power)", "power", energy.Power, "startKwh", startKwh)
		return startKwh, false
	}

	// Convert wall-side energy to battery-side
	wallDelta := energy.Total - startTotalKwh
	sessionEnergyKwh := wallDelta * efficiency
	slog.Debug("[SOC] CalcBlendedKwh", "total", energy.Total, "startTotal", startTotalKwh, "wallDelta", wallDelta, "eff", efficiency, "sessionEnergyKwh", sessionEnergyKwh, "epsilon", epsilonKwh)

	if sessionEnergyKwh > epsilonKwh {
		result := startKwh + sessionEnergyKwh
		slog.Debug("[SOC] CalcBlendedKwh: TICK path", "blendedKwh", result)
		return result, true
	}

	// No meaningful tick from Tasmota - return startKwh unchanged.
	// Interpolation is omitted because CalcBlendedKwh is stateless and
	// called independently on each poll; cumulative interpolation would
	// compound phantom energy across successive calls.
	slog.Debug("[SOC] CalcBlendedKwh: NO TICK, returning startKwh", "sessionEnergyKwh", sessionEnergyKwh, "epsilon", epsilonKwh, "startKwh", startKwh)
	return startKwh, false
}

// ChargeSessionService orchestrates charge session lifecycle with concurrent-safe
// state mutations. Three independent services (lifecycle, monitoring, notifier)
// coordinate via a shared mutex to serialize access to the active session.
//
// Concurrency Model:
//
// The mu mutex guards all mutations to the active ChargeSession. It enforces
// the invariant: only one goroutine may mutate session state at a time.
// This prevents race conditions when:
//   - SessionLifecycleService: Start, Cancel, Stop operations mutate session status
//   - SessionMonitoringService: Energy polling and auto-stop detection mutate session fields
//   - ChargeNotifier: Concurrently sends push notifications while session transitions
//
// The mutex is held ONLY during state mutations, not during I/O (fetch, notify).
// Consumers (handlers) see consistent snapshots because all transitions are atomic.
//
// Future Extensions: Adding a new service that mutates session state requires:
// 1. Injecting that service into ChargeSessionService.New()
// 2. Ensuring all mutations are protected by mu.Lock() / mu.Unlock()
// 3. Testing under concurrent load with go test -race
type ChargeSessionService struct {
	lock          *sessionLock
	sessionReader internal.SessionReader
	sessionWriter internal.SessionWriter
	snapshotRepo  internal.SnapshotReader
	vehicleRepo   internal.VehicleRepo
	plugCtrl      internal.PlugController
	socGenerator  *SOCGenerator
	notifier      *ChargeNotifier
	socWorker     *SOCWorker
	monitoring    *SessionMonitoringService
	lifecycle     *SessionLifecycleService
	shutdownOnce  sync.Once
}

func NewChargeSessionService(
	ctx context.Context,
	repo internal.ChargeSessionServiceRepo,
	vehicleRepo internal.VehicleRepo,
	plugRepo internal.PlugRepo,
	plugCtrl internal.PlugController,
	carbonIntensity internal.CarbonIntensityFetcher,
	pushService *PushService,
) *ChargeSessionService {
	notifier := NewChargeNotifier(ctx, pushService, vehicleRepo, plugRepo)
	lock := newSessionLock()
	s := &ChargeSessionService{
		lock:          lock,
		sessionReader: repo,
		sessionWriter: repo,
		snapshotRepo:  repo,
		vehicleRepo:   vehicleRepo,
		plugCtrl:      plugCtrl,
		socGenerator:  NewSOCGenerator(),
		notifier:      notifier,
	}
	s.socWorker = NewSOCWorker(s)
	go s.socWorker.Start(ctx)
	s.lifecycle = NewSessionLifecycleService(repo, repo, vehicleRepo, plugRepo, plugCtrl, repo, notifier, lock)
	s.monitoring = NewSessionMonitoringService(repo, repo, repo, repo, vehicleRepo, plugCtrl, carbonIntensity, s.socWorker, lock)
	return s
}

// SetTariffProvider wires the tariff provider used to freeze a session's
// time-weighted electricity cost when it completes.
func (s *ChargeSessionService) SetTariffProvider(p TariffProvider) {
	s.lifecycle.SetTariffProvider(p)
}

// SetPlugController wires in the MQTT plug controller after it connects at startup.
func (s *ChargeSessionService) SetPlugController(ctrl internal.PlugController) {
	s.plugCtrl = ctrl
	s.lifecycle.plugCtrl = ctrl
	s.monitoring.plugCtrl = ctrl
}

// Notifier returns the internal ChargeNotifier so other services (e.g. ScheduleService)
// can send push notifications without creating a second notification channel.
func (s *ChargeSessionService) Notifier() *ChargeNotifier {
	return s.notifier
}

// Locker exposes the shared session lock so other services (e.g. VehicleService)
// can serialise their own check-then-act against session lifecycle transitions.
func (s *ChargeSessionService) Locker() sync.Locker {
	return s.lock
}

// Shutdown closes the SOC worker channel and waits for the worker to finish.
// Call this during graceful shutdown to prevent goroutine leaks.
// Safe to call multiple times.
func (s *ChargeSessionService) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.socWorker.Shutdown()
		s.notifier.Wait()
		slog.Info("ChargeSessionService SOC worker shut down")
	})
}

func (s *ChargeSessionService) GetEnergy(ctx context.Context) (*tasmota.EnergyData, error) {
	return s.monitoring.GetEnergy(ctx)
}

func (s *ChargeSessionService) SetPowerState(ctx context.Context, powerOn bool) error {
	return s.monitoring.SetPowerState(ctx, powerOn)
}

func (s *ChargeSessionService) GetActiveSession(ctx context.Context) (*models.ChargeSession, error) {
	return s.sessionReader.GetActive(ctx)
}

// GetActiveByPlug returns the active session for a specific plug, or nil if none.
func (s *ChargeSessionService) GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error) {
	return s.sessionReader.GetActiveByPlug(ctx, plugID)
}

func (s *ChargeSessionService) FindVehicleByID(ctx context.Context, id string) (*models.Vehicle, error) {
	return s.vehicleRepo.FindByID(ctx, id)
}

func (s *ChargeSessionService) GetActive(ctx context.Context) (*models.ChargeSessionView, error) {
	session, err := s.sessionReader.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	return s.enrichSessionView(ctx, session), nil
}

// GetActiveByVehicle returns the active charge session for a specific vehicle.
func (s *ChargeSessionService) GetActiveByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSessionView, error) {
	session, err := s.sessionReader.GetActiveByVehicle(ctx, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session for vehicle %s: %w", vehicleID, err)
	}
	return s.enrichSessionView(ctx, session), nil
}

// enrichSessionView builds a read-only ChargeSessionView from the stored session,
// overlaying live MQTT energy data and the computed current SoC percent when
// available. It is a pure read: LastBlendedKwh persistence is owned by the
// energy-poller worker (via SOCWorker and CheckAndAutoStopReachingSession) and
// must not be duplicated here - doing so would make GET endpoints perform DB
// writes, violating REST idempotency and adding unnecessary lock contention on
// the hot read path.
func (s *ChargeSessionService) enrichSessionView(ctx context.Context, session *models.ChargeSession) *models.ChargeSessionView {
	if session == nil {
		return nil
	}

	// Pending sessions have no meaningful energy yet; return the bare view.
	if session.Status == models.SessionStatusPending {
		return sessionToView(session)
	}

	var energy *tasmota.EnergyData
	if s.plugCtrl != nil && session.PlugID != nil {
		energy = s.plugCtrl.LastEnergy(*session.PlugID)
	}
	// No live, positive-power reading: the plug isn't reporting fresh energy
	// right now. Report the last persisted blended SoC instead of omitting it,
	// otherwise the gauge regresses to the start percent on a fresh page load
	// until the next poll arrives.
	if energy == nil || energy.Power <= 0 {
		return s.viewWithBlendedPercent(ctx, session)
	}

	view := sessionToView(session)
	view.PowerDraw = &energy.Power
	view.Voltage = &energy.Voltage
	view.Current = &energy.Current

	vehicle, err := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	if err != nil || vehicle == nil || vehicle.CapacityKwh <= 0 || session.StartTotalKwh == nil {
		return view
	}

	wallEnergyAdded := energy.Total - *session.StartTotalKwh
	if wallEnergyAdded > 0 {
		efficiency := vehicle.ChargingEfficiency
		if efficiency <= 0 {
			efficiency = models.DefaultChargingEfficiency
		}
		batteryEnergyAdded := wallEnergyAdded * efficiency
		view.EnergyAddedKwh = &batteryEnergyAdded
	}

	slog.Debug("[SOC] enrichSessionView", "sessionID", session.ID, "startKwh", session.StartKwh, "startTotalKwh", *session.StartTotalKwh, "startPct", session.StartPercent, "targetPct", session.TargetPercent)

	progress := CalculateProgress(session, energy, vehicle)
	slog.Debug("[SOC] enrichSessionView", "sessionID", session.ID, "blendedKwh", progress.BlendedKwh, "currentPercent", progress.CurrentPercent)
	view.CurrentPercent = &progress.CurrentPercent

	return view
}

// viewWithBlendedPercent builds a view whose CurrentPercent is derived from the
// session's last persisted blended kWh, for read paths where no live energy
// reading is available. When the session has no persisted baseline (e.g. a
// freshly activated session that hasn't ticked yet), CurrentPercent is left
// unset and callers fall back to the start percent.
func (s *ChargeSessionService) viewWithBlendedPercent(ctx context.Context, session *models.ChargeSession) *models.ChargeSessionView {
	view := sessionToView(session)
	vehicle, err := s.vehicleRepo.FindByID(ctx, session.VehicleID)
	if err != nil || vehicle == nil {
		return view
	}
	if pct, ok := CurrentPercentFromBlended(session, vehicle); ok {
		view.CurrentPercent = &pct
	}
	return view
}

// sessionToView converts a ChargeSession to a ChargeSessionView.
func sessionToView(s *models.ChargeSession) *models.ChargeSessionView {
	return &models.ChargeSessionView{ChargeSession: *s}
}

// ActivatePending transitions a pending session to active.
func (s *ChargeSessionService) ActivatePending(ctx context.Context, sessionID string) (float64, error) {
	return s.lifecycle.ActivatePending(ctx, sessionID)
}

// CancelPending transitions a pending session to cancelled.
func (s *ChargeSessionService) CancelPending(ctx context.Context, sessionID string) error {
	return s.lifecycle.CancelPending(ctx, sessionID)
}

// CancelPendingIfTimedOut atomically checks for a pending session that has
// exceeded the given timeout and cancels it.
func (s *ChargeSessionService) CancelPendingIfTimedOut(ctx context.Context, timeout time.Duration) (bool, error) {
	return s.lifecycle.CancelPendingIfTimedOut(ctx, timeout)
}

func (s *ChargeSessionService) GetPending(ctx context.Context) (*models.ChargeSession, error) {
	return s.sessionReader.GetPending(ctx)
}

func (s *ChargeSessionService) StartSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent float64) (*models.ChargeSession, error) {
	return s.lifecycle.StartSession(ctx, plugID, vehicleID, startPercent, targetPercent)
}

// StartTwoStageSession starts a ready-by two-stage session. See
// SessionLifecycleService.StartTwoStageSession for details.
func (s *ChargeSessionService) StartTwoStageSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent, holdPercent float64, readyByTime string) (*models.ChargeSession, error) {
	return s.lifecycle.StartTwoStageSession(ctx, plugID, vehicleID, startPercent, targetPercent, holdPercent, readyByTime)
}


type StopResult struct {
	Stopped    bool   `json:"stopped"`
	TasmotaErr string `json:"tasmotaError,omitempty"`
}

// StopWithPercent stops a specific session by ID.
// For use from handlers and tests where the session hasn't been pre-fetched.
func (s *ChargeSessionService) StopWithPercent(ctx context.Context, id string, endPercent float64) (*StopResult, error) {
	return s.lifecycle.StopWithPercent(ctx, id, endPercent)
}

// stopWithPercent stops a specific session using the already-read session object.
// The caller must have already fetched the session to avoid TOCTOU races.
// Implements sessionStopper interface for SessionMonitoringService.
func (s *ChargeSessionService) stopWithPercent(ctx context.Context, session *models.ChargeSession, endPercent float64, reason ...models.StopReason) (*StopResult, error) {
	return s.lifecycle.stopWithPercent(ctx, session, endPercent, reason...)
}

// Stop stops the active charge session, calculating endPercent from Tasmota energy.
// If session is pending, it will be cancelled instead of completed.
func (s *ChargeSessionService) Stop(ctx context.Context) (*StopResult, error) {
	session, err := s.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stop charging: %w", err)
	}
	return s.lifecycle.Stop(ctx, session)
}

// StopByVehicle stops the active charge session for a specific vehicle.
func (s *ChargeSessionService) StopByVehicle(ctx context.Context, vehicleID string) (*StopResult, error) {
	session, err := s.GetActiveByVehicle(ctx, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to stop charging for vehicle %s: %w", vehicleID, err)
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	return s.lifecycle.Stop(ctx, session)
}

func (s *ChargeSessionService) AddPowerReading(ctx context.Context, reading *models.PowerReading) error {
	return s.monitoring.AddPowerReading(ctx, reading)
}

func (s *ChargeSessionService) GetLastCompleted(ctx context.Context) (*models.ChargeSession, error) {
	return s.monitoring.GetLastCompleted(ctx)
}

// CheckAndAutoStopReachingSession checks if the active session has reached its
// target and automatically stops it if so.
func (s *ChargeSessionService) CheckAndAutoStopReachingSession(ctx context.Context) {
	s.monitoring.CheckAndAutoStopReachingSession(ctx, s)
}

// CheckAndStopConditioningSession checks whether a conditioning session has
// tapered to the stop threshold and completes it.
func (s *ChargeSessionService) CheckAndStopConditioningSession(ctx context.Context) {
	s.monitoring.CheckAndStopConditioningSession(ctx, s)
}

// CheckAndResumeHoldingSession resumes a two-stage session held at its
// intermediate percent once it's time to reach the real target by the
// schedule's ready-by deadline.
func (s *ChargeSessionService) CheckAndResumeHoldingSession(ctx context.Context) {
	s.monitoring.CheckAndResumeHoldingSession(ctx)
}

// CheckAndCancelDisconnectedSession cancels an active or conditioning session
// if the plug appears to have been disconnected or switched off.
// Safe to call when Tasmota reports zero power or after consecutive HTTP errors.
func (s *ChargeSessionService) CheckAndCancelDisconnectedSession(ctx context.Context) {
	session, err := s.sessionReader.GetActive(ctx)
	if err != nil || session == nil {
		return
	}
	if session.Status != models.SessionStatusActive && session.Status != models.SessionStatusConditioning {
		return
	}
	// Only cancel if the session was actually charging - avoids cancelling on startup glitches
	if session.LastBlendedKwh == nil || *session.LastBlendedKwh <= epsilonKwh {
		return
	}
	slog.Info("[DISCONNECT] Cancelling session due to power loss", "sessionID", session.ID, "status", session.Status)
	if err := s.lifecycle.CancelActiveSession(ctx, session); err != nil {
		slog.Error("[DISCONNECT] Error cancelling session", "err", err)
	}
}

// CancelActiveSession cancels an active or conditioning session (e.g. due to LWT offline).
// Satisfies mqtt.lwtLifecycle.
func (s *ChargeSessionService) CancelActiveSession(ctx context.Context, session *models.ChargeSession) error {
	return s.lifecycle.CancelActiveSession(ctx, session)
}

// NotifyPlugUnavailable sends a push notification when a plug goes offline.
// Satisfies mqtt.lwtNotifier.
func (s *ChargeSessionService) NotifyPlugUnavailable(ctx context.Context, plug *models.Plug) {
	s.notifier.NotifyPlugUnavailable(ctx, plug)
}

// HandleSensorMessage processes an MQTT SENSOR message for a plug, updating energy
// readings for any active session on that plug.
func (s *ChargeSessionService) HandleSensorMessage(ctx context.Context, plugID string, energy *tasmota.EnergyData) {
	session, err := s.sessionReader.GetActiveByPlug(ctx, plugID)
	if err != nil || session == nil {
		return
	}
	s.monitoring.SaveEnergyReadings(ctx, energy)
}

// UpdateTarget updates the target percent for an active session.
func (s *ChargeSessionService) UpdateTarget(ctx context.Context, sessionID string, newTargetPercent float64) error {
	return s.lifecycle.UpdateTarget(ctx, sessionID, newTargetPercent)
}

// UpdateActiveTarget updates the target percent of the currently active session.
// It resolves the active session itself so the transport layer never has to
// fetch-then-decide; returns ErrSessionNotFound when there is no active session.
func (s *ChargeSessionService) UpdateActiveTarget(ctx context.Context, newTargetPercent float64) error {
	session, err := s.GetActiveSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve active session: %w", err)
	}
	if session == nil {
		return ErrSessionNotFound
	}
	return s.lifecycle.UpdateTarget(ctx, session.ID, newTargetPercent)
}

// UpdateActiveTargetByVehicle updates the target percent of the active session for a specific vehicle.
func (s *ChargeSessionService) UpdateActiveTargetByVehicle(ctx context.Context, vehicleID string, newTargetPercent float64) error {
	session, err := s.GetActiveByVehicle(ctx, vehicleID)
	if err != nil {
		return fmt.Errorf("failed to resolve active session for vehicle %s: %w", vehicleID, err)
	}
	if session == nil {
		return ErrSessionNotFound
	}
	return s.lifecycle.UpdateTarget(ctx, session.ID, newTargetPercent)
}

// DeleteSession deletes a completed or cancelled session.
func (s *ChargeSessionService) DeleteSession(ctx context.Context, id string) error {
	return s.lifecycle.DeleteSession(ctx, id)
}

// StoreSOCSnapshot calculates the SOC from Tasmota energy readings and persists it.
// Used by the polling goroutine in server.go to avoid duplicating the blended Kwh calculation.
func (s *ChargeSessionService) StoreSOCSnapshot(ctx context.Context, session *models.ChargeSession, energy *tasmota.EnergyData) error {
	return s.monitoring.StoreSOCSnapshot(ctx, session, energy)
}

// SaveEnergyReadings atomically checks session status and saves a power reading.
// SOC snapshot is offloaded to an async worker to avoid blocking the poll cycle.
// The mutex ensures the session status check and power reading write are serialised
// with Stop, StartSession, and other session lifecycle mutations.
func (s *ChargeSessionService) SaveEnergyReadings(ctx context.Context, energy *tasmota.EnergyData) {
	s.monitoring.SaveEnergyReadings(ctx, energy)
}

// ProcessSOC handles a single SOC snapshot request.
// Implements SOCProcessor interface.
func (s *ChargeSessionService) ProcessSOC(ctx context.Context, req socRequest) error {
	vehicle, err := s.vehicleRepo.FindByID(ctx, req.vehicleID)
	if err != nil {
		return fmt.Errorf("failed to find vehicle %s for SOC processing: %w", req.vehicleID, err)
	}
	if vehicle == nil || vehicle.CapacityKwh <= 0 {
		return nil
	}

	session := &models.ChargeSession{
		ID:            req.sessionID,
		VehicleID:     req.vehicleID,
		CreatedAt:     req.createdAt,
		StartedAt:     req.startedAt,
		StartKwh:      req.startKwh,
		TargetKwh:     req.targetKwh,
		StartTotalKwh: &req.startTotalKwh,
		LastBlendedKwh: req.lastBlendedKwh,
	}

	socPercent, lastBlendedKwh, err := s.socGenerator.CalculateSOC(session, req.energy, vehicle)
	if err != nil {
		return fmt.Errorf("failed to calculate SOC for session %s: %w", req.sessionID, err)
	}

	snapshot := s.socGenerator.BuildSnapshot(req.sessionID, socPercent)

	if err := s.snapshotRepo.CreateSOCSnapshot(ctx, snapshot); err != nil {
		return err
	}

	return s.sessionWriter.UpdateLastBlendedKwh(ctx, req.sessionID, lastBlendedKwh)
}
