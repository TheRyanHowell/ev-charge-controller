package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/chargeestimate"
	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupScheduleServiceTest(t *testing.T) (*ScheduleService, *sql.DB, *ChargeSessionService) {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)

	// Seed user + vehicle instances (model IDs used as instance IDs for simplicity).
	seedTestUser(t, db)
	seedTestVehicle(t, db)
	now := time.Now()
	_, err = db.Exec(`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"rm1", testUserID, "rm1", "Schedule RM1", 20.0, 80.0, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"rm1s", testUserID, "rm1s", "Schedule RM1S", 30.0, 70.0, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic, vehicle_id) VALUES (?, ?, ?, ?, ?, ?)`,
		testPlugID, testUserID, "Test Plug", "ns-testdefault", "test-topic", "rm1")
	require.NoError(t, err)

	vehicleRepo := repository.NewVehicleRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	chargeService := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer chargeService.Shutdown()

	return NewScheduleService(scheduleRepo, plugRepo, vehicleRepo, chargeService), db, chargeService
}

func TestScheduleService_UpsertByPlugID(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	schedule, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", nil, true)
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.Equal(t, "03:00", schedule.Time)
	assert.True(t, schedule.Enabled)
}

func TestScheduleService_UpsertByPlugID_InvalidTime(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	for _, invalidTime := range []string{"25:00", "3:00", "03:60", "abc", "", "03:00:00", "24:00"} {
		_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, invalidTime, nil, true)
		assert.ErrorIs(t, err, ErrInvalidScheduleTime, "expected error for time: %s", invalidTime)
	}
}

func TestScheduleService_UpsertByPlugID_EmptyUserID(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := service.UpsertByPlugID(t.Context(), testPlugID, "", "03:00", nil, true)
	assert.ErrorIs(t, err, ErrUserIDRequired)
}

func TestScheduleService_UpsertByPlugID_UpdateExisting(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", nil, true)
	require.NoError(t, err)

	s2, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "04:00", nil, false)
	require.NoError(t, err)

	assert.Equal(t, "04:00", s2.Time)
	assert.False(t, s2.Enabled)
}

func TestScheduleService_UpsertByPlugID_ReadyBy(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	readyBy := "07:00"
	schedule, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", &readyBy, true)
	require.NoError(t, err)
	require.NotNil(t, schedule)
	require.NotNil(t, schedule.ReadyBy)
	assert.Equal(t, "07:00", *schedule.ReadyBy)
}

func TestScheduleService_UpsertByPlugID_ReadyByInvalidFormat(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	invalid := "25:00"
	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", &invalid, true)
	assert.ErrorIs(t, err, ErrInvalidScheduleTime)
}

func TestScheduleService_UpsertByPlugID_ReadyByEqualsTime(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	sameTime := "03:00"
	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", &sameTime, true)
	assert.ErrorIs(t, err, ErrReadyByEqualsTime)
}

func TestScheduleService_GetByPlugID(t *testing.T) {
	service, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	schedule, err := service.GetByPlugID(t.Context(), testPlugID)
	require.NoError(t, err)
	assert.Nil(t, schedule)

	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", nil, true)
	require.NoError(t, err)

	schedule, err = service.GetByPlugID(t.Context(), testPlugID)
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.Equal(t, "03:00", schedule.Time)
}

func TestScheduleService_CheckAndActivateAll_NoSchedule(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	// No schedule exists - should not crash
	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestScheduleService_CheckAndActivateAll_Disabled(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	currentTime := formatTime(time.Now())
	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, false)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestScheduleService_CheckAndActivateAll_TimeMismatch(t *testing.T) {
	service, db, chargeSvc := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "23:59", nil, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeSvc.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestScheduleService_CheckAndActivateAll_NoVehicleOnPlug(t *testing.T) {
	service, db, chargeSvc := setupScheduleServiceTest(t)
	defer db.Close()

	// Remove vehicle assignment from the plug
	_, err := db.Exec(`UPDATE plugs SET vehicle_id = NULL WHERE id = ?`, testPlugID)
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeSvc.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestScheduleService_CheckAndActivateAll_SkipWhenActiveSession(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	currentTime := formatTime(time.Now())
	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, true)
	require.NoError(t, err)

	// Create an active session for this plug to block activation
	plugID := testPlugID
	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    &plugID,
		StartKwh:  0.5,
		TargetKwh: 1.5,
		Status:    "active",
	}
	err = chargeService.sessionWriter.Create(t.Context(), session)
	require.NoError(t, err)

	var countBefore int
	err = db.QueryRow("SELECT COUNT(*) FROM charge_sessions WHERE status = 'active'").Scan(&countBefore)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	var countAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM charge_sessions WHERE status = 'active'").Scan(&countAfter)
	require.NoError(t, err)
	assert.Equal(t, countBefore, countAfter)
}

func TestScheduleService_CheckAndActivateAll_SkipWhenAtTarget(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	// Set vehicle to already be at target
	_, err := db.Exec(`UPDATE vehicles SET current_percent = 80.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestScheduleService_CheckAndActivateAll_Throttle(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	currentTime := formatTime(time.Now())
	_, err := service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, true)
	require.NoError(t, err)

	// Set last activation to 30s ago (within 60s throttle window)
	service.SetLastActivation(time.Now().Add(-30 * time.Second))

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)
}

func TestScheduleService_CheckAndActivateAll_HappyPath(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	// Ensure vehicle has current < target
	_, err := db.Exec(`UPDATE vehicles SET current_percent = 20.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, true)
	require.NoError(t, err)

	// Verify no active session before
	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active)

	service.CheckAndActivateAll(t.Context())

	// Verify a session was created for the plug
	active, err = chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected an active charge session to be created")
	assert.Equal(t, "rm1", active.VehicleID)

	// Verify throttle was set
	assert.WithinDuration(t, time.Now(), service.GetLastActivation(), 2*time.Second)
}

// TestScheduleService_Daily_TwoStage_MinDurationBoundary pins the exact
// boundary of the worthwhileTwoStage guard using a mock estimator (fixed
// return value, independent of chargeestimate internals): d2 just below the
// threshold falls back to single-stage, exactly at and just above it stays
// two-stage - confirming the guard uses >= not >.
func TestScheduleService_Daily_TwoStage_MinDurationBoundary(t *testing.T) {
	tests := []struct {
		name           string
		d2             int
		expectTwoStage bool
	}{
		{"below threshold", models.MinTwoStageStageDurationMin - 1, false},
		{"at threshold", models.MinTwoStageStageDurationMin, true},
		{"above threshold", models.MinTwoStageStageDurationMin + 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
			plugID := testPlugID
			readyBy := "23:59"
			scheduleRepo.listAllResult = []models.Schedule{
				{PlugID: &plugID, Time: formatTime(mockNow), ReadyBy: &readyBy, Enabled: true},
			}
			plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
			vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
			chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}
			chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

			svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
				return tt.d2, nil
			}, nil)
			svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return mockNow }
			t.Cleanup(func() { scheduleNowFunc = old })

			svc.CheckAndActivateAll(t.Context())
			assert.Equal(t, tt.expectTwoStage, chargeAdapter.twoStageCalled)
			assert.Equal(t, !tt.expectTwoStage, chargeAdapter.createPendingCalled)
		})
	}
}

func TestScheduleService_CheckAndActivateAll_TwoStage_HoldsAtEightyPercentOfTarget(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	// current=20, target=80 -> hold = 0.8*80 = 64, well above current.
	_, err := db.Exec(`UPDATE vehicles SET current_percent = 20.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	readyBy := "23:59"
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, &readyBy, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected a two-stage session to be created")
	require.NotNil(t, active.HoldPercent)
	assert.Equal(t, 64.0, *active.HoldPercent)
	require.NotNil(t, active.ReadyByTime)
	assert.Equal(t, "23:59", *active.ReadyByTime)
	assert.Equal(t, 80.0, active.TargetPercent)
}

func TestScheduleService_CheckAndActivateAll_TwoStage_SkipsHoldWhenAlreadyPastEightyPercent(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	// current=70, target=80 -> hold = 64, already below current: nothing to hold for.
	_, err := db.Exec(`UPDATE vehicles SET current_percent = 70.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	readyBy := "23:59"
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, &readyBy, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected a single-stage session to be created")
	assert.Nil(t, active.HoldPercent)
	assert.Nil(t, active.ReadyByTime)
	assert.Equal(t, 80.0, active.TargetPercent)
}

// TestScheduleService_CheckAndActivateAll_TwoStage_SkipsWhenStage2TooShort is a
// regression test for two-stage triggering on degenerate splits: with
// current=1, target=2, hold=1.6 - stage 2 (1.6->2.0) is only a couple of
// minutes, far too short to justify a relay power-cycle and hold/resume
// transition. Uses the real seeded RM1 vehicle spec (2.026kWh/600W/0.8 eff),
// so the duration numbers are grounded, not assumed.
func TestScheduleService_CheckAndActivateAll_TwoStage_SkipsWhenStage2TooShort(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := db.Exec(`UPDATE vehicles SET current_percent = 1.0, target_percent = 2.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	readyBy := "23:59"
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, &readyBy, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected a single-stage session to be created")
	assert.Nil(t, active.HoldPercent, "stage 2 duration is too short to be worthwhile")
	assert.Nil(t, active.ReadyByTime)
	assert.Equal(t, 2.0, active.TargetPercent)
}

func TestScheduleService_CheckAndActivateAll_NoReadyBy_StartsSingleStage(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := db.Exec(`UPDATE vehicles SET current_percent = 20.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, nil, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Nil(t, active.HoldPercent)
	assert.Nil(t, active.ReadyByTime)
}

func TestResolveDeadline(t *testing.T) {
	now := time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC)

	// Later today.
	deadline, err := resolveDeadline(now, "23:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC), deadline)

	// Earlier clock time than now - rolls forward to tomorrow.
	deadline, err = resolveDeadline(now, "07:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 7, 0, 0, 0, time.UTC), deadline)

	// Invalid format.
	_, err = resolveDeadline(now, "not-a-time")
	assert.Error(t, err)
}

// TestResolveDeadline_MidnightBoundary characterizes resolveDeadline's
// "next occurrence of HH:MM at or after now" semantics across the midnight
// boundary. This mechanism is deliberately unchanged by the two-stage
// resume audit - resolveDeadline is re-resolved fresh on every
// CheckAndResumeHoldingSession tick (every PollIntervalSec=5s), so as long
// as the deadline guard fires before now crosses the literal deadline
// clock-time, the "roll forward a full day" behavior below is never
// actually reached in practice. These tests pin the mechanism as correct,
// not as a bug fix.
func TestResolveDeadline_MidnightBoundary(t *testing.T) {
	// Deadline is later tonight, just before midnight - no roll needed.
	now := time.Date(2024, 1, 1, 23, 30, 0, 0, time.UTC)
	deadline, err := resolveDeadline(now, "00:15")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 0, 15, 0, 0, time.UTC), deadline, "hhmm is 45min away, tomorrow's clock time")

	// now is just after midnight, deadline is later the same (new) day - no roll.
	now = time.Date(2024, 1, 2, 0, 5, 0, 0, time.UTC)
	deadline, err = resolveDeadline(now, "00:15")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 0, 15, 0, 0, time.UTC), deadline, "hhmm is 10min away, same day")

	// now is just after the deadline clock-time already passed today - rolls
	// forward a full day. This is *correct* next-occurrence behavior, not a
	// bug: at 00:20 the 00:15 deadline for today has already gone by, so the
	// next valid occurrence is tomorrow's 00:15.
	now = time.Date(2024, 1, 2, 0, 20, 0, 0, time.UTC)
	deadline, err = resolveDeadline(now, "00:15")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 3, 0, 15, 0, 0, time.UTC), deadline, "00:15 already passed 5 minutes ago - correctly rolls to tomorrow, not treated as in the past")

	// now exactly equals the deadline clock-time - no roll (0 minutes away,
	// not "already passed").
	now = time.Date(2024, 1, 2, 0, 15, 0, 0, time.UTC)
	deadline, err = resolveDeadline(now, "00:15")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 0, 15, 0, 0, time.UTC), deadline, "now == hhmm should not trigger a spurious roll")
}

