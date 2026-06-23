package workers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/tasmota"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testWorkerPlugID = "test-plug-workers"
	testWorkerUserID = "test-user-workers"
)

// mockWorkerPlugCtrl implements internal.PlugController for workers tests.
// Energy is seeded via SetEnergy before the test runs.
type mockWorkerPlugCtrl struct {
	energy  *tasmota.EnergyData
	powerOn bool
}

func (m *mockWorkerPlugCtrl) SetPower(_ context.Context, _ string, on bool) error {
	m.powerOn = on
	return nil
}

func (m *mockWorkerPlugCtrl) SetPowerAndWait(_ context.Context, _ string, on bool, _ time.Duration) (bool, error) {
	m.powerOn = on
	return true, nil
}

func (m *mockWorkerPlugCtrl) LastEnergy(_ string) *tasmota.EnergyData {
	return m.energy
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	require.NoError(t, testdb.InsertUser(db, testWorkerUserID, "worker-test@example.com", ""))
	require.NoError(t, testdb.InsertPlug(db, testWorkerPlugID, testWorkerUserID, "Worker Test Plug", "ns-workertest", "worker-topic"))
	return db
}

func seedWorkerVehicle(t *testing.T, db *sql.DB, id, modelID string) {
	t.Helper()
	require.NoError(t, testdb.InsertVehicle(db, id, testWorkerUserID, modelID, id, 20, 80))
}

func setupTestService(t *testing.T) (*services.ChargeSessionService, *sql.DB) {
	db := setupTestDB(t)
	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	service := services.NewChargeSessionService(context.Background(), chargeRepo, vehicleRepo, nil, nil, nil, nil)
	return service, db
}

func setupTestServiceWithEnergy(t *testing.T, energy *tasmota.EnergyData) (*services.ChargeSessionService, *sql.DB) {
	db := setupTestDB(t)
	ctrl := &mockWorkerPlugCtrl{energy: energy}
	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	service := services.NewChargeSessionService(context.Background(), chargeRepo, vehicleRepo, nil, ctrl, nil, nil)
	return service, db
}

func TestNewEnergyPoller(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	poller := NewEnergyPoller(service)
	require.NotNil(t, poller)
	assert.Equal(t, time.Duration(models.PollIntervalSec)*time.Second, poller.pollInterval)
}

func TestEnergyPoller_Start_ContextCancellation(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	poller := NewEnergyPoller(service)
	ctx, cancel := context.WithCancel(t.Context())
	go poller.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestEnergyPoller_saveEnergyReadings_NoActiveSession(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	poller := NewEnergyPoller(service)
	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0, Voltage: 230.0}
	poller.saveEnergyReadings(t.Context(), energy)
}

func TestEnergyPoller_saveEnergyReadings_WithActiveSession(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	_, err := service.StartSession(t.Context(), testWorkerPlugID, "test-vehicle", 20, 80)
	require.NoError(t, err)

	poller := NewEnergyPoller(service)
	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0, Voltage: 230.0}
	poller.saveEnergyReadings(t.Context(), energy)

	repo := repository.NewChargeSessionRepository(db)
	readings, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, readings)
}

func TestCheckPendingSessionTimeout_NoPending(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	checkPendingSessionTimeout(t.Context(), service)
}

func TestCheckPendingSessionTimeout_TimedOut(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	oldTime := time.Now().Add(-2 * pendingSessionTimeout)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "pending-session",
		VehicleID: "test-vehicle",
		UserID:    testWorkerUserID,
		PlugID:    testWorkerPlugID,
		Status:    models.SessionStatusPending,
		CreatedAt: oldTime,
		StartKwh:  0.5,
		TargetKwh: 1.5,
		StartPct:  20,
		TargetPct: 80,
	}))

	checkPendingSessionTimeout(t.Context(), service)

	pending, _ := service.GetPending(context.Background())
	assert.Nil(t, pending)
}

func TestCheckPendingSessionActivation_NoPending(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0}
	checkPendingSessionActivation(t.Context(), service, energy)
}

