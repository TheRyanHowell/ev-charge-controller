package services

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"ev-charge-controller/api/carbonintensity"
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

func (m *mockScheduleRepo) Get(context.Context) (*models.Schedule, error)                       { return nil, nil }
func (m *mockScheduleRepo) Upsert(context.Context, *models.Schedule) error                      { return nil }
func (m *mockScheduleRepo) GetByPlugID(context.Context, string) (*models.Schedule, error)       { return m.getByPlugResult, m.getByPlugErr }
func (m *mockScheduleRepo) UpsertByPlugID(context.Context, *models.Schedule) error              { return m.upsertErr }
func (m *mockScheduleRepo) ListAll(context.Context) ([]models.Schedule, error)                  { return m.listAllResult, m.listAllErr }

// mockPlugRepo implements internal.PlugRepo for error injection.
type mockPlugRepo struct {
	findByIDErr    error
	findByIDResult *models.Plug
}

func (m *mockPlugRepo) Create(context.Context, *models.Plug) error                                   { return nil }
func (m *mockPlugRepo) FindByID(context.Context, string) (*models.Plug, error)                        { return m.findByIDResult, m.findByIDErr }
func (m *mockPlugRepo) FindByNamespaceAndSlug(context.Context, string, string) (*models.Plug, error) { return nil, nil }
func (m *mockPlugRepo) ListNamespacesByUserID(context.Context, string) ([]string, error)              { return nil, nil }
func (m *mockPlugRepo) List(context.Context, string) ([]models.Plug, error)                           { return nil, nil }
func (m *mockPlugRepo) Update(context.Context, *models.Plug) error                                    { return nil }
func (m *mockPlugRepo) Delete(context.Context, string, string) error                                  { return nil }
func (m *mockPlugRepo) SetOnline(context.Context, string, bool) error                                 { return nil }
func (m *mockPlugRepo) UpdateLastOfflineNotifiedAt(context.Context, string) error                     { return nil }
func (m *mockPlugRepo) SetInitialized(context.Context, string) error  { return nil }
func (m *mockPlugRepo) SetPowerState(context.Context, string, bool) error { return nil }

// mockVehicleRepo implements internal.VehicleRepo for error injection.
type mockVehicleRepo struct {
	findByIDErr    error
	findByIDResult *models.Vehicle
}

func (m *mockVehicleRepo) FindByID(context.Context, string) (*models.Vehicle, error)                                   { return m.findByIDResult, m.findByIDErr }
func (m *mockVehicleRepo) FindByIDs(context.Context, []string) (map[string]*models.Vehicle, error)                    { return nil, nil }
func (m *mockVehicleRepo) List(context.Context) ([]models.Vehicle, error)                                              { return nil, nil }
func (m *mockVehicleRepo) UpdatePercents(context.Context, string, float64, float64) error                              { return nil }
func (m *mockVehicleRepo) UpdateName(context.Context, string, string, string) error                                    { return nil }
func (m *mockVehicleRepo) CreateInstance(context.Context, *models.Vehicle) error                                       { return nil }
func (m *mockVehicleRepo) DeleteInstance(context.Context, string, string) error                                        { return nil }
func (m *mockVehicleRepo) IncrementLifetimeStats(context.Context, string, float64, float64, float64, float64, time.Time) error { return nil }
func (m *mockVehicleRepo) DecrementLifetimeStats(context.Context, string, float64, float64, float64, float64) error             { return nil }
func (m *mockVehicleRepo) UpdateNotificationPrefs(context.Context, string, string, bool, bool, bool, bool) error                { return nil }

// mockChargeServiceAdapter implements ChargeServiceAdapter for error injection.
type mockChargeServiceAdapter struct {
	getActiveByPlugErr    error
	getActiveByPlugResult *models.ChargeSession
	createPendingErr      error
	createPendingResult   *models.ChargeSession
	createPendingCalled   bool

	twoStageErr        error
	twoStageResult     *models.ChargeSession
	twoStageCalled     bool
	twoStageHoldArg    float64
	twoStageReadyByArg string
}

func (m *mockChargeServiceAdapter) GetActiveByPlug(context.Context, string) (*models.ChargeSession, error) {
	return m.getActiveByPlugResult, m.getActiveByPlugErr
}

func (m *mockChargeServiceAdapter) StartSession(context.Context, string, string, float64, float64) (*models.ChargeSession, error) {
	m.createPendingCalled = true
	return m.createPendingResult, m.createPendingErr
}

func (m *mockChargeServiceAdapter) StartTwoStageSession(_ context.Context, _, _ string, _, _, holdPercent float64, readyByTime string) (*models.ChargeSession, error) {
	m.twoStageCalled = true
	m.twoStageHoldArg = holdPercent
	m.twoStageReadyByArg = readyByTime
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

	sch, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "22:00", "06:00", true)
	require.NoError(t, err)
	require.NotNil(t, sch)
	assert.Equal(t, models.ScheduleTypeCarbonAware, sch.Type)
	assert.Equal(t, "22:00", *sch.WindowStart)
	assert.Equal(t, "06:00", *sch.WindowEnd)
	assert.True(t, sch.Enabled)
}

func TestScheduleService_UpsertCarbonAware_EmptyUserID(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, "", "09:00", "13:00", true)
	assert.ErrorIs(t, err, ErrUserIDRequired)
}

func TestScheduleService_UpsertCarbonAware_MissingWindow(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "", "13:00", true)
	assert.ErrorIs(t, err, ErrWindowRequired)

	_, err = svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "09:00", "", true)
	assert.ErrorIs(t, err, ErrWindowRequired)
}

func TestScheduleService_UpsertCarbonAware_EqualWindows(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "09:00", "09:00", true)
	assert.ErrorIs(t, err, ErrWindowEqual)
}

func TestScheduleService_UpsertCarbonAware_UpdateExisting(t *testing.T) {
	svc, db, _ := setupScheduleServiceTest(t)
	defer db.Close()

	_, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "22:00", "06:00", true)
	require.NoError(t, err)

	// Update with different window
	sch, err := svc.UpsertCarbonAware(t.Context(), testPlugID, testUserID, "20:00", "05:00", false)
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
	tests := []struct{ input string; h, m int }{
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
	// Tie at 10:30 and 11:00 - both score 300 - findOptimalStart returns first encountered (10:30)
	buckets := makeBuckets(now, []int{400, 400, 200, 400})
	latestStart := now.Add(60 * time.Minute) // 11:00

	optimal := findOptimalStart(buckets, now, latestStart, 60*time.Minute)
	assert.True(t, optimal.After(now.UTC()), "optimal should be after now (a later bucket)")
}

func TestFindOptimalStart_EmptyBuckets(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	optimal := findOptimalStart(nil, now, now.Add(time.Hour), 30*time.Minute)
	assert.True(t, optimal.IsZero())
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