func TestIsValidTimeFormat(t *testing.T) {
	for _, validTime := range []string{"00:00", "23:59", "03:30", "12:00"} {
		assert.True(t, isValidTimeFormat(validTime), "expected valid: %s", validTime)
	}

	for _, invalidTime := range []string{"25:00", "03:60", "3:00", "03:0", "abc", "", "03:00:00", "24:00"} {
		assert.False(t, isValidTimeFormat(invalidTime), "expected invalid: %s", invalidTime)
	}
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2024, 1, 1, 9, 5, 0, 0, time.UTC)
	assert.Equal(t, "09:05", formatTime(ts))

	ts = time.Date(2024, 1, 1, 9, 50, 0, 0, time.UTC)
	assert.Equal(t, "09:50", formatTime(ts))

	ts = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "00:00", formatTime(ts))

	ts = time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC)
	assert.Equal(t, "14:30", formatTime(ts))
}

// TestScheduleService_CheckAndActivateAll_EarlyMorningHour is a regression test
// for the bug where formatTime() produced "9:05" (no leading zero) but the
// schedule stored "09:05" (zero-padded). The comparison always failed for
// hours 0-9, so schedules never activated in the morning.
func TestScheduleService_CheckAndActivateAll_EarlyMorningHour(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	// Ensure vehicle has current < target and plug has vehicle assigned
	_, err := db.Exec(`UPDATE vehicles SET current_percent = 20.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	// Set schedule to "09:05" - zero-padded
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, "09:05", nil, true)
	require.NoError(t, err)

	// Verify formatTime produces zero-padded hour
	now := time.Date(2024, 1, 1, 9, 5, 30, 0, time.UTC)
	currentTime := formatTime(now)
	assert.Equal(t, "09:05", currentTime, "formatTime must produce zero-padded hour for schedule match")

	// Confirm no active session before
	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	assert.Nil(t, active, "no active session before activation")

	// Re-upsert with actual current time to trigger activation
	actualTime := formatTime(time.Now())
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, actualTime, nil, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err = chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected an active charge session to be created")
	assert.Equal(t, "rm1", active.VehicleID)
}

// mockScheduleRepo implements internal.ScheduleRepo for error injection.
type mockScheduleRepo struct {
	listAllErr      error
	listAllResult   []models.Schedule
	upsertErr       error
	getByPlugErr    error
	getByPlugResult *models.Schedule
}

func (m *mockScheduleRepo) Get(context.Context) (*models.Schedule, error)  { return nil, nil }
func (m *mockScheduleRepo) Upsert(context.Context, *models.Schedule) error { return nil }
func (m *mockScheduleRepo) GetByPlugID(context.Context, string) (*models.Schedule, error) {
	return m.getByPlugResult, m.getByPlugErr
}
func (m *mockScheduleRepo) UpsertByPlugID(context.Context, *models.Schedule) error {
	return m.upsertErr
}
func (m *mockScheduleRepo) ListAll(context.Context) ([]models.Schedule, error) {
	return m.listAllResult, m.listAllErr
}

// mockPlugRepo implements internal.PlugRepo for error injection.
type mockPlugRepo struct {
	findByIDErr    error
	findByIDResult *models.Plug
}

func (m *mockPlugRepo) Create(context.Context, *models.Plug) error { return nil }
func (m *mockPlugRepo) FindByID(context.Context, string) (*models.Plug, error) {
	return m.findByIDResult, m.findByIDErr
}
func (m *mockPlugRepo) FindByNamespaceAndSlug(context.Context, string, string) (*models.Plug, error) {
	return nil, nil
}
func (m *mockPlugRepo) ListNamespacesByUserID(context.Context, string) ([]string, error) {
	return nil, nil
}
func (m *mockPlugRepo) List(context.Context, string) ([]models.Plug, error)       { return nil, nil }
func (m *mockPlugRepo) Update(context.Context, *models.Plug) error                { return nil }
func (m *mockPlugRepo) Delete(context.Context, string, string) error              { return nil }
func (m *mockPlugRepo) SetOnline(context.Context, string, bool) error             { return nil }
func (m *mockPlugRepo) UpdateLastOfflineNotifiedAt(context.Context, string) error { return nil }
func (m *mockPlugRepo) SetInitialized(context.Context, string) error              { return nil }
func (m *mockPlugRepo) SetPowerState(context.Context, string, bool) error         { return nil }

// mockVehicleRepo implements internal.VehicleRepo for error injection.
type mockVehicleRepo struct {
	findByIDErr    error
	findByIDResult *models.Vehicle
}

func (m *mockVehicleRepo) FindByID(context.Context, string) (*models.Vehicle, error) {
	return m.findByIDResult, m.findByIDErr
}
func (m *mockVehicleRepo) FindByIDs(context.Context, []string) (map[string]*models.Vehicle, error) {
	return nil, nil
}
func (m *mockVehicleRepo) List(context.Context) ([]models.Vehicle, error)                 { return nil, nil }
func (m *mockVehicleRepo) UpdatePercents(context.Context, string, float64, float64) error { return nil }
func (m *mockVehicleRepo) UpdateName(context.Context, string, string, string) error       { return nil }
func (m *mockVehicleRepo) CreateInstance(context.Context, *models.Vehicle) error          { return nil }
func (m *mockVehicleRepo) DeleteInstance(context.Context, string, string) error           { return nil }
func (m *mockVehicleRepo) IncrementLifetimeStats(context.Context, string, float64, float64, float64, float64, time.Time) error {
	return nil
}
func (m *mockVehicleRepo) DecrementLifetimeStats(context.Context, string, float64, float64, float64, float64) error {
	return nil
}
func (m *mockVehicleRepo) UpdateNotificationPrefs(context.Context, string, string, bool, bool, bool, bool) error {
	return nil
}

// mockChargeServiceAdapter implements ChargeServiceAdapter for error injection.
type mockChargeServiceAdapter struct {
	getActiveByPlugErr    error
	getActiveByPlugResult *models.ChargeSession
	createPendingErr      error
	createPendingResult   *models.ChargeSession
	createPendingCalled   bool
	createPendingCalls    int

	twoStageErr            error
	twoStageResult         *models.ChargeSession
	twoStageCalled         bool
	twoStageHoldArg        float64
	twoStageReadyByArg     string
	twoStageCarbonAwareArg bool
}

func (m *mockChargeServiceAdapter) GetActiveByPlug(context.Context, string) (*models.ChargeSession, error) {
	return m.getActiveByPlugResult, m.getActiveByPlugErr
}

func (m *mockChargeServiceAdapter) StartSession(context.Context, string, string, float64, float64) (*models.ChargeSession, error) {
	m.createPendingCalled = true
	m.createPendingCalls++
	return m.createPendingResult, m.createPendingErr
}

func (m *mockChargeServiceAdapter) StartTwoStageSession(_ context.Context, _, _ string, _, _, holdPercent float64, readyByTime string, carbonAwareHold bool) (*models.ChargeSession, error) {
	m.twoStageCalled = true
	m.twoStageHoldArg = holdPercent
	m.twoStageReadyByArg = readyByTime
	m.twoStageCarbonAwareArg = carbonAwareHold
	return m.twoStageResult, m.twoStageErr
}

func newMockScheduleService() (*ScheduleService, *mockScheduleRepo, *mockPlugRepo, *mockVehicleRepo, *mockChargeServiceAdapter) {
	scheduleRepo := &mockScheduleRepo{}
	plugRepo := &mockPlugRepo{}
	vehicleRepo := &mockVehicleRepo{}
	chargeAdapter := &mockChargeServiceAdapter{}
	svc := NewScheduleServiceWithAdapter(scheduleRepo, plugRepo, vehicleRepo, chargeAdapter)
	return svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter
}

func TestScheduleService_UpsertByPlugID_RepoUpsertError(t *testing.T) {
	svc, scheduleRepo, _, _, _ := newMockScheduleService()
	scheduleRepo.upsertErr = assert.AnError

	_, err := svc.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", nil, true)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestScheduleService_UpsertByPlugID_RepoGetByPlugIDError(t *testing.T) {
	svc, scheduleRepo, _, _, _ := newMockScheduleService()
	scheduleRepo.getByPlugErr = assert.AnError

	_, err := svc.UpsertByPlugID(t.Context(), testPlugID, testUserID, "03:00", nil, true)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestScheduleService_CheckAndActivateAll_ListAllError(t *testing.T) {
	svc, scheduleRepo, _, _, _ := newMockScheduleService()
	scheduleRepo.listAllErr = assert.AnError

	// Should not panic, just log and return
	svc.CheckAndActivateAll(t.Context())
}

func TestScheduleService_CheckAndActivateAll_GetActiveByPlugError(t *testing.T) {
	svc, scheduleRepo, _, _, chargeAdapter := newMockScheduleService()
	plugID := testPlugID
	currentTime := formatTime(time.Now())
	scheduleRepo.listAllResult = []models.Schedule{
		{PlugID: &plugID, Time: currentTime, Enabled: true},
	}
	chargeAdapter.getActiveByPlugErr = assert.AnError

	// Should not panic, logs error and continues
	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled)
}

func TestScheduleService_CheckAndActivateAll_PlugRepoError(t *testing.T) {
	svc, scheduleRepo, plugRepo, _, chargeAdapter := newMockScheduleService()
	plugID := testPlugID
	currentTime := formatTime(time.Now())
	scheduleRepo.listAllResult = []models.Schedule{
		{PlugID: &plugID, Time: currentTime, Enabled: true},
	}
	plugRepo.findByIDErr = assert.AnError

	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))
	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled)
}

func TestScheduleService_CheckAndActivateAll_VehicleRepoError(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	plugID := testPlugID
	vehicleID := "test-vehicle"
	currentTime := formatTime(time.Now())
	scheduleRepo.listAllResult = []models.Schedule{
		{PlugID: &plugID, Time: currentTime, Enabled: true},
	}
	plugRepo.findByIDResult = &models.Plug{ID: plugID, VehicleID: &vehicleID}
	vehicleRepo.findByIDErr = assert.AnError

	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))
	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled)
}

func TestScheduleService_CheckAndActivateAll_StartSessionError(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	plugID := testPlugID
	vehicleID := "test-vehicle"
	currentTime := formatTime(time.Now())
	scheduleRepo.listAllResult = []models.Schedule{
		{PlugID: &plugID, Time: currentTime, Enabled: true},
	}
	plugRepo.findByIDResult = &models.Plug{ID: plugID, VehicleID: &vehicleID}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: vehicleID, CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingErr = assert.AnError

	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))
	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled)
}

func TestScheduleService_CheckAndActivateAll_MultipleSchedulesOneMatch(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	plugID1 := "plug-1"
	plugID2 := "plug-2"
	vehicleID := "test-vehicle"
	currentTime := formatTime(time.Now())
	scheduleRepo.listAllResult = []models.Schedule{
		{PlugID: &plugID1, Time: "23:59", Enabled: true},
		{PlugID: &plugID2, Time: currentTime, Enabled: true},
	}
	plugRepo.findByIDResult = &models.Plug{ID: plugID2, VehicleID: &vehicleID}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: vehicleID, CurrentPercent: 20, TargetPercent: 80}

	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))
	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "expected StartSession to be called for the matching schedule")
}

// --- mockForecaster ---

type mockForecaster struct {
	buckets []carbonintensity.ForecastBucket
	err     error
}

func (m *mockForecaster) GetForecast(_ context.Context, _, _ time.Time) ([]carbonintensity.ForecastBucket, error) {
	return m.buckets, m.err
}

// --- UpsertCarbonAware tests ---

func TestScheduleService_UpsertCarbonAware_Success(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	sch, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "22:00", "06:00", false, true)
	require.NoError(t, err)
	require.NotNil(t, sch)
	assert.Equal(t, models.ScheduleTypeCarbonAware, sch.Type)
	assert.Equal(t, "22:00", *sch.WindowStart)
	assert.Equal(t, "06:00", *sch.WindowEnd)
	assert.True(t, sch.Enabled)
}

func TestScheduleService_UpsertCarbonAware_TwoStage(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	sch, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "22:00", "06:00", true, true)
	require.NoError(t, err)
	require.NotNil(t, sch)
	assert.True(t, sch.TwoStage)

	// Upsert again with two-stage off - must clear the flag.
	sch, err = svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "22:00", "06:00", false, true)
	require.NoError(t, err)
	assert.False(t, sch.TwoStage)
}

func TestScheduleService_UpsertCarbonAware_EmptyUserID(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, "", "09:00", "13:00", false, true)
	assert.ErrorIs(t, err, ErrUserIDRequired)
}

func TestScheduleService_UpsertCarbonAware_MissingWindow(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "", "13:00", false, true)
	assert.ErrorIs(t, err, ErrWindowRequired)

	_, err = svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "09:00", "", false, true)
	assert.ErrorIs(t, err, ErrWindowRequired)
}

func TestScheduleService_UpsertCarbonAware_EqualWindows(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "09:00", "09:00", false, true)
	assert.ErrorIs(t, err, ErrWindowEqual)
}

func TestScheduleService_UpsertCarbonAware_UpdateExisting(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "22:00", "06:00", false, true)
	require.NoError(t, err)

	// Update with different window
	sch, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "20:00", "05:00", false, false)
	require.NoError(t, err)
	assert.Equal(t, "20:00", *sch.WindowStart)
	assert.Equal(t, "05:00", *sch.WindowEnd)
	assert.False(t, sch.Enabled)
}

// --- resolveWindow unit tests ---

func TestResolveWindow_SameDayWindow(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "09:00", "13:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC), end)
}

func TestResolveWindow_MidnightCrossing(t *testing.T) {
	now := time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "22:00", "06:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 22, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 2, 6, 0, 0, 0, time.UTC), end)
}

// TestResolveWindow_OvernightWindow_AfterMidnight is a regression test for a bug
// where resolveWindow always anchored "today" to now's calendar date. For an
// overnight window like 22:00-06:00 checked at 02:00 (still inside the window
// that opened the previous evening), the old code computed today's 22:00 as the
// start - hours in the future - making the caller think the window hadn't
// started yet, when in reality only 4 hours remained until the real 06:00
// deadline. This silently abandoned the ready-by guarantee every night an
// overnight schedule was still open past midnight.
func TestResolveWindow_OvernightWindow_AfterMidnight(t *testing.T) {
	now := time.Date(2024, 1, 2, 2, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "22:00", "06:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 22, 0, 0, 0, time.UTC), start, "should resolve to yesterday's window instance, not tonight's")
	assert.Equal(t, time.Date(2024, 1, 2, 6, 0, 0, 0, time.UTC), end)
	assert.True(t, now.Before(end), "now must still be inside the resolved window")
}

// TestResolveWindow_OvernightWindow_MiddayWaitsForTonight confirms the
// after-midnight fix doesn't misfire once yesterday's window instance has
// already ended - at midday, there's nothing left to resume, so the caller
// should be told to wait for tonight.
func TestResolveWindow_OvernightWindow_MiddayWaitsForTonight(t *testing.T) {
	now := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "22:00", "06:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 22, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 3, 6, 0, 0, 0, time.UTC), end)
}

// TestResolveWindow_OvernightWindow_JustPastEnd confirms that just after
// yesterday's window closes, the caller is told to wait for tonight rather
// than incorrectly still being considered "inside" the closed window.
func TestResolveWindow_OvernightWindow_JustPastEnd(t *testing.T) {
	now := time.Date(2024, 1, 2, 7, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "22:00", "06:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 22, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 3, 6, 0, 0, 0, time.UTC), end)
}

// TestResolveWindow_OvernightWindow_ExactlyAtStart confirms the inclusive-start
// boundary is preserved by the after-midnight fix (now.Before(start) is false
// at the exact instant, so the yesterday-check never fires).
func TestResolveWindow_OvernightWindow_ExactlyAtStart(t *testing.T) {
	now := time.Date(2024, 1, 1, 22, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "22:00", "06:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 22, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 2, 6, 0, 0, 0, time.UTC), end)
	assert.False(t, now.Before(start), "now should be exactly at start, inside the window")
}

// TestResolveWindow_OvernightWindow_ExactlyAtEnd confirms the exclusive-end
// boundary is preserved: exactly at the window's end, it should be treated as
// NOT inside and rolled to the next instance.
func TestResolveWindow_OvernightWindow_ExactlyAtEnd(t *testing.T) {
	now := time.Date(2024, 1, 2, 6, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "22:00", "06:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 2, 22, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 3, 6, 0, 0, 0, time.UTC), end)
}

// TestResolveWindow_SameDayWindow_BeforeStart_NotAffectedByMidnightCheck guards
// against the after-midnight fix misfiring for ordinary same-day windows: now
// being before today's start is a perfectly mundane "window hasn't started
// yet today" case here, not a sign of an overnight window still open from
// yesterday.
func TestResolveWindow_SameDayWindow_BeforeStart_NotAffectedByMidnightCheck(t *testing.T) {
	now := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "09:00", "13:00")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC), end)
}

func TestResolveWindow_RollForwardWhenPast(t *testing.T) {
	// now is past the window end - both should roll forward 24h.
	now := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	start, end, err := resolveWindow(now, "09:00", "13:00")
	require.NoError(t, err)
	// After roll-forward, now should be before start (next day).
	assert.True(t, now.Before(start), "now should be before rolled-forward start")
	assert.Equal(t, time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 1, 2, 13, 0, 0, 0, time.UTC), end)
}

func TestResolveWindow_InvalidWindowStart(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	_, _, err := resolveWindow(now, "bad", "13:00")
	assert.Error(t, err)
}

func TestResolveWindow_InvalidWindowEnd(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	_, _, err := resolveWindow(now, "09:00", "nope")
	assert.Error(t, err)
}

// --- parseHHMM unit tests ---

func TestParseHHMM_Valid(t *testing.T) {
	tests := []struct {
		input string
		h, m  int
	}{
		{"00:00", 0, 0},
		{"09:05", 9, 5},
		{"23:59", 23, 59},
		{"12:30", 12, 30},
	}
	for _, tt := range tests {
		h, m, err := parseHHMM(tt.input)
		require.NoError(t, err, "input: %s", tt.input)
		assert.Equal(t, tt.h, h)
		assert.Equal(t, tt.m, m)
	}
}

func TestParseHHMM_Invalid(t *testing.T) {
	// parseHHMM only validates range, not zero-padding. "9:00" and "09:0" are valid per parseHHMM.
	for _, input := range []string{"24:00", "23:60", "nope", "", "a:bb", "1:2:3"} {
		_, _, err := parseHHMM(input)
		assert.Error(t, err, "expected error for %q", input)
	}
}

// --- scoreWindow unit tests ---

func TestScoreWindow_SingleBucket(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	buckets := []carbonintensity.ForecastBucket{{From: t0, To: t1, ForecastGCo2: 200}}
	score := scoreWindow(buckets, t0, t1)
	assert.InDelta(t, 200.0, score, 0.01)
}

func TestScoreWindow_TwoBucketsEqual(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(30 * time.Minute)
	buckets := []carbonintensity.ForecastBucket{
		{From: t0, To: t1, ForecastGCo2: 100},
		{From: t1, To: t2, ForecastGCo2: 300},
	}
	score := scoreWindow(buckets, t0, t2)
	assert.InDelta(t, 200.0, score, 0.01)
}

func TestScoreWindow_NoBucketOverlap(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(30 * time.Minute)
	t3 := t2.Add(30 * time.Minute)
	buckets := []carbonintensity.ForecastBucket{{From: t0, To: t1, ForecastGCo2: 200}}
	// Query outside bucket range.
	score := scoreWindow(buckets, t2, t3)
	assert.Equal(t, math.MaxFloat64, score)
}

// --- findOptimalStart unit tests ---

func makeBuckets(start time.Time, co2Values []int) []carbonintensity.ForecastBucket {
	buckets := make([]carbonintensity.ForecastBucket, len(co2Values))
	for i, v := range co2Values {
		from := start.Add(time.Duration(i) * 30 * time.Minute)
		to := from.Add(30 * time.Minute)
		buckets[i] = carbonintensity.ForecastBucket{From: from, To: to, ForecastGCo2: v}
	}
	return buckets
}

func TestFindOptimalStart_CurrentIsOptimal(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// 4 half-hour buckets: 10:00=100, 10:30=300, 11:00=300, 11:30=300
	// D = 1h. latestStart = 12:00 - 1h = 11:00
	// Windows: start@10:00 → score=(100*30+300*30)/60=200; start@10:30 → (300+300)/60=300; start@11:00 → (300+300)/60=300
	// Optimal = 10:00
	buckets := makeBuckets(now, []int{100, 300, 300, 300})
	latestStart := now.Add(60 * time.Minute)

	optimal := findOptimalStart(buckets, now, latestStart, 60*time.Minute)
	assert.Equal(t, alignToHalfHour(now), optimal, "optimal should be the current bucket")
}

func TestFindOptimalStart_LaterBucketIsOptimal(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// D = 1h. Buckets: 10:00=400, 10:30=400, 11:00=200, 11:30=400
	// latestStart = 11:00
	// start@10:00 → (400+400)/60=400; start@10:30 → (400+200)/60=300; start@11:00 → (200+400)/60=300
	// Tie at 10:30 and 11:00 - both score 300 - findOptimalStart prefers the latest
	// equally-good candidate (11:00), minimizing time at high SoC before departure.
	buckets := makeBuckets(now, []int{400, 400, 200, 400})
	latestStart := now.Add(60 * time.Minute) // 11:00

	optimal := findOptimalStart(buckets, now, latestStart, 60*time.Minute)
	assert.Equal(t, now.Add(60*time.Minute), optimal, "tie should resolve to the latest candidate (11:00)")
}

func TestFindOptimalStart_EmptyBuckets(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	optimal := findOptimalStart(nil, now, now.Add(time.Hour), 30*time.Minute)
	assert.True(t, optimal.IsZero())
}

// TestFindOptimalStart_AllCandidatesOutOfBucketRange guards the latest-wins tie
// break from mistaking the shared math.MaxFloat64 "no data" sentinel for a
// genuine tie: every candidate here has zero forecast overlap, so none should
// be treated as valid even though they'd otherwise compare equal.
func TestFindOptimalStart_AllCandidatesOutOfBucketRange(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// Forecast data exists, but entirely outside the [now, latestStart+d] search range.
	buckets := makeBuckets(now.Add(24*time.Hour), []int{100, 100, 100})
	optimal := findOptimalStart(buckets, now, now.Add(time.Hour), 30*time.Minute)
	assert.True(t, optimal.IsZero(), "no candidate has overlapping data, so no winner should be chosen")
}

// --- findBalancedStart unit tests ---

func TestFindBalancedStart_FlatCarbon_PicksLatest(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	latestStart := now.Add(2 * time.Hour) // 12:00

	// Carbon is identical everywhere - only dwell time should decide.
	buckets := makeBuckets(now, []int{200, 200, 200, 200, 200})
	start := findBalancedStart(buckets, now, latestStart, deadline, 30*time.Minute)
	assert.Equal(t, latestStart, start, "flat carbon should collapse to pure lateness preference")
}

func TestFindBalancedStart_BalancesCleanlinessAndLateness(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	latestStart := now.Add(2 * time.Hour) // 12:00

	// 10:00=500(dirty,early) 10:30=300 11:00=100(cleanest,mid) 11:30=300 12:00=500(dirty,latest)
	// Neither the earliest (cleanest-biased) nor the latest (lateness-biased) slot
	// wins - the middle slot balances both dimensions best. See plan file for the
	// worked normalization: combined scores are 2.0, 1.25, 0.5, 0.75, 1.0.
	buckets := makeBuckets(now, []int{500, 300, 100, 300, 500})
	start := findBalancedStart(buckets, now, latestStart, deadline, 30*time.Minute)
	assert.Equal(t, now.Add(60*time.Minute), start, "expected 11:00 - the balanced trade-off, not earliest-clean or latest")
}

func TestFindBalancedStart_PrefersDecentLateOverVeryCleanEarly(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	latestStart := now.Add(2 * time.Hour) // 12:00

	// 10:00=50(very clean,earliest) 10:30=200 11:00=200 11:30=200 12:00=190(decent,latest)
	// Combined scores: 1.0, 1.75, 1.5, 1.25, 0.9333 - the decent-but-latest slot
	// wins over the much-cleaner-but-earliest one, proving carbon isn't dominant.
	buckets := makeBuckets(now, []int{50, 200, 200, 200, 190})
	start := findBalancedStart(buckets, now, latestStart, deadline, 30*time.Minute)
	assert.Equal(t, latestStart, start, "expected 12:00 to win despite worse carbon than 10:00")
}

func TestFindBalancedStart_SingleCandidate(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deadline := now.Add(90 * time.Minute)
	buckets := makeBuckets(now, []int{300})

	// latestStart == now, so only the current bucket is a valid candidate.
	start := findBalancedStart(buckets, now, now, deadline, 30*time.Minute)
	assert.Equal(t, alignToHalfHour(now), start, "single candidate should be picked without divide-by-zero")
}

func TestFindBalancedStart_NoValidCandidates(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	deadline := now.Add(2 * time.Hour)

	start := findBalancedStart(nil, now, now.Add(time.Hour), deadline, 30*time.Minute)
	assert.True(t, start.IsZero(), "no forecast data anywhere should yield no winner")
}

// TestFindOptimalStart_NowNotHalfHourAligned_NeverReturnsBeforeNow is a
// regression test for a bug where the search's lower bound was computed via
// alignToHalfHour(now), which truncates DOWN to the nearest half-hour. When
// now itself isn't half-hour aligned (the common case - schedule checks run
// every few seconds, not on the half-hour), this let the search consider a
// candidate starting before now, which could then win the scoring and be
// returned as the "optimal start" - a time before the caller's requested
// lower bound. All existing tests happened to use exactly-aligned `now`
// values, which is why this was never caught.
func TestFindOptimalStart_NowNotHalfHourAligned_NeverReturnsBeforeNow(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 3, 0, 0, time.UTC) // not on a half-hour boundary
	// The 10:00 bucket (before now) is made artificially very clean so it
	// would win the scoring outright if the truncated candidate were ever
	// considered - this proves the fix excludes it, not just happens to lose.
	buckets := makeBuckets(now.Truncate(30*time.Minute), []int{10, 400, 400, 400})
	latestStart := now.Add(90 * time.Minute)

	optimal := findOptimalStart(buckets, now, latestStart, 30*time.Minute)
	require.False(t, optimal.IsZero())
	assert.False(t, optimal.Before(now), "optimal start must never be before the requested lower bound")
}

// TestFindBalancedStart_NowNotHalfHourAligned_NeverReturnsBeforeNow is the
// findBalancedStart counterpart to the same bug - this is the function used
// by carbon-aware two-stage, where the reported symptom was a plan showing
// Stage 2 starting before Stage 1 even finished charging (stage2's search
// lower bound is stage1End, which is essentially never half-hour aligned
// since it's stage1Start plus an arbitrary integer number of minutes). The
// narrow now-to-latestStart gap (both inside the same half-hour grid cell)
// matches the real reproduction: it left only the truncated, too-early
// candidate in range, so it won by being the sole candidate rather than by
// outscoring anything.
func TestFindBalancedStart_NowNotHalfHourAligned_NeverReturnsBeforeNow(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 3, 0, 0, time.UTC)
	deadline := now.Add(2 * time.Hour)
	latestStart := now.Add(5 * time.Minute) // 10:08 - same half-hour cell as now
	buckets := makeBuckets(now.Truncate(30*time.Minute), []int{200, 200, 200, 200})

	start := findBalancedStart(buckets, now, latestStart, deadline, 30*time.Minute)
	if !start.IsZero() {
		assert.False(t, start.Before(now), "balanced start must never be before the requested lower bound - a zero (no candidate) result is acceptable here, unlike a too-early one")
	}
}

// --- Carbon-aware CheckAndActivateAll scenarios ---

func carbonAwareSchedule(windowStart, windowEnd string) models.Schedule {
	plugID := testPlugID
	return models.Schedule{
		PlugID:      &plugID,
		Type:        models.ScheduleTypeCarbonAware,
		Time:        windowStart,
		WindowStart: &windowStart,
		WindowEnd:   &windowEnd,
		Enabled:     true,
	}
}

// mockNow is a fixed time used across carbon-aware mock tests.
var mockNow = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

func TestScheduleService_CarbonAware_BeforeWindow(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("11:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// now is 10:00 - before window start 11:00
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled)
}

func TestScheduleService_CarbonAware_AfterWindow_RolledForward(t *testing.T) {
	nowPast := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "11:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// now is 12:00 - after window end 11:00, so resolveWindow rolls forward to next day
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return nowPast }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled)
}

// TestScheduleService_CarbonAware_AfterMidnight_StillConsidersWindowOpen is the
// integration-level regression test for the resolveWindow midnight bug: an
// overnight window (22:00-06:00) evaluated at 02:00 - still inside the
// instance that opened the previous evening - must be treated as open and
// reach the deadline-guard logic, not silently wait for tonight's window.
func TestScheduleService_CarbonAware_AfterMidnight_StillConsidersWindowOpen(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("22:00", "06:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	// D = 300min (5h); windowEnd resolves to today 06:00, so latestStart = 01:00.
	// now = 02:00, already past latestStart - deadline guard should fire.
	// Before the fix, resolveWindow would have computed windowEnd as tomorrow
	// 06:00 (a full day away) and the scheduler would have bailed out early
	// thinking the window hadn't started yet, leaving createPendingCalled false.
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 300, nil
	}, nil)

	nowAfterMidnight := time.Date(2024, 1, 2, 2, 0, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return nowAfterMidnight }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "overnight window still open after midnight should reach the deadline guard and start")
}

// TestScheduleService_CarbonAware_DeadlineGuard_FailureBackoff guards against
// the session-start retry storm: forced (deadline-guard) starts bypass the
// activation throttle, so when StartSession keeps failing (plug offline, power
// confirmation timeout - e.g. the bike isn't plugged in yet), the scheduler
// would otherwise create and cancel a session on EVERY 5-second poll tick for
// as long as the plug stays unreachable. A failed start must back off and
// retry at most once per scheduleThrottleDuration, then resume normally.
func TestScheduleService_CarbonAware_DeadlineGuard_FailureBackoff(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingErr = errors.New("power confirmation failed")

	// D = 60min → latestStart = 12:00. now = 12:30, past latestStart: the
	// deadline guard forces a start on every check.
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)

	now := time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	// First tick: attempts and fails.
	svc.CheckAndActivateAll(t.Context())
	assert.Equal(t, 1, chargeAdapter.createPendingCalls, "first tick should attempt the forced start")

	// Ticks 5s and 10s later: still failing - must NOT retry yet.
	for _, offset := range []time.Duration{5 * time.Second, 10 * time.Second} {
		now = time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC).Add(offset)
		svc.CheckAndActivateAll(t.Context())
	}
	assert.Equal(t, 1, chargeAdapter.createPendingCalls, "failed forced start must back off, not retry every poll tick")

	// After the backoff window: retries once more.
	now = time.Date(2024, 1, 1, 12, 30, 0, 0, time.UTC).Add(scheduleThrottleDuration + time.Second)
	svc.CheckAndActivateAll(t.Context())
	assert.Equal(t, 2, chargeAdapter.createPendingCalls, "forced start should retry after the backoff window")

	// The plug comes back: the next allowed attempt succeeds and clears the backoff.
	chargeAdapter.createPendingErr = nil
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}
	now = now.Add(scheduleThrottleDuration + time.Second)
	svc.CheckAndActivateAll(t.Context())
	assert.Equal(t, 3, chargeAdapter.createPendingCalls, "start should succeed once the plug is reachable again")
}

func TestScheduleService_CarbonAware_EstimatorError_FailsafeStart(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	// Estimator returns error → failsafe start (throttle-exempt)
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 0, errors.New("no estimate")
	}, nil)

	// Throttle is active (10s ago relative to mockNow) - but failsafe bypasses it
	svc.SetLastActivation(mockNow.Add(-10 * time.Second))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "failsafe should start session even with active throttle")
}

func TestScheduleService_CarbonAware_DeadlineGuard_BypassesThrottle(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "10:30")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	// D = 60min, windowEnd = 10:30, latestStart = 09:30
	// now = 10:00 (past latestStart) → deadline guard fires even with active throttle
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-10 * time.Second))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "deadline guard should start despite active throttle")
}

func TestScheduleService_CarbonAware_ThrottleBlocks_NonForced(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// D = 30min, windowEnd = 13:00, latestStart = 12:30
	// now = 10:00 - well before latestStart, throttle fires
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-10 * time.Second))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled, "throttle should block non-forced carbon-aware start")
}

func TestScheduleService_CarbonAware_NoForecaster_Defers(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	// No forecaster set → forecaster is nil → should defer
	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled, "no forecaster should defer start")
}

func TestScheduleService_CarbonAware_ForecastError_Defers(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	svc.SetCarbonAwareDeps(&mockForecaster{err: errors.New("API down")}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled, "forecast error should defer start")
}

func TestScheduleService_CarbonAware_OptimalIsNow_Starts(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "12:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	// D=60min, latestStart=11:00
	// Buckets: 10:00=100 (cleanest), 10:30=300, 11:00=300, 11:30=300
	// Optimal start is 10:00 = current bucket → start now
	buckets := makeBuckets(now, []int{100, 300, 300, 300})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "optimal-is-now should trigger immediate start")
}

func TestScheduleService_CarbonAware_BetterWindowLater_Waits(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// D=60min, latestStart=12:00
	// Buckets: 10:00=400, 10:30=400, 11:00=200(clean), 11:30=400, 12:00=400, 12:30=400
	// Window [11:00,12:00] scores better than [10:00,11:00] → wait
	buckets := makeBuckets(now, []int{400, 400, 200, 400, 400, 400})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled, "better-window-later should not start now")
}

func TestScheduleService_CarbonAware_MidnightCrossing_InWindow(t *testing.T) {
	now := time.Date(2024, 1, 1, 23, 30, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
	// Window 22:00 → 06:00 (crosses midnight)
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("22:00", "06:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	// D=60min, latestStart = next-day 05:00 → well after now
	// Make current bucket the optimal (cleanest)
	buckets := makeBuckets(now.Truncate(30*time.Minute), []int{100, 300, 300})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "midnight-crossing window: should start when in window and optimal is now")
}

func TestScheduleService_CarbonAware_MissingWindowFields_Skips(t *testing.T) {
	svc, scheduleRepo, _, _, chargeAdapter := newMockScheduleService()
	plugID := testPlugID
	// Schedule with carbon_aware type but nil WindowStart/WindowEnd
	scheduleRepo.listAllResult = []models.Schedule{
		{
			PlugID:  &plugID,
			Type:    models.ScheduleTypeCarbonAware,
			Enabled: true,
			// WindowStart and WindowEnd are nil
		},
	}

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.createPendingCalled, "missing window fields should skip carbon-aware schedule")
}

func TestScheduleService_CarbonAware_ShortfallNotification(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	// Window 09:00–10:30. D=60min → latestStart=09:30. now=10:00 (past latestStart).
	// availableMin = 10:30-10:00 = 30 < dMin=60 → shortfall notification expected.
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule("09:00", "10:30")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{
		ID: "v1", CurrentPercent: 20, TargetPercent: 80,
		RangeMinMi: 30, RangeMaxMi: 50,
	}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1", VehicleID: "v1"}

	title, body := "", ""
	push := &mockNotifierPushService{title: &title, body: &body}
	notifier := newTestNotifier(push, vehicleRepo)

	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil // latestStart = 10:30 - 60min = 09:30; now=10:00 > 09:30 → deadline guard
	}, notifier)

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	notifier.Wait()

	require.True(t, chargeAdapter.createPendingCalled, "deadline guard should start session")
	gotTitle, gotBody := push.GetTitleBody()
	assert.Equal(t, "Charging Shortfall", gotTitle, "expected shortfall notification title")
	assert.Contains(t, gotBody, "won't reach", "expected shortfall notification body to describe projected shortfall")
}

// --- Carbon-aware two-stage CheckAndActivateAll scenarios ---

func carbonAwareTwoStageSchedule(windowStart, windowEnd string) models.Schedule {
	plugID := testPlugID
	return models.Schedule{
		PlugID:      &plugID,
		Type:        models.ScheduleTypeCarbonAware,
		Time:        windowStart,
		WindowStart: &windowStart,
		WindowEnd:   &windowEnd,
		TwoStage:    true,
		Enabled:     true,
	}
}

func dailyTwoStageSchedule(dailyTime, readyBy string) models.Schedule {
	plugID := testPlugID
	return models.Schedule{
		PlugID:  &plugID,
		Type:    models.ScheduleTypeDaily,
		Time:    dailyTime,
		ReadyBy: &readyBy,
		Enabled: true,
	}
}

func TestScheduleService_CarbonAwareTwoStage_SkipsHoldWhenAlreadyPastHoldPercent(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	// current=70, target=80 -> hold=64, already below current: nothing to hold for.
	// Window 09:00-10:30, D=60min -> latestStart=09:30; now=10:00 (past) -> deadline guard forces
	// an immediate single-stage start.
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "10:30")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 70, TargetPercent: 80}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "expected single-stage start since vehicle is already past the hold point")
	assert.False(t, chargeAdapter.twoStageCalled, "should not start a two-stage session")
}

// TestScheduleService_CarbonAwareTwoStage_SkipsWhenStage2TooShort is a
// regression test for two-stage triggering on degenerate splits: with
// current=1, target=2, holdPercent=1.6 - stage 2 (1.6->2.0) is only a couple
// of minutes, far too short to justify a relay power-cycle and hold/resume
// transition. Uses the real chargeestimate.EstimateMinutes (no mock) against
// the seeded RM1 spec so the numbers are grounded, not assumed.
func TestScheduleService_CarbonAwareTwoStage_SkipsWhenStage2TooShort(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("22:00", "06:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{
		ID: "v1", CurrentPercent: 1, TargetPercent: 2,
		CapacityKwh: 2.026, ChargerOutputW: 600, ChargingEfficiency: 0.8, // matches seeded RM1 spec
	}
	chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

	svc.SetCarbonAwareDeps(nil, chargeestimate.EstimateMinutes, nil)

	// Just before windowEnd (06:00) so the single-stage deadline guard forces
	// an immediate start rather than deferring for lack of a forecaster.
	nowNearWindowEnd := time.Date(2024, 1, 2, 5, 59, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return nowNearWindowEnd }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.createPendingCalled, "expected a single-stage start")
	assert.False(t, chargeAdapter.twoStageCalled, "stage 2 duration is too short to be worthwhile")
}

// TestScheduleService_CarbonAwareTwoStage_MinDurationBoundary is the
// carbon-aware counterpart to the daily boundary test, pinning the same
// >= not > semantics for the worthwhileTwoStage guard.
func TestScheduleService_CarbonAwareTwoStage_MinDurationBoundary(t *testing.T) {
	tests := []struct {
		name           string
		d2             int
		expectTwoStage bool
	}{
		{"below threshold", models.MinTwoStageStageDurationMin - 1, false},
		{"at threshold", models.MinTwoStageStageDurationMin, true},
		{"above threshold", models.MinTwoStageStageDurationMin + 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
			// Window 09:00-10:30, now=10:20 - close enough to windowEnd that the
			// deadline guard forces an immediate decision for both the two-stage
			// path (d1+d2 up to 32min) and the single-stage fallback path (d=14min),
			// regardless of which branch is taken, so this test isolates the
			// worthwhileTwoStage decision itself rather than deadline-guard timing.
			scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "10:30")}
			plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
			vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
			chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}
			chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

			svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
				return tt.d2, nil
			}, nil)

			now := time.Date(2024, 1, 1, 10, 20, 0, 0, time.UTC)
			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return now }
			t.Cleanup(func() { scheduleNowFunc = old })

			svc.CheckAndActivateAll(t.Context())
			assert.Equal(t, tt.expectTwoStage, chargeAdapter.twoStageCalled)
			assert.Equal(t, !tt.expectTwoStage, chargeAdapter.createPendingCalled)
		})
	}
}

// TestScheduleService_CarbonAwareTwoStage_SmallStage1DoesNotBlockTwoStage
// proves stage 1 is intentionally never gated: a tiny stage 1 (current
// already close to the hold point) alongside a substantial stage 2 must
// still proceed as two-stage.
func TestScheduleService_CarbonAwareTwoStage_SmallStage1DoesNotBlockTwoStage(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	// current=63, target=80 -> hold=64. Stage 1 (63->64) is tiny; stage 2
	// (64->80) is substantial. Both stages get the same mock duration (60min)
	// to isolate stage 1 size from the worthwhileness decision, which only
	// looks at stage 2.
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "10:30")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 63, TargetPercent: 80}
	chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00, past latestStart -> deadline guard
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.True(t, chargeAdapter.twoStageCalled, "small stage 1 must not block two-stage when stage 2 is substantial")
	assert.Equal(t, 64.0, chargeAdapter.twoStageHoldArg)
}

// TestScheduleService_CheckAndActivateAll_TwoStage_79To80_AlreadyPastHold
// confirms the user's 79%->80% example resolves via the pre-existing
// "already past hold" branch, not the new duration guard: hold=64% is below
// current=79%, so there's nothing to hold for regardless of stage 2's size.
func TestScheduleService_CheckAndActivateAll_TwoStage_79To80_AlreadyPastHold(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := db.Exec(`UPDATE vehicles SET current_percent = 79.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	readyBy := "23:59"
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, &readyBy, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected a single-stage session to be created")
	assert.Nil(t, active.HoldPercent, "already past the 64%% hold point, nothing to hold for")
}

// TestScheduleService_CheckAndActivateAll_TwoStage_50To80_StaysTwoStage
// confirms the user's 50%->80% example stays two-stage: hold=64%, stage 2
// (64->80, ~28min on the seeded RM1 spec) comfortably clears the threshold.
func TestScheduleService_CheckAndActivateAll_TwoStage_50To80_StaysTwoStage(t *testing.T) {
	service, db, chargeService := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := db.Exec(`UPDATE vehicles SET current_percent = 50.0, target_percent = 80.0 WHERE id = ?`, "rm1")
	require.NoError(t, err)

	currentTime := formatTime(time.Now())
	readyBy := "23:59"
	_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, currentTime, &readyBy, true)
	require.NoError(t, err)

	service.CheckAndActivateAll(t.Context())

	active, err := chargeService.sessionReader.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active, "expected a two-stage session to be created")
	require.NotNil(t, active.HoldPercent)
	assert.Equal(t, 64.0, *active.HoldPercent)
}

