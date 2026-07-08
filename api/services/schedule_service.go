package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/chargeestimate"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"

	"github.com/google/uuid"
)

var (
	ErrInvalidScheduleTime    = errors.New("invalid schedule time format, expected HH:MM")
	ErrInvalidScheduleType    = errors.New("invalid schedule type")
	ErrWindowRequired         = errors.New("windowStart and windowEnd are required for carbon_aware schedule")
	ErrWindowEqual            = errors.New("windowStart and windowEnd must differ")
	ErrMaintenancePlugSchedule = errors.New("schedules are not supported for maintenance plugs")
	ErrReadyByEqualsTime      = errors.New("readyBy must differ from time")
)

// scheduleThrottleDuration is the minimum time between schedule activations across all plugs.
const scheduleThrottleDuration = 60 * time.Second

// forecastBucketSize is the granularity of carbon intensity forecast buckets.
const forecastBucketSize = 30 * time.Minute

// DurationEstimator estimates how many minutes it will take to charge a vehicle.
// Injected as a function type for easy stubbing in tests.
type DurationEstimator func(v *models.Vehicle, current, target float64) (int, error)

// ChargeServiceAdapter exposes the charge session methods needed by ScheduleService.
type ChargeServiceAdapter interface {
	GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error)
	StartSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent float64) (*models.ChargeSession, error)
	StartTwoStageSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent, holdPercent float64, readyByTime string) (*models.ChargeSession, error)
}

// chargeSessionServiceAdapter wraps *ChargeSessionService to satisfy ChargeServiceAdapter.
type chargeSessionServiceAdapter struct {
	svc *ChargeSessionService
}

func (a *chargeSessionServiceAdapter) GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error) {
	return a.svc.GetActiveByPlug(ctx, plugID)
}

func (a *chargeSessionServiceAdapter) StartSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent float64) (*models.ChargeSession, error) {
	return a.svc.StartSession(ctx, plugID, vehicleID, startPercent, targetPercent)
}

func (a *chargeSessionServiceAdapter) StartTwoStageSession(ctx context.Context, plugID, vehicleID string, startPercent, targetPercent, holdPercent float64, readyByTime string) (*models.ChargeSession, error) {
	return a.svc.StartTwoStageSession(ctx, plugID, vehicleID, startPercent, targetPercent, holdPercent, readyByTime)
}

// scheduleNowFunc returns the current time. Overridable in tests.
var scheduleNowFunc = time.Now

type ScheduleService struct {
	repo             internal.ScheduleRepo
	plugRepo         internal.PlugRepo
	vehicleRepo      internal.VehicleRepo
	chargeService    ChargeServiceAdapter
	lastActivationMu sync.Mutex
	lastActivation   time.Time

	// Carbon-aware deps - set via SetCarbonAwareDeps after construction.
	forecaster internal.CarbonForecaster
	estimator  DurationEstimator
	notifier   *ChargeNotifier
}

func NewScheduleService(
	repo internal.ScheduleRepo,
	plugRepo internal.PlugRepo,
	vehicleRepo internal.VehicleRepo,
	chargeService *ChargeSessionService,
) *ScheduleService {
	return &ScheduleService{
		repo:          repo,
		plugRepo:      plugRepo,
		vehicleRepo:   vehicleRepo,
		chargeService: &chargeSessionServiceAdapter{svc: chargeService},
	}
}

// NewScheduleServiceWithAdapter creates a ScheduleService with a custom ChargeServiceAdapter.
// Used in tests to inject mock adapters.
func NewScheduleServiceWithAdapter(
	repo internal.ScheduleRepo,
	plugRepo internal.PlugRepo,
	vehicleRepo internal.VehicleRepo,
	chargeService ChargeServiceAdapter,
) *ScheduleService {
	return &ScheduleService{
		repo:          repo,
		plugRepo:      plugRepo,
		vehicleRepo:   vehicleRepo,
		chargeService: chargeService,
	}
}