func TestNewAutoStopChecker(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	checker := NewAutoStopChecker(service)
	require.NotNil(t, checker)
	assert.Equal(t, time.Duration(models.PollIntervalSec)*time.Second, checker.pollInterval)
}

func TestAutoStopChecker_Start_ContextCancellation(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	checker := NewAutoStopChecker(service)
	ctx, cancel := context.WithCancel(t.Context())
	go checker.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestNewScheduleActivator(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	chargeService := services.NewChargeSessionService(context.Background(), chargeRepo, vehicleRepo, nil, nil, nil, nil)
	scheduleService := services.NewScheduleService(scheduleRepo, repository.NewPlugRepository(db), vehicleRepo, chargeService)

	activator := NewScheduleActivator(scheduleService)
	require.NotNil(t, activator)
	assert.Equal(t, time.Duration(models.PollIntervalSec)*time.Second, activator.pollInterval)
}

func TestScheduleActivator_Start_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	chargeService := services.NewChargeSessionService(context.Background(), chargeRepo, vehicleRepo, nil, nil, nil, nil)
	scheduleService := services.NewScheduleService(scheduleRepo, repository.NewPlugRepository(db), vehicleRepo, chargeService)

	activator := NewScheduleActivator(scheduleService)
	ctx, cancel := context.WithCancel(t.Context())
	go activator.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestEnergyPoller_tick_NoActiveSession(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	poller := NewEnergyPoller(service)
	poller.tick(t.Context())

	repo := repository.NewChargeSessionRepository(db)
	readings, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, readings)
}

func TestEnergyPoller_tick_WithActiveSession(t *testing.T) {
	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0, Voltage: 230.0}
	service, db := setupTestServiceWithEnergy(t, energy)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	// StartSession via the service; plugCtrl will be asked for LastEnergy
	_, err := service.StartSession(t.Context(), testWorkerPlugID, "test-vehicle", 20, 80)
	require.NoError(t, err)

	poller := NewEnergyPoller(service)
	poller.tick(t.Context())

	repo := repository.NewChargeSessionRepository(db)
	readings, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, readings)
}

func TestEnergyPoller_tick_NoMQTTData(t *testing.T) {
	// nil plugCtrl → GetEnergy returns nil → tick returns early (no error, no crash)
	service, db := setupTestService(t)
	defer db.Close()

	poller := NewEnergyPoller(service)
	poller.tick(t.Context()) // Should not panic
}

func TestEnergyPoller_saveEnergyReadings_PendingSession(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "pending-session",
		VehicleID: "test-vehicle",
		UserID:    testWorkerUserID,
		PlugID:    testWorkerPlugID,
		Status:    models.SessionStatusPending,
		CreatedAt: time.Now(),
		StartKwh:  0.5,
		TargetKwh: 1.5,
		StartPct:  20,
		TargetPct: 80,
	}))

	poller := NewEnergyPoller(service)
	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0, Voltage: 230.0}
	poller.saveEnergyReadings(t.Context(), energy)

	repo := repository.NewChargeSessionRepository(db)
	readings, err := repo.GetAll(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, readings)

	snapshots, err := repo.GetSOCSnapshots(context.Background(), "pending-session")
	assert.NoError(t, err)
	assert.Empty(t, snapshots)
}

func TestEnergyPoller_saveEnergyReadings_DBError(t *testing.T) {
	service, db := setupTestService(t)

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	_, err := service.StartSession(t.Context(), testWorkerPlugID, "test-vehicle", 20, 80)
	require.NoError(t, err)

	db.Close()

	poller := NewEnergyPoller(service)
	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0, Voltage: 230.0}
	poller.saveEnergyReadings(t.Context(), energy) // Should not panic on DB error
}