func TestScheduleService_CarbonAwareTwoStage_DeadlineGuard_StartsNow(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	// current=20, target=80 -> hold=64. Window 09:00-10:30, d1=d2=60min ->
	// stage1LatestStart = 10:30 - 120min = 08:30; now=10:00 is past it -> deadline guard.
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "10:30")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	// Recent activation should be bypassed - deadline guard is throttle-exempt.
	svc.SetLastActivation(mockNow.Add(-2 * time.Second))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	require.True(t, chargeAdapter.twoStageCalled, "deadline guard should force a two-stage start")
	assert.Equal(t, 64.0, chargeAdapter.twoStageHoldArg)
	assert.Equal(t, "10:30", chargeAdapter.twoStageReadyByArg)
	assert.True(t, chargeAdapter.twoStageCarbonAwareArg, "session should be marked carbon-aware origin")
}

func TestScheduleService_CarbonAwareTwoStage_EstimatorError_FailsafeStart(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 0, errors.New("no estimate")
	}, nil)
	// Recent activation should be bypassed - estimator-error failsafe is throttle-exempt.
	svc.SetLastActivation(mockNow.Add(-2 * time.Second))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	require.True(t, chargeAdapter.twoStageCalled, "estimator error should trigger the two-stage failsafe start")
	assert.Equal(t, 64.0, chargeAdapter.twoStageHoldArg)
}