// SetCarbonAwareDeps injects optional carbon-aware scheduling dependencies.
// Called from server.go after construction; not needed for daily schedules.
func (s *ScheduleService) SetCarbonAwareDeps(f internal.CarbonForecaster, est DurationEstimator, n *ChargeNotifier) {
	s.forecaster = f
	s.estimator = est
	s.notifier = n
}

// rejectMaintenancePlug returns ErrMaintenancePlugSchedule if the plug is a maintenance plug.
func (s *ScheduleService) rejectMaintenancePlug(ctx context.Context, plugID string) error {
	plug, err := s.plugRepo.FindByID(ctx, plugID)
	if err != nil {
		return fmt.Errorf("schedule service: find plug: %w", err)
	}
	if plug != nil && plug.Type == models.PlugTypeMaintenance {
		return ErrMaintenancePlugSchedule
	}
	return nil
}

// UpsertByPlugID creates or updates a daily schedule for a specific plug. readyBy is
// optional: when set, it enables two-stage charging (see tryActivateDaily) - the
// vehicle charges to 80% of its target, holds, then resumes to reach 100% of target
// by readyBy.
func (s *ScheduleService) UpsertByPlugID(ctx context.Context, plugID, userID, scheduleTime string, readyBy *string, enabled bool) (*models.Schedule, error) {
	if !isValidTimeFormat(scheduleTime) {
		return nil, ErrInvalidScheduleTime
	}

	if readyBy != nil {
		if !isValidTimeFormat(*readyBy) {
			return nil, ErrInvalidScheduleTime
		}
		if *readyBy == scheduleTime {
			return nil, ErrReadyByEqualsTime
		}
	}

	if userID == "" {
		return nil, ErrUserIDRequired
	}

	if err := s.rejectMaintenancePlug(ctx, plugID); err != nil {
		return nil, err
	}

	schedule := &models.Schedule{
		ID:      uuid.New().String(),
		PlugID:  &plugID,
		Time:    scheduleTime,
		ReadyBy: readyBy,
		Type:    models.ScheduleTypeDaily,
		Enabled: enabled,
		UserID:  &userID,
	}

	if err := s.repo.UpsertByPlugID(ctx, schedule); err != nil {
		return nil, err
	}

	// Re-fetch to get the actual persisted ID after ON CONFLICT update.
	persisted, err := s.repo.GetByPlugID(ctx, plugID)
	if err != nil {
		return nil, err
	}
	if persisted != nil {
		return persisted, nil
	}
	return schedule, nil
}

// UpsertCarbonAware creates or updates a carbon-aware schedule for a specific plug.
func (s *ScheduleService) UpsertCarbonAware(ctx context.Context, plugID, userID, windowStart, windowEnd string, enabled bool) (*models.Schedule, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	if err := s.rejectMaintenancePlug(ctx, plugID); err != nil {
		return nil, err
	}

	if !isValidTimeFormat(windowStart) || !isValidTimeFormat(windowEnd) {
		return nil, ErrWindowRequired
	}
	if windowStart == windowEnd {
		return nil, ErrWindowEqual
	}

	schedule := &models.Schedule{
		ID:          uuid.New().String(),
		PlugID:      &plugID,
		Type:        models.ScheduleTypeCarbonAware,
		Time:        windowStart, // stored in time column for schema compat
		WindowStart: &windowStart,
		WindowEnd:   &windowEnd,
		Enabled:     enabled,
		UserID:      &userID,
	}

	if err := s.repo.UpsertByPlugID(ctx, schedule); err != nil {
		return nil, err
	}

	persisted, err := s.repo.GetByPlugID(ctx, plugID)
	if err != nil {
		return nil, err
	}
	if persisted != nil {
		return persisted, nil
	}
	return schedule, nil
}