func TestCheckPendingSessionTimeout_NotTimedOut(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "pending-session",
		VehicleID: "test-vehicle",
		UserID:    testWorkerUserID,
		PlugID:    testWorkerPlugID,
		Status:    models.SessionStatusPending,
		CreatedAt: time.Now(),
		StartKwh:  0.5,
		TargetKwh: 1.5,
		StartPct:  20,
		TargetPct: 80,
	}))

	checkPendingSessionTimeout(t.Context(), service)

	pending, _ := service.GetPending(context.Background())
	assert.NotNil(t, pending)
	assert.Equal(t, "pending-session", pending.ID)
}

func TestCheckPendingSessionActivation_Activates(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "pending-session",
		VehicleID: "test-vehicle",
		UserID:    testWorkerUserID,
		PlugID:    testWorkerPlugID,
		Status:    models.SessionStatusPending,
		CreatedAt: time.Now(),
		StartKwh:  0.5,
		TargetKwh: 1.5,
		StartPct:  20,
		TargetPct: 80,
	}))

	// Power above threshold → activates pending session
	energy := &tasmota.EnergyData{Power: 1500.0}
	checkPendingSessionActivation(t.Context(), service, energy)

	active, err := service.GetActiveSession(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, "pending-session", active.ID)
}

func TestCheckPendingSessionActivation_BelowThreshold(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "pending-session",
		VehicleID: "test-vehicle",
		UserID:    testWorkerUserID,
		PlugID:    testWorkerPlugID,
		Status:    models.SessionStatusPending,
		CreatedAt: time.Now(),
		StartKwh:  0.5,
		TargetKwh: 1.5,
		StartPct:  20,
		TargetPct: 80,
	}))

	// Power below threshold → stays pending
	energy := &tasmota.EnergyData{Power: 1.0}
	checkPendingSessionActivation(t.Context(), service, energy)

	pending, _ := service.GetPending(context.Background())
	assert.NotNil(t, pending)
}

func TestAutoStopChecker_Start_RunsCheck(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	checker := NewAutoStopChecker(service)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		checker.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
}

func TestScheduleActivator_Start_RunsCheck(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	chargeService := services.NewChargeSessionService(context.Background(), chargeRepo, vehicleRepo, nil, nil, nil, nil)
	scheduleService := services.NewScheduleService(scheduleRepo, repository.NewPlugRepository(db), vehicleRepo, chargeService)

	activator := NewScheduleActivator(scheduleService)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		activator.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
}

func TestEnergyPoller_saveEnergyReadings_SkipsAfterStop(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	seedWorkerVehicle(t, db, "test-vehicle", "rm1")

	session, err := service.StartSession(t.Context(), testWorkerPlugID, "test-vehicle", 20, 80)
	require.NoError(t, err)

	poller := NewEnergyPoller(service)
	energy := &tasmota.EnergyData{Total: 100.0, Power: 600.0}

	// Stop the session before saving readings
	_, err = service.Stop(t.Context())
	require.NoError(t, err)

	// saveEnergyReadings should skip because session is no longer active
	poller.saveEnergyReadings(t.Context(), energy)

	repo := repository.NewChargeSessionRepository(db)
	readings, err := repo.GetPowerReadings(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Empty(t, readings, "should not save readings for stopped session")

	snapshots, err := repo.GetSOCSnapshots(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Empty(t, snapshots, "should not save SOC snapshots for stopped session")
}

func TestEnergyPoller_Start_SkipsOverlappingTicks(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	poller := NewEnergyPoller(service)
	poller.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		poller.Start(ctx)
		close(done)
	}()

	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done
}

func TestAutoStopChecker_Start_SkipsOverlappingTicks(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()

	checker := NewAutoStopChecker(service)
	checker.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		checker.Start(ctx)
		close(done)
	}()

	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done
}

func TestScheduleActivator_Start_SkipsOverlappingTicks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	chargeService := services.NewChargeSessionService(context.Background(), chargeRepo, vehicleRepo, nil, nil, nil, nil)
	scheduleService := services.NewScheduleService(scheduleRepo, repository.NewPlugRepository(db), vehicleRepo, chargeService)

	activator := NewScheduleActivator(scheduleService)
	activator.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		activator.Start(ctx)
		close(done)
	}()

	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done
}