func TestScheduleService_CarbonAwareTwoStage_NoForecaster_Defers(t *testing.T) {
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// Estimator works fine, but no forecaster is configured.
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.twoStageCalled, "no forecaster should defer rather than start")
}

func TestScheduleService_CarbonAwareTwoStage_ThrottleBlocks_NonForced(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	svc.SetCarbonAwareDeps(&mockForecaster{buckets: makeBuckets(now, []int{100, 100, 100})}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	// Very recent activation - inside the throttle window, and not deadline-forced.
	svc.SetLastActivation(now.Add(-2 * time.Second))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.twoStageCalled, "throttle should block a non-forced two-stage start")
}

func TestScheduleService_CarbonAwareTwoStage_BetterWindowLater_Waits(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	// current=20, target=80 -> hold=64. Window 09:00-13:00, d1=d2=30min ->
	// stage1LatestStart = 13:00 - 60min = 12:00. Candidates every 30min from 10:00 to 12:00.
	// Carbon [500,300,100,300,500] at [10:00,10:30,11:00,11:30,12:00] balances to a clear
	// winner at 11:00 (see TestFindBalancedStart_BalancesCleanlinessAndLateness for the
	// worked normalization - a uniform deadline shift doesn't change relative ranking).
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "13:00")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	buckets := makeBuckets(now, []int{500, 300, 100, 300, 500})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	assert.False(t, chargeAdapter.twoStageCalled, "a better-balanced window later should defer starting")
}