// GetByPlugID returns the schedule for a plug, or nil if none exists.
func (s *ScheduleService) GetByPlugID(ctx context.Context, plugID string) (*models.Schedule, error) {
	return s.repo.GetByPlugID(ctx, plugID)
}

// CheckAndActivateAll iterates all plug schedules and starts sessions when appropriate.
func (s *ScheduleService) CheckAndActivateAll(ctx context.Context) {
	schedules, err := s.repo.ListAll(ctx)
	if err != nil {
		slog.Error("schedule: listing schedules", "err", err)
		return
	}

	now := scheduleNowFunc()

	s.lastActivationMu.Lock()
	lastAct := s.lastActivation
	s.lastActivationMu.Unlock()

	for _, sch := range schedules {
		if sch.PlugID == nil || !sch.Enabled {
			continue
		}
		plugID := *sch.PlugID

		switch sch.Type {
		case models.ScheduleTypeCarbonAware:
			s.tryActivateCarbonAware(ctx, sch, plugID, now, lastAct)
		default: // "daily" and unset (legacy rows)
			s.tryActivateDaily(ctx, sch, plugID, now, lastAct)
		}
	}
}

// tryActivateDaily fires a charge session when the current HH:MM matches the schedule time.
// When sch.ReadyBy is set, activates two-stage charging: to 80% of the vehicle's
// target now, holding there until CheckAndResumeHoldingSession resumes it in time
// to reach 100% of target by ReadyBy. If the vehicle is already past the 80% mark,
// there's nothing to hold for, so it falls back to a normal single-stage charge.
func (s *ScheduleService) tryActivateDaily(ctx context.Context, sch models.Schedule, plugID string, now time.Time, lastAct time.Time) {
	if sch.Time != formatTime(now) {
		return
	}
	if now.Sub(lastAct) < scheduleThrottleDuration {
		slog.Warn("schedule: throttle active", "plugID", plugID, "lastActivationAgo", now.Sub(lastAct).Round(time.Second))
		return
	}

	cand := s.loadCandidate(ctx, plugID)
	if cand == nil {
		return
	}

	var sess *models.ChargeSession
	var err error
	if holdPercent := cand.vehicle.TargetPercent * models.TwoStageHoldFraction; sch.ReadyBy != nil && holdPercent > cand.vehicle.CurrentPercent {
		sess, err = s.chargeService.StartTwoStageSession(ctx, plugID, cand.vehicle.ID, cand.vehicle.CurrentPercent, cand.vehicle.TargetPercent, holdPercent, *sch.ReadyBy)
	} else {
		sess, err = s.chargeService.StartSession(ctx, plugID, cand.vehicle.ID, cand.vehicle.CurrentPercent, cand.vehicle.TargetPercent)
	}
	if err != nil {
		if errors.Is(err, ErrActiveSessionExists) {
			return
		}
		slog.Error("schedule: daily start failed", "plugID", plugID, "vehicleID", cand.vehicle.ID, "err", err)
		return
	}
	_ = sess

	s.lastActivationMu.Lock()
	s.lastActivation = now
	s.lastActivationMu.Unlock()
	slog.Info("schedule: daily activated charge", "plugID", plugID, "vehicleID", cand.vehicle.ID)
}