func TestScheduleService_CarbonAwareTwoStage_SingleCandidateWindow_StartsNow(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()

	// current=20, target=80 -> hold=64. Window 09:00-11:15, d1=d2=30min ->
	// stage1LatestStart = 11:15 - 60min = 10:15, strictly after now (so the deadline
	// guard does NOT fire) but less than one bucket away, so the search range collapses
	// to a single candidate (the current bucket) and the forecast path must start now.
	scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule("09:00", "11:15")}
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

	buckets := makeBuckets(now, []int{300})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	svc.SetLastActivation(mockNow.Add(-2 * time.Minute))

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	svc.CheckAndActivateAll(t.Context())
	require.True(t, chargeAdapter.twoStageCalled, "single feasible candidate should start now via the forecast path")
	assert.Equal(t, 64.0, chargeAdapter.twoStageHoldArg)
}

// stringPtr is a local helper for creating string pointers in tests.
func stringPtr(s string) *string { return &s }

// --- EstimateCarbonAwareStart tests ---

func TestScheduleService_EstimateCarbonAwareStart_NilSchedule(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	_, ok := svc.EstimateCarbonAwareStart(t.Context(), nil)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_NotCarbonAware(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := &models.Schedule{Type: models.ScheduleTypeDaily, Time: "03:00", Enabled: true}
	_, ok := svc.EstimateCarbonAwareStart(t.Context(), sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_Disabled(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := carbonAwareSchedule("09:00", "13:00")
	sch.Enabled = false
	_, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_MissingWindow(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	plugID := testPlugID
	sch := &models.Schedule{PlugID: &plugID, Type: models.ScheduleTypeCarbonAware, Enabled: true}
	_, ok := svc.EstimateCarbonAwareStart(t.Context(), sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_NoForecaster(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := carbonAwareSchedule("09:00", "13:00")
	_, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_EstimatorError(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 0, errors.New("no estimate")
	}, nil)
	sch := carbonAwareSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_AlreadyAtTarget(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 80, TargetPercent: 80}
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_PastDeadline_ReturnsLatestStart(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	// Window 09:00-10:30, D=60min → latestStart=09:30. now=10:00 (past latestStart).
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareSchedule("09:00", "10:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	start, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "09:30", start)
}

// TestScheduleService_EstimateCarbonAwareStart_AfterMidnight_StillConsidersWindowOpen
// is the estimate-mirror counterpart to the activation-side midnight
// regression test: an overnight window evaluated just after local midnight
// must still resolve to the instance that opened the previous evening.
func TestScheduleService_EstimateCarbonAwareStart_AfterMidnight_StillConsidersWindowOpen(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	// Window 22:00-06:00, D=300min(5h) -> latestStart = today 01:00.
	// now = today 02:00, already past latestStart.
	// Before the fix, resolveWindow would have computed windowEnd as tomorrow
	// 06:00 and searchFrom as tomorrow's windowStart (22:00, still in the
	// future relative to now), so the deadline guard would never fire here
	// and the empty mockForecaster would make the whole estimate fail (ok=false).
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 300, nil
	}, nil)
	sch := carbonAwareSchedule("22:00", "06:00")

	nowAfterMidnight := time.Date(2024, 1, 2, 2, 0, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return nowAfterMidnight }
	t.Cleanup(func() { scheduleNowFunc = old })

	start, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	require.True(t, ok, "overnight window still open after midnight should still produce a confident estimate")
	assert.Equal(t, "01:00", start)
}

func TestScheduleService_EstimateCarbonAwareStart_ForecastError(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(&mockForecaster{err: errors.New("API down")}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00, well before latestStart 12:00
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareStart_InsideWindow_ScansFromNow(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// Window 09:00-13:00, D=60min → latestStart=12:00.
	// Buckets from 10:00: 10:00=500, 10:30=350, 11:00=100, 11:30=450, 12:00=500, 12:30=500
	// start@10:30 → (350+100)/2=225 is the unambiguous minimum.
	buckets := makeBuckets(now, []int{500, 350, 100, 450, 500, 500})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	start, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "10:30", start, "should pick the cleanest window starting at 10:30")
}

func TestScheduleService_EstimateCarbonAwareStart_BeforeWindow_ScansFromWindowStart(t *testing.T) {
	// now is 07:00, window opens at 09:00 - search should start at windowStart, not now,
	// otherwise a clean bucket before the window opens could be picked incorrectly.
	now := time.Date(2024, 1, 1, 7, 0, 0, 0, time.UTC)
	windowStartTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}

	// Window 09:00-13:00, D=60min → latestStart=12:00.
	// Buckets from windowStart(09:00): 09:00=50 (clean), rest=400.
	buckets := makeBuckets(windowStartTime, []int{50, 400, 400, 400, 400, 400, 400, 400})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	start, ok := svc.EstimateCarbonAwareStart(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "09:00", start, "should scan from windowStart since we're before it opens")
}

// --- EstimateCarbonAwareTwoStagePlan tests ---

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_NilSchedule(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), nil)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_NotTwoStage(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareSchedule("09:00", "13:00") // TwoStage defaults to false

	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "single-stage schedules have no two-stage plan")
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_NoForecaster(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := carbonAwareTwoStageSchedule("09:00", "13:00")
	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_AlreadyPastHoldPercent(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 70, TargetPercent: 80} // hold=64 < 70
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareTwoStageSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "no plan when there's nothing to hold for")
}

// TestScheduleService_EstimateCarbonAwareTwoStagePlan_AfterMidnight_StillConsidersWindowOpen
// is the two-stage estimate-mirror counterpart to the midnight regression test.
func TestScheduleService_EstimateCarbonAwareTwoStagePlan_AfterMidnight_StillConsidersWindowOpen(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	// Window 23:00-00:30 (crosses midnight), d1=d2=60min -> stage1LatestStart =
	// today's windowEnd(00:30) - 120min = yesterday 22:30, well before now.
	// Before the fix, windowEnd would resolve to tomorrow 00:30 - a full day
	// away - so the deadline guard would never fire and the empty
	// mockForecaster would make the whole plan fail (ok=false).
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareTwoStageSchedule("23:00", "00:30")

	nowAfterMidnight := time.Date(2024, 1, 2, 0, 15, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return nowAfterMidnight }
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	require.True(t, ok, "overnight window still open after midnight should still produce a confident plan")
	assert.Equal(t, "22:30", plan.Stage1Start)
	assert.Equal(t, "23:30", plan.Stage1End)
	assert.Equal(t, "23:30", plan.Stage2Start)
	assert.Equal(t, "00:30", plan.Stage2End)
}

// TestScheduleService_EstimateCarbonAwareTwoStagePlan_SkipsWhenStage2TooShort
// is the estimate-mirror counterpart: it must not show a plan preview for a
// schedule whose activation would actually fall back to single-stage.
func TestScheduleService_EstimateCarbonAwareTwoStagePlan_SkipsWhenStage2TooShort(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{
		ID: "v1", CurrentPercent: 1, TargetPercent: 2,
		CapacityKwh: 2.026, ChargerOutputW: 600, ChargingEfficiency: 0.8,
	}
	// Real forecast data spanning the window, so a pre-fix run (with no
	// duration guard) would successfully compute a plan via the forecaster
	// instead of accidentally returning ok=false for the unrelated reason of
	// having no forecast data - this test must fail red before the fix.
	now := time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC)
	buckets := makeBuckets(now.Truncate(30*time.Minute), []int{200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, chargeestimate.EstimateMinutes, nil)
	sch := carbonAwareTwoStageSchedule("22:00", "06:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "stage 2 duration is too short to be worthwhile")
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_EstimatorError(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 0, errors.New("no estimate")
	}, nil)
	sch := carbonAwareTwoStageSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

// TestScheduleService_EstimateCarbonAwareTwoStagePlan_MinDurationBoundary
// pins the same >= not > boundary for the plan-preview mirror.
func TestScheduleService_EstimateCarbonAwareTwoStagePlan_MinDurationBoundary(t *testing.T) {
	tests := []struct {
		name   string
		d2     int
		wantOk bool
	}{
		{"below threshold", models.MinTwoStageStageDurationMin - 1, false},
		{"at threshold", models.MinTwoStageStageDurationMin, true},
		{"above threshold", models.MinTwoStageStageDurationMin + 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
			plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
			vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
			// Window 09:00-10:30, now=10:00 (mockNow) - past latestStart for
			// both stages, so the deadline-guard-fallback path is used and no
			// forecast data is needed.
			svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
				return tt.d2, nil
			}, nil)
			sch := carbonAwareTwoStageSchedule("09:00", "10:30")

			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return mockNow }
			t.Cleanup(func() { scheduleNowFunc = old })

			_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

// TestScheduleService_EstimateCarbonAwareTwoStagePlan_Stage1NeverBeforeWindowStart
// is an end-to-end regression test for the reported bug: a non-half-hour-
// aligned window start (01:15) let the search consider a candidate at 01:00
// - before the window even opened. d1+d2 is sized so stage1LatestStart
// (01:20) sits only 5 minutes after windowStart(01:15) - the same narrow-gap
// shape as the confirmed stage 2 repro, needed because findBalancedStart's
// equal-weighted scoring otherwise naturally avoids the earliest candidate
// on its own (it can tie for best but never strictly win), which would let
// this test pass even without the fix.
func TestScheduleService_EstimateCarbonAwareTwoStagePlan_Stage1NeverBeforeWindowStart(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 0, TargetPercent: 100} // hold=80
	buckets := makeBuckets(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), []int{
		200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 200,
	})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, current, _ float64) (int, error) {
		if current == 0 {
			return 260, nil // d1: current(0) -> hold(80)
		}
		return 20, nil // d2: hold(80) -> target(100), above MinTwoStageStageDurationMin
	}, nil)
	sch := carbonAwareTwoStageSchedule("01:15", "06:00")

	// now is before windowStart, so searchFrom resolves to windowStart(01:15) itself.
	now := time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	require.True(t, ok)
	// HH:MM string comparison matches chronological order here since both
	// times fall on the same day (no midnight wrap in this scenario).
	assert.GreaterOrEqual(t, plan.Stage1Start, "01:15", "stage 1 must never start before the window's earliest bound")
}

// TestScheduleService_EstimateCarbonAwareTwoStagePlan_Stage2NeverBeforeStage1End
// is an end-to-end regression test for the reported bug: with a tiny stage 1
// and a stage 2 deadline guard only a few minutes after stage 1 finishes,
// the search picked a half-hour-aligned candidate before stage 1 even ended.
func TestScheduleService_EstimateCarbonAwareTwoStagePlan_Stage2NeverBeforeStage1End(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{
		ID: "v1", CurrentPercent: 79, TargetPercent: 100,
		CapacityKwh: 2.026, ChargerOutputW: 600, ChargingEfficiency: 0.8,
	}
	buckets := makeBuckets(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), func() []int {
		v := make([]int, 96)
		for i := range v {
			v[i] = 200
		}
		return v
	}())
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, chargeestimate.EstimateMinutes, nil)
	sch := carbonAwareTwoStageSchedule("01:00", "06:00")

	now := time.Date(2024, 1, 1, 3, 47, 0, 0, time.UTC) // not half-hour aligned
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	require.True(t, ok)
	stage1End, err := time.Parse("15:04", plan.Stage1End)
	require.NoError(t, err)
	stage2Start, err := time.Parse("15:04", plan.Stage2Start)
	require.NoError(t, err)
	assert.False(t, stage2Start.Before(stage1End), "stage 2 must never start before stage 1 finishes charging (stage1End=%s, stage2Start=%s)", plan.Stage1End, plan.Stage2Start)
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_PastDeadline_UsesLatestStarts(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	// Window 09:00-10:30, d1=d2=60min -> stage1LatestStart=10:30-120min=08:30.
	// now=10:00 is past it for both stages, so both fall back to their deadline-guard times.
	svc.SetCarbonAwareDeps(&mockForecaster{}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := carbonAwareTwoStageSchedule("09:00", "10:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "08:30", plan.Stage1Start)
	assert.Equal(t, "09:30", plan.Stage1End)
	assert.Equal(t, "09:30", plan.Stage2Start)
	assert.Equal(t, "10:30", plan.Stage2End)
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_ForecastComputesBothStages(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	// Window 09:00-13:00, d1=d2=30min -> stage1LatestStart=13:00-60min=12:00.
	// Same buckets as TestFindBalancedStart_BalancesCleanlinessAndLateness: stage 1
	// balances to 11:00 (ends 11:30). Stage 2 then searches [11:30,12:30] using the
	// same fixed bucket set; the only two candidates with forecast coverage
	// (11:30@300, 12:00@500) tie at a combined score of 1.0 and resolve to the
	// later one (12:00) via the latest-wins tie break.
	buckets := makeBuckets(now, []int{500, 300, 100, 300, 500})
	svc.SetCarbonAwareDeps(&mockForecaster{buckets: buckets}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	sch := carbonAwareTwoStageSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return now }
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "11:00", plan.Stage1Start)
	assert.Equal(t, "11:30", plan.Stage1End)
	assert.Equal(t, "12:00", plan.Stage2Start)
	assert.Equal(t, "12:30", plan.Stage2End)
}

func TestScheduleService_EstimateCarbonAwareTwoStagePlan_ForecastError(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(&mockForecaster{err: errors.New("API down")}, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 30, nil
	}, nil)
	sch := carbonAwareTwoStageSchedule("09:00", "13:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateCarbonAwareTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

// --- EstimateDailyTwoStagePlan tests ---

func TestScheduleService_EstimateDailyTwoStagePlan_NilSchedule(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), nil)
	assert.False(t, ok)
}

func TestScheduleService_EstimateDailyTwoStagePlan_CarbonAwareType(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := carbonAwareTwoStageSchedule("22:00", "06:00")
	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "carbon-aware schedules use EstimateCarbonAwareTwoStagePlan instead")
}

func TestScheduleService_EstimateDailyTwoStagePlan_Disabled(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := dailyTwoStageSchedule("22:00", "06:00")
	sch.Enabled = false
	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateDailyTwoStagePlan_ReadyByNil(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := models.Schedule{PlugID: stringPtr(testPlugID), Type: models.ScheduleTypeDaily, Time: "22:00", Enabled: true}
	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "single-stage daily schedules have no two-stage plan")
}