// tryActivateCarbonAware runs the carbon-aware optimizer for a single schedule.
func (s *ScheduleService) tryActivateCarbonAware(ctx context.Context, sch models.Schedule, plugID string, now time.Time, lastAct time.Time) {
	if sch.WindowStart == nil || sch.WindowEnd == nil {
		slog.Warn("schedule: carbon_aware missing window", "plugID", plugID)
		return
	}

	windowStart, windowEnd, err := resolveWindow(now, *sch.WindowStart, *sch.WindowEnd)
	if err != nil {
		slog.Error("schedule: resolveWindow failed", "plugID", plugID, "err", err)
		return
	}

	// Not inside window - wait.
	if now.Before(windowStart) || !now.Before(windowEnd) {
		return
	}

	cand := s.loadCandidate(ctx, plugID)
	if cand == nil {
		return
	}

	// Step 1: Estimate charge duration.
	est := s.estimator
	if est == nil {
		est = chargeestimate.EstimateMinutes
	}
	d, estimateErr := est(cand.vehicle, cand.vehicle.CurrentPercent, cand.vehicle.TargetPercent)

	if estimateErr != nil {
		// Failsafe: start immediately, throttle-exempt.
		slog.Warn("schedule: carbon_aware estimator error, starting now", "plugID", plugID, "err", estimateErr)
		s.startCarbonAwareSession(ctx, plugID, cand.vehicle, now, windowEnd, 0, true)
		return
	}

	// Step 2: Deadline guard - must start now if we've hit latestStart.
	latestStart := windowEnd.Add(-time.Duration(d) * time.Minute)
	if !now.Before(latestStart) {
		slog.Info("schedule: carbon_aware deadline guard, starting now", "plugID", plugID, "latestStart", latestStart)
		s.startCarbonAwareSession(ctx, plugID, cand.vehicle, now, windowEnd, d, true)
		return
	}

	// Step 3: Throttle check for non-forced path.
	if now.Sub(lastAct) < scheduleThrottleDuration {
		return
	}

	// Step 4: Forecast - defer if unavailable.
	if s.forecaster == nil {
		return
	}
	buckets, ferr := s.forecaster.GetForecast(ctx, now, windowEnd)
	if ferr != nil || len(buckets) == 0 {
		slog.Warn("schedule: carbon_aware forecast unavailable, deferring", "plugID", plugID, "err", ferr)
		return
	}

	// Score candidate start times and pick the cleanest window.
	optimalStart := findOptimalStart(buckets, now, latestStart, time.Duration(d)*time.Minute)
	currentBucket := alignToHalfHour(now.UTC())

	if optimalStart.IsZero() || !optimalStart.After(currentBucket) {
		// Optimal start is the current bucket - start now.
		s.startCarbonAwareSession(ctx, plugID, cand.vehicle, now, windowEnd, d, false)
	}
	// else: a cleaner window exists later - wait.
}

// EstimateCarbonAwareStart computes when an enabled carbon-aware schedule's charge
// session is expected to start, based on the current carbon intensity forecast. This
// mirrors the decision logic in tryActivateCarbonAware but only reports the answer -
// it never starts a session. Returns ok=false when no confident estimate can be made
// (missing deps, forecast unavailable, vehicle already at target, etc.), so callers can
// fall back to displaying the ready-by (windowEnd) time instead.
func (s *ScheduleService) EstimateCarbonAwareStart(ctx context.Context, sch *models.Schedule) (start string, ok bool) {
	if sch == nil || sch.Type != models.ScheduleTypeCarbonAware || !sch.Enabled {
		return "", false
	}
	if sch.PlugID == nil || sch.WindowStart == nil || sch.WindowEnd == nil {
		return "", false
	}
	if s.forecaster == nil {
		return "", false
	}

	now := scheduleNowFunc()
	windowStart, windowEnd, err := resolveWindow(now, *sch.WindowStart, *sch.WindowEnd)
	if err != nil {
		return "", false
	}

	cand := s.loadCandidate(ctx, *sch.PlugID)
	if cand == nil {
		return "", false
	}

	est := s.estimator
	if est == nil {
		est = chargeestimate.EstimateMinutes
	}
	d, estimateErr := est(cand.vehicle, cand.vehicle.CurrentPercent, cand.vehicle.TargetPercent)
	if estimateErr != nil {
		return "", false
	}

	latestStart := windowEnd.Add(-time.Duration(d) * time.Minute)

	// If we're already inside the window, only future starts matter. Before the
	// window opens, scan the whole window from its start.
	searchFrom := windowStart
	if now.After(windowStart) {
		searchFrom = now
	}
	if !searchFrom.Before(latestStart) {
		return formatTime(latestStart.In(now.Location())), true
	}

	buckets, ferr := s.forecaster.GetForecast(ctx, searchFrom, windowEnd)
	if ferr != nil || len(buckets) == 0 {
		return "", false
	}

	optimalStart := findOptimalStart(buckets, searchFrom, latestStart, time.Duration(d)*time.Minute)
	if optimalStart.IsZero() {
		return "", false
	}

	return formatTime(optimalStart.In(now.Location())), true
}