func TestScheduleService_EstimateDailyTwoStagePlan_MissingPlugID(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	readyBy := "06:00"
	sch := models.Schedule{Type: models.ScheduleTypeDaily, Time: "22:00", ReadyBy: &readyBy, Enabled: true}
	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateDailyTwoStagePlan_VehicleAlreadyAtTarget(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 80, TargetPercent: 80}
	sch := dailyTwoStageSchedule("22:00", "06:00")

	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

func TestScheduleService_EstimateDailyTwoStagePlan_AlreadyPastHoldPercent(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 70, TargetPercent: 80} // hold=64 < 70
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := dailyTwoStageSchedule("22:00", "06:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "no plan when there's nothing to hold for")
}

func TestScheduleService_EstimateDailyTwoStagePlan_SkipsWhenStage2TooShort(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{
		ID: "v1", CurrentPercent: 1, TargetPercent: 2,
		CapacityKwh: 2.026, ChargerOutputW: 600, ChargingEfficiency: 0.8,
	}
	svc.SetCarbonAwareDeps(nil, chargeestimate.EstimateMinutes, nil)
	sch := dailyTwoStageSchedule("22:00", "06:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok, "stage 2 duration is too short to be worthwhile")
}

func TestScheduleService_EstimateDailyTwoStagePlan_EstimatorError(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, current, _ float64) (int, error) {
		// worthwhileTwoStage's own d2 estimate returns true on error (proceed),
		// so d1/d2 here must fail specifically at this later estimator call to
		// exercise this branch - simulate by erroring only for the d1 call
		// (current == vehicle's CurrentPercent, i.e. the stage-1 estimate).
		if current == 20 {
			return 0, errors.New("no estimate")
		}
		return 60, nil
	}, nil)
	sch := dailyTwoStageSchedule("22:00", "06:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	_, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	assert.False(t, ok)
}

// TestScheduleService_EstimateDailyTwoStagePlan_HappyPath is a concrete worked
// example with no midnight rollover: Time=01:00 (next occurrence rolls to
// tomorrow since mockNow=10:00 is later in the day), ReadyBy=07:00 the same
// day as stage 1.
func TestScheduleService_EstimateDailyTwoStagePlan_HappyPath(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := dailyTwoStageSchedule("01:00", "07:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 2024-01-01 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "01:00", plan.Stage1Start, "next occurrence of the daily time, rolled to tomorrow since now=10:00 is later today")
	assert.Equal(t, "02:00", plan.Stage1End)
	assert.Equal(t, "06:00", plan.Stage2Start, "deadline(07:00) - d2(60min)")
	assert.Equal(t, "07:00", plan.Stage2End)
}

// TestScheduleService_EstimateDailyTwoStagePlan_ReadyByCrossesMidnight pins the
// daily-specific case where ReadyBy's clock time is earlier than the daily
// start time, so it must resolve to the following day relative to stage 1 -
// not relative to `now`.
func TestScheduleService_EstimateDailyTwoStagePlan_ReadyByCrossesMidnight(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil
	}, nil)
	sch := dailyTwoStageSchedule("23:30", "02:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow } // 2024-01-01 10:00
	t.Cleanup(func() { scheduleNowFunc = old })

	plan, ok := svc.EstimateDailyTwoStagePlan(t.Context(), &sch)
	require.True(t, ok)
	assert.Equal(t, "23:30", plan.Stage1Start)
	assert.Equal(t, "00:30", plan.Stage1End, "stage 1 crosses midnight")
	assert.Equal(t, "01:00", plan.Stage2Start, "readyBy(02:00) resolved relative to stage1Start rolls to the next day, not to today's already-passed 02:00")
	assert.Equal(t, "02:00", plan.Stage2End)
}

// --- EstimateTargetReachable tests ---

func TestScheduleService_EstimateTargetReachable_NilSchedule(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	assert.True(t, svc.EstimateTargetReachable(t.Context(), nil))
}

func TestScheduleService_EstimateTargetReachable_Disabled(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	sch := dailyTwoStageSchedule("01:00", "07:00")
	sch.Enabled = false
	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_EstimateTargetReachable_MissingPlugID(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()
	readyBy := "07:00"
	sch := models.Schedule{Type: models.ScheduleTypeDaily, Time: "01:00", ReadyBy: &readyBy, Enabled: true}
	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_EstimateTargetReachable_DailySingleStage_AlwaysTrue(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 0, TargetPercent: 100}
	sch := models.Schedule{PlugID: stringPtr(testPlugID), Type: models.ScheduleTypeDaily, Time: "01:00", Enabled: true}
	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch), "no readyBy means no deadline to violate")
}

func TestScheduleService_EstimateTargetReachable_DailyTwoStage_Feasible(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil // d1+d2=120min, plenty of room in a 6-hour window
	}, nil)
	sch := dailyTwoStageSchedule("01:00", "07:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_EstimateTargetReachable_DailyTwoStage_Infeasible(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil // d1+d2=120min, but the window below is only 30 minutes
	}, nil)
	sch := dailyTwoStageSchedule("01:00", "01:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.False(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_EstimateTargetReachable_CarbonAwareSingleStage_Infeasible(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 90, nil // d=90min, window below is only 30 minutes
	}, nil)
	sch := carbonAwareSchedule("01:00", "01:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.False(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_EstimateTargetReachable_CarbonAwareSingleStage_Feasible(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 90, nil
	}, nil)
	sch := carbonAwareSchedule("01:00", "06:00")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_EstimateTargetReachable_CarbonAwareTwoStage_Infeasible(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80} // hold=64
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 60, nil // d1+d2=120min, window below is only 30 minutes
	}, nil)
	sch := carbonAwareTwoStageSchedule("01:00", "01:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.False(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

// TestScheduleService_EstimateTargetReachable_CarbonAwareTwoStage_NotWorthwhile_UsesSingleStageDuration
// proves the check mirrors real activation behavior rather than assuming
// two-stage always applies: when worthwhileTwoStage says the schedule will
// actually fall back to single-stage, reachability must be judged against
// the single-stage duration d, not d1+d2.
func TestScheduleService_EstimateTargetReachable_CarbonAwareTwoStage_NotWorthwhile_UsesSingleStageDuration(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	// current=70, target=80 -> hold=64, already below current: worthwhileTwoStage
	// is false ("already past hold point"), so this behaves as single-stage.
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 70, TargetPercent: 80}
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 20, nil // single-stage d=20min fits comfortably in a 30-minute window
	}, nil)
	sch := carbonAwareTwoStageSchedule("01:00", "01:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch),
		"should be judged against the single-stage duration, not d1+d2, since worthwhileTwoStage falls back to single-stage")
}

func TestScheduleService_EstimateTargetReachable_EstimatorError(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: 20, TargetPercent: 80}
	svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
		return 0, errors.New("no estimate")
	}, nil)
	sch := carbonAwareSchedule("01:00", "01:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch), "estimator error should fail open, not warn")
}

func TestScheduleService_EstimateTargetReachable_VehicleNotFound(t *testing.T) {
	svc, _, plugRepo, vehicleRepo, _ := newMockScheduleService()
	plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
	vehicleRepo.findByIDErr = errors.New("db error")
	sch := carbonAwareSchedule("01:00", "01:30")

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	assert.True(t, svc.EstimateTargetReachable(t.Context(), &sch))
}

func TestScheduleService_formatTime_MatchesUpsertFormat(t *testing.T) {
	// Regression test: formatTime must produce zero-padded hours ("09:05")
	// so it matches the time stored by UpsertByPlugID ("09:05"). Previously
	// formatTime produced "9:05" (no leading zero) causing the comparison to
	// always fail for hours 0-9.
	for hour := 0; hour < 24; hour++ {
		ts := time.Date(2024, 1, 1, hour, 30, 0, 0, time.UTC)
		formatted := formatTime(ts)

		assert.True(t, isValidTimeFormat(formatted),
			"formatTime for hour %02d produced %q which isValidTimeFormat rejects", hour, formatted)

		colonIdx := strings.IndexByte(formatted, ':')
		assert.Equal(t, 2, colonIdx,
			"formatTime for hour %02d produced %q, expected 2-digit hour", hour, formatted)
	}
}

// --- Exhaustive property sweeps ---
//
// The tests below are pure-function invariant checks (no DB, no service
// wiring) covering percent and time boundaries at the density requested
// during the edge-case audit: 5-percentage-point increments across 0-100%,
// and 30-minute increments across a full 24h day including midnight
// crossover. They check *invariants* that must hold for every generated
// case, rather than a hand-computed expected value per row - the only way
// to state "expected" for hundreds/thousands of generated cases.

// sweepVehicleSpec matches the seeded RM1 model (api/database/seed.sql) so
// EstimateMinutes produces grounded, non-error durations across the sweep.
var sweepVehicleSpec = &models.Vehicle{
	ID:                 "sweep-vehicle",
	CapacityKwh:        2.026,
	ChargerOutputW:     600,
	ChargingEfficiency: 0.8,
}

// TestWorthwhileTwoStage_PercentMatrix exhaustively sweeps every
// (current, target) pair at 5-percentage-point increments across 0-100%
// (current < target) and cross-checks worthwhileTwoStage's decision against
// an independently recomputed expectation, rather than hard-coding
// "yes/no" per pair. If MinTwoStageStageDurationMin or TwoStageHoldFraction
// ever change, this test pinpoints exactly which percent pairs flip
// behavior instead of silently drifting.
func TestWorthwhileTwoStage_PercentMatrix(t *testing.T) {
	svc, _, _, _, _ := newMockScheduleService()

	var lastTarget float64 = -1
	var prevWorthwhile bool
	for target := 0.0; target <= 100.0; target += 5.0 {
		if target != lastTarget {
			lastTarget = target
			prevWorthwhile = false // reset monotonicity tracking per target
		}
		for current := 0.0; current < target; current += 5.0 {
			t.Run(formatPercentPair(current, target), func(t *testing.T) {
				vehicle := *sweepVehicleSpec
				vehicle.CurrentPercent = current
				vehicle.TargetPercent = target

				d, err := chargeestimate.EstimateMinutes(&vehicle, current, target)
				require.NoError(t, err)
				assert.Greater(t, d, 0, "duration must never be zero for current < target (ceil-to->=1min invariant)")

				holdPercent := target * models.TwoStageHoldFraction
				got := svc.worthwhileTwoStage(&vehicle, holdPercent)

				if holdPercent <= current {
					assert.False(t, got, "already past hold point must never be worthwhile")
					return
				}

				d2, err := chargeestimate.EstimateMinutes(&vehicle, holdPercent, target)
				require.NoError(t, err)
				want := d2 >= models.MinTwoStageStageDurationMin
				assert.Equal(t, want, got, "worthwhileTwoStage must match an independently recomputed d2 >= threshold")

				// Monotonicity: for a fixed target, once current gets close enough
				// to no longer be worthwhile, increasing current further (getting
				// even closer to target) must never flip it back to worthwhile.
				if !prevWorthwhile && current > 0 {
					assert.False(t, got, "worthwhileTwoStage flipped false->true as current increased toward a fixed target")
				}
				prevWorthwhile = got
			})
		}
	}
}

func formatPercentPair(current, target float64) string {
	return fmt.Sprintf("%.0f->%.0fpct", current, target)
}