// candidate holds the resolved plug and vehicle for an activation check.
type candidate struct {
	vehicle *models.Vehicle
}

// loadCandidate resolves plug and vehicle for the given plug ID, enforcing all
// preconditions that are shared between daily and carbon-aware paths.
// Returns nil when the schedule should be skipped (soft skip, not an error).
func (s *ScheduleService) loadCandidate(ctx context.Context, plugID string) *candidate {
	active, err := s.chargeService.GetActiveByPlug(ctx, plugID)
	if err != nil {
		slog.Error("schedule: checking active session", "plugID", plugID, "err", err)
		return nil
	}
	if active != nil {
		slog.Warn("schedule: plug already has active session", "plugID", plugID, "sessionID", active.ID)
		return nil
	}

	plug, err := s.plugRepo.FindByID(ctx, plugID)
	if err != nil || plug == nil || plug.VehicleID == nil {
		slog.Warn("schedule: plug has no vehicle assigned", "plugID", plugID)
		return nil
	}
	vehicle, err := s.vehicleRepo.FindByID(ctx, *plug.VehicleID)
	if err != nil || vehicle == nil {
		slog.Warn("schedule: vehicle not found for plug", "plugID", plugID, "vehicleID", *plug.VehicleID)
		return nil
	}
	if vehicle.CurrentPercent >= vehicle.TargetPercent {
		slog.Warn("schedule: vehicle already at target", "plugID", plugID, "vehicleID", vehicle.ID)
		return nil
	}

	return &candidate{vehicle: vehicle}
}

// startCarbonAwareSession starts a charge session and optionally notifies of a shortfall.
// throttleExempt=true skips the throttle check (used for forced/failsafe starts).
func (s *ScheduleService) startCarbonAwareSession(ctx context.Context, plugID string, vehicle *models.Vehicle, now, windowEnd time.Time, dMin int, throttleExempt bool) {
	sess, err := s.chargeService.StartSession(ctx, plugID, vehicle.ID, vehicle.CurrentPercent, vehicle.TargetPercent)
	if err != nil {
		if errors.Is(err, ErrActiveSessionExists) {
			return
		}
		slog.Error("schedule: carbon_aware start failed", "plugID", plugID, "vehicleID", vehicle.ID, "err", err)
		return
	}

	s.lastActivationMu.Lock()
	s.lastActivation = now
	s.lastActivationMu.Unlock()
	slog.Info("schedule: carbon_aware activated charge", "plugID", plugID, "vehicleID", vehicle.ID, "throttleExempt", throttleExempt)

	if s.notifier != nil && dMin > 0 {
		availableMin := int(windowEnd.Sub(now).Minutes())
		if availableMin < dMin {
			projPercent := chargeestimate.ProjectPercent(vehicle, vehicle.CurrentPercent, availableMin)
			readyBy := formatTime(windowEnd.In(now.Location()))
			s.notifier.NotifyShortfallProjected(ctx, sess, projPercent, vehicle.TargetPercent, readyBy)
		}
	}
}

// resolveWindow converts HH:MM window strings to absolute timestamps relative to now.
// Handles midnight crossing (windowEnd < windowStart → add 24h to end).
// If now is past the window end, rolls both forward 24h so the caller sees "before window".
func resolveWindow(now time.Time, windowStart, windowEnd string) (start, end time.Time, err error) {
	startH, startM, err := parseHHMM(windowStart)
	if err != nil {
		return
	}
	endH, endM, err := parseHHMM(windowEnd)
	if err != nil {
		return
	}

	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	start = today.Add(time.Duration(startH)*time.Hour + time.Duration(startM)*time.Minute)
	end = today.Add(time.Duration(endH)*time.Hour + time.Duration(endM)*time.Minute)

	// Midnight crossing: windowEnd wraps to the next day.
	if !end.After(start) {
		end = end.Add(24 * time.Hour)
	}

	// Window already passed today - roll forward 24h so we fall into "before window".
	if !now.Before(end) {
		start = start.Add(24 * time.Hour)
		end = end.Add(24 * time.Hour)
	}

	return
}

// resolveDeadline converts an HH:MM string to the next absolute timestamp at or
// after now - single-timestamp counterpart to resolveWindow, used to resolve a
// holding session's ready-by time relative to the current poll tick.
func resolveDeadline(now time.Time, hhmm string) (time.Time, error) {
	h, m, err := parseHHMM(hhmm)
	if err != nil {
		return time.Time{}, err
	}

	loc := now.Location()
	deadline := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	if deadline.Before(now) {
		deadline = deadline.Add(24 * time.Hour)
	}
	return deadline, nil
}

// parseHHMM parses a "HH:MM" string into hour and minute integers.
func parseHHMM(s string) (h, m int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid HH:MM: %q", s)
	}
	h, err = strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err = strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", s)
	}
	return
}

// alignToHalfHour truncates t to the nearest 30-minute boundary (UTC).
func alignToHalfHour(t time.Time) time.Time {
	return t.UTC().Truncate(forecastBucketSize)
}

// findOptimalStart picks the 30-minute start time in [now, latestStart] with the
// lowest time-weighted average gCO2/kWh over the [start, start+d] window.
// Returns the zero Time if no valid candidate is found.
func findOptimalStart(buckets []carbonintensity.ForecastBucket, now, latestStart time.Time, d time.Duration) time.Time {
	nowUTC := now.UTC()
	latestUTC := latestStart.UTC()

	var bestStart time.Time
	bestScore := math.MaxFloat64

	for candidate := alignToHalfHour(nowUTC); !candidate.After(latestUTC); candidate = candidate.Add(forecastBucketSize) {
		score := scoreWindow(buckets, candidate, candidate.Add(d))
		if score < bestScore {
			bestScore = score
			bestStart = candidate
		}
	}

	return bestStart
}

// scoreWindow computes the time-weighted average gCO2/kWh over [start, end] using
// the provided forecast buckets. Returns math.MaxFloat64 when no buckets overlap.
func scoreWindow(buckets []carbonintensity.ForecastBucket, start, end time.Time) float64 {
	var totalWeight, weightedSum float64

	for _, b := range buckets {
		overlapStart := b.From
		if start.After(overlapStart) {
			overlapStart = start
		}
		overlapEnd := b.To
		if end.Before(overlapEnd) {
			overlapEnd = end
		}
		if !overlapStart.Before(overlapEnd) {
			continue
		}
		weight := overlapEnd.Sub(overlapStart).Minutes()
		totalWeight += weight
		weightedSum += float64(b.ForecastGCo2) * weight
	}

	if totalWeight <= 0 {
		return math.MaxFloat64
	}
	return weightedSum / totalWeight
}

func formatTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

func isValidTimeFormat(t string) bool {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return false
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return false
	}
	return len(parts[0]) == 2 && len(parts[1]) == 2
}

// GetLastActivation returns the time of the last schedule activation.
func (s *ScheduleService) GetLastActivation() time.Time {
	s.lastActivationMu.Lock()
	defer s.lastActivationMu.Unlock()
	return s.lastActivation
}

// SetLastActivation sets the last activation time (used in tests).
func (s *ScheduleService) SetLastActivation(t time.Time) {
	s.lastActivationMu.Lock()
	defer s.lastActivationMu.Unlock()
	s.lastActivation = t
}