// TestResolveWindow_TimeMatrix exhaustively sweeps windowStart at 30-minute
// increments across a full 24h day, crossed with 3 representative window
// durations (30m short, 8h typical overnight, 23h nearly-full-day - covering
// both same-day and midnight-crossing shapes) and now at 30-minute
// increments across the same day. This is the literal "30-minute increments
// covering 24 hours + crossover" sweep requested during the edge-case audit:
// every combination that pushes a window's end past midnight, crossed with
// every possible now, appears somewhere in the grid.
//
// Rather than a hand-computed expected value per one of the 6,912 generated
// cases, this checks an independently-derived invariant: given the window
// recurs with an exact 24h period, "elapsed" (how far now is past the most
// recent same-phase instance start) determines whether now is inside that
// instance or must roll to the next one - this is an oracle reimplementation
// of the recurrence, cross-checked against resolveWindow's actual output.
func TestResolveWindow_TimeMatrix(t *testing.T) {
	durations := []time.Duration{30 * time.Minute, 8 * time.Hour, 23 * time.Hour}
	baseDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	const dayMinutes = 24 * 60

	for _, duration := range durations {
		for startMin := 0; startMin < dayMinutes; startMin += 30 {
			windowStartStr := fmt.Sprintf("%02d:%02d", startMin/60, startMin%60)
			endMin := (startMin + int(duration.Minutes())) % dayMinutes
			windowEndStr := fmt.Sprintf("%02d:%02d", endMin/60, endMin%60)
			todayBaseStart := baseDate.Add(time.Duration(startMin) * time.Minute)

			for nowMin := 0; nowMin < dayMinutes; nowMin += 30 {
				now := baseDate.Add(time.Duration(nowMin) * time.Minute)
				name := fmt.Sprintf("dur=%s/start=%s/now=%02d:%02d", duration, windowStartStr, nowMin/60, nowMin%60)

				t.Run(name, func(t *testing.T) {
					start, end, err := resolveWindow(now, windowStartStr, windowEndStr)
					require.NoError(t, err)

					assert.True(t, end.After(start), "end must be strictly after start")
					assert.Equal(t, duration, end.Sub(start), "window length must be preserved exactly")

					elapsed := now.Sub(todayBaseStart) % (24 * time.Hour)
					if elapsed < 0 {
						elapsed += 24 * time.Hour
					}
					lastInstanceStart := now.Add(-elapsed)

					var wantStart, wantEnd time.Time
					if elapsed < duration {
						wantStart, wantEnd = lastInstanceStart, lastInstanceStart.Add(duration)
					} else {
						wantStart = lastInstanceStart.Add(24 * time.Hour)
						wantEnd = wantStart.Add(duration)
					}

					assert.Equal(t, wantStart, start, "resolved start does not match the instance actually containing (or soonest after) now")
					assert.Equal(t, wantEnd, end)
				})
			}
		}
	}
}

// TestResolveDeadline_TimeMatrix exhaustively sweeps hhmm and now at
// 30-minute increments across a full 24h day (48x48 = 2,304 cases),
// asserting the "next occurrence" invariant holds everywhere: the returned
// deadline is never before now, is always within 24h of now, and lands on
// the requested clock time. This covers every possible now-vs-hhmm
// relationship including the crossover case already spot-checked in
// TestResolveDeadline_MidnightBoundary.
func TestResolveDeadline_TimeMatrix(t *testing.T) {
	baseDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	const dayMinutes = 24 * 60

	for hhmmMin := 0; hhmmMin < dayMinutes; hhmmMin += 30 {
		hhmmStr := fmt.Sprintf("%02d:%02d", hhmmMin/60, hhmmMin%60)

		for nowMin := 0; nowMin < dayMinutes; nowMin += 30 {
			now := baseDate.Add(time.Duration(nowMin) * time.Minute)
			name := fmt.Sprintf("hhmm=%s/now=%02d:%02d", hhmmStr, nowMin/60, nowMin%60)

			t.Run(name, func(t *testing.T) {
				deadline, err := resolveDeadline(now, hhmmStr)
				require.NoError(t, err)

				assert.False(t, deadline.Before(now), "deadline must never be before now")
				assert.LessOrEqual(t, deadline.Sub(now), 24*time.Hour, "deadline must be within 24h of now")
				assert.Equal(t, hhmmStr, formatTime(deadline), "deadline must land on the requested clock time")
			})
		}
	}
}

// --- Curated DB-backed integration matrices ---
//
// The exhaustive sweeps above prove the underlying math is sound everywhere.
// These smaller, hand-picked tables prove the *wiring* - that the right
// repository/service/session calls actually happen - at a handful of
// representative points spanning both schedule types and both charging
// modes. Kept deliberately small since each row pays real SQLite setup cost.

func TestScheduleService_Daily_SingleStage_Matrix(t *testing.T) {
	tests := []struct {
		name          string
		current       float64
		target        float64
		startTime     string
		expectSession bool
	}{
		{"tiny target, exact midnight start", 0, 1, "00:00", true},
		{"full range, last-minute-of-day start", 0, 100, "23:59", true},
		{"tiny top-up near full", 99, 100, "12:00", true},
		{"fractional percents", 49.5, 50.5, "06:00", true},
		{"baseline sanity", 20, 80, "03:00", true},
		{"already at target - no session", 100, 100, "03:00", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, db, chargeService := setupScheduleServiceTest(t)
			defer db.Close()

			_, err := db.Exec(`UPDATE vehicles SET current_percent = ?, target_percent = ? WHERE id = ?`, tt.current, tt.target, "rm1")
			require.NoError(t, err)

			_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, tt.startTime, nil, true)
			require.NoError(t, err)

			now := mustParseHHMMOnFixedDate(t, tt.startTime)
			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return now }
			t.Cleanup(func() { scheduleNowFunc = old })

			service.CheckAndActivateAll(t.Context())

			active, err := chargeService.sessionReader.GetActive(t.Context())
			require.NoError(t, err)
			if !tt.expectSession {
				assert.Nil(t, active, "expected no session to be created")
				return
			}
			require.NotNil(t, active, "expected a single-stage session to be created")
			assert.Nil(t, active.HoldPercent, "no readyBy set - must never be two-stage")
			assert.Equal(t, tt.target, active.TargetPercent)
		})
	}
}

// mustParseHHMMOnFixedDate constructs a fixed UTC time.Time for an "HH:MM"
// string, for tests that need a deterministic scheduleNowFunc override.
func mustParseHHMMOnFixedDate(t *testing.T, hhmm string) time.Time {
	t.Helper()
	h, m, err := parseHHMM(hhmm)
	require.NoError(t, err)
	return time.Date(2024, 1, 1, h, m, 0, 0, time.UTC)
}

func TestScheduleService_Daily_TwoStage_Matrix(t *testing.T) {
	tests := []struct {
		name           string
		current        float64
		target         float64
		startTime      string
		readyBy        string
		expectTwoStage bool
	}{
		{"stage2 too short", 1, 2, "00:00", "00:01", false},
		{"typical overnight", 20, 80, "22:00", "06:00", true},
		{"already past hold", 79, 80, "03:00", "04:00", false},
		{"user's 50->80 example", 50, 80, "01:00", "07:00", true},
		{"tiny stage 1, big stage 2", 63, 80, "02:00", "03:00", true},
		{"readyBy wraps past midnight - no activation-time deadline guard for daily", 20, 80, "23:59", "00:01", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, db, chargeService := setupScheduleServiceTest(t)
			defer db.Close()

			_, err := db.Exec(`UPDATE vehicles SET current_percent = ?, target_percent = ? WHERE id = ?`, tt.current, tt.target, "rm1")
			require.NoError(t, err)

			readyBy := tt.readyBy
			_, err = service.UpsertByPlugID(t.Context(), testPlugID, testUserID, tt.startTime, &readyBy, true)
			require.NoError(t, err)

			now := mustParseHHMMOnFixedDate(t, tt.startTime)
			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return now }
			t.Cleanup(func() { scheduleNowFunc = old })

			service.CheckAndActivateAll(t.Context())

			active, err := chargeService.sessionReader.GetActive(t.Context())
			require.NoError(t, err)
			require.NotNil(t, active, "expected a session to be created")
			if tt.expectTwoStage {
				require.NotNil(t, active.HoldPercent)
				assert.Equal(t, tt.target*models.TwoStageHoldFraction, *active.HoldPercent)
				require.NotNil(t, active.ReadyByTime)
				assert.Equal(t, tt.readyBy, *active.ReadyByTime)
			} else {
				assert.Nil(t, active.HoldPercent)
				assert.Nil(t, active.ReadyByTime)
			}
			assert.Equal(t, tt.target, active.TargetPercent)
		})
	}
}

func TestScheduleService_CarbonAware_SingleStage_Matrix(t *testing.T) {
	tests := []struct {
		name        string
		current     float64
		target      float64
		windowStart string
		windowEnd   string
		now         time.Time
		estimateMin int
	}{
		// now is placed 1 minute before windowEnd and estimateMin=1, so the
		// deadline guard fires deterministically regardless of window length,
		// without needing a forecaster.
		{"tiny target inside a long overnight window", 0, 1, "22:00", "06:00", time.Date(2024, 1, 2, 5, 59, 0, 0, time.UTC), 1},
		{"10-minute window forces immediate deadline guard", 79, 80, "22:00", "22:10", time.Date(2024, 1, 1, 22, 9, 0, 0, time.UTC), 1},
		{"tiny top-up, very short window", 99, 100, "01:00", "01:05", time.Date(2024, 1, 1, 1, 4, 0, 0, time.UTC), 1},
		// Midnight-crossing case (ties into Part 1's fix): now is inside
		// yesterday's still-open instance of the overnight window.
		{"midnight-crossing window still open after midnight", 20, 80, "22:00", "06:00", time.Date(2024, 1, 2, 2, 0, 0, 0, time.UTC), 240},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
			scheduleRepo.listAllResult = []models.Schedule{carbonAwareSchedule(tt.windowStart, tt.windowEnd)}
			plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
			vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: tt.current, TargetPercent: tt.target}
			chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}

			svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
				return tt.estimateMin, nil
			}, nil)

			now := tt.now
			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return now }
			t.Cleanup(func() { scheduleNowFunc = old })

			svc.CheckAndActivateAll(t.Context())
			assert.True(t, chargeAdapter.createPendingCalled, "expected a single-stage session to be created")
			assert.False(t, chargeAdapter.twoStageCalled)
		})
	}
}

func TestScheduleService_CarbonAware_TwoStage_Matrix(t *testing.T) {
	tests := []struct {
		name           string
		current        float64
		target         float64
		windowStart    string
		windowEnd      string
		now            time.Time
		estimateMin    int
		expectTwoStage bool
	}{
		{"stage2 too short", 1, 2, "22:00", "06:00", time.Date(2024, 1, 2, 5, 59, 0, 0, time.UTC), models.MinTwoStageStageDurationMin - 1, false},
		{"comfortably above threshold", 50, 80, "22:00", "06:00", time.Date(2024, 1, 2, 5, 59, 0, 0, time.UTC), 30, true},
		{"already past hold", 79, 80, "22:00", "06:00", time.Date(2024, 1, 2, 5, 59, 0, 0, time.UTC), 30, false},
		// d1=d2=15 (both exactly at the worthwhileness threshold, sum=30min)
		// inside a 35-minute window - short enough that the deadline guard
		// forces stage 1 to start immediately once now is close to the end.
		{"window shorter than d1+d2 forces stage 1 immediately", 20, 80, "22:00", "22:35", time.Date(2024, 1, 1, 22, 34, 0, 0, time.UTC), models.MinTwoStageStageDurationMin, true},
		{"tiny stage 1 does not block two-stage", 63, 80, "22:00", "06:00", time.Date(2024, 1, 2, 5, 59, 0, 0, time.UTC), 30, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, scheduleRepo, plugRepo, vehicleRepo, chargeAdapter := newMockScheduleService()
			scheduleRepo.listAllResult = []models.Schedule{carbonAwareTwoStageSchedule(tt.windowStart, tt.windowEnd)}
			plugRepo.findByIDResult = &models.Plug{ID: testPlugID, VehicleID: stringPtr("v1")}
			vehicleRepo.findByIDResult = &models.Vehicle{ID: "v1", CurrentPercent: tt.current, TargetPercent: tt.target}
			chargeAdapter.createPendingResult = &models.ChargeSession{ID: "sess1"}
			chargeAdapter.twoStageResult = &models.ChargeSession{ID: "sess1"}

			svc.SetCarbonAwareDeps(nil, func(_ *models.Vehicle, _, _ float64) (int, error) {
				return tt.estimateMin, nil
			}, nil)

			now := tt.now
			old := scheduleNowFunc
			scheduleNowFunc = func() time.Time { return now }
			t.Cleanup(func() { scheduleNowFunc = old })

			svc.CheckAndActivateAll(t.Context())
			assert.Equal(t, tt.expectTwoStage, chargeAdapter.twoStageCalled)
			assert.Equal(t, !tt.expectTwoStage, chargeAdapter.createPendingCalled)
		})
	}
}
