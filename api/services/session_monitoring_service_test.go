package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionMonitoringService_GetEnergy_ReturnsNilWhenNoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	energy, err := service.GetEnergy(context.Background())
	require.NoError(t, err)
	assert.Nil(t, energy)
}

func TestSessionMonitoringService_GetEnergy_ReturnsEnergyForActivePlug(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)


	session := &models.ChargeSession{

		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.0, Power: 600})

	energy, err := service.GetEnergy(context.Background())
	require.NoError(t, err)
	require.NotNil(t, energy)
	assert.Greater(t, energy.Power, float64(0))
}

func TestSessionMonitoringService_SetPowerState(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// No active session - no-op, no error
	err := service.SetPowerState(context.Background(), true)
	require.NoError(t, err)

	err = service.SetPowerState(context.Background(), false)
	require.NoError(t, err)
}

func TestSessionMonitoringService_AddPowerReading(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	reading := &models.PowerReading{
		SessionID: session.ID,
		EnergyKwh: 100.0,
		Power:     600.0,
		Voltage:   230.0,
		Current:   2.6,
		Timestamp: time.Now(),
	}

	err := service.AddPowerReading(context.Background(), reading)
	require.NoError(t, err)
}

func TestSessionMonitoringService_GetLastCompleted(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	completed, err := service.GetLastCompleted(context.Background())
	require.NoError(t, err)
	assert.Nil(t, completed)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusCompleted,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	completed, err = service.GetLastCompleted(context.Background())
	require.NoError(t, err)
	require.NotNil(t, completed)
	assert.Equal(t, session.ID, completed.ID)
}

func TestSessionMonitoringService_StoreSOCSnapshot_NoVehicle(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID: "nonexistent",
		StartKwh:  0.38,
	}
	energy := &tasmota.EnergyData{
		Total: 1000,
		Power: 600,
	}

	err := service.StoreSOCSnapshot(context.Background(), session, energy)
	require.NoError(t, err)
}

func TestSessionMonitoringService_SaveEnergyReadings_NoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	energy := &tasmota.EnergyData{Total: 1000, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)
}

func TestSessionMonitoringService_SaveEnergyReadings_ActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy := &tasmota.EnergyData{Total: 1000, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 1)
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_PendingSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_NilEnergy(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// Active session with no MQTT energy cached
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_SaveEnergyReadings_AttachesCarbonIntensity(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	ciClient := &mockCarbonIntensityFetcher{value: 220}

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, ciClient, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy := &tasmota.EnergyData{Total: 1000, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	require.NotNil(t, readings[0].CarbonIntensityGCo2PerKwh)
	assert.InDelta(t, 220, *readings[0].CarbonIntensityGCo2PerKwh, 0.001)
}

func TestSessionMonitoringService_SaveEnergyReadings_ConditioningSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy := &tasmota.EnergyData{Total: 1000, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 1)
}

func TestSessionMonitoringService_SaveEnergyReadings_SkipsDuplicate(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy := &tasmota.EnergyData{Total: 1000, Power: 600, Voltage: 230, Current: 2.6}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 1, "duplicate readings with same values should be deduplicated")
}

func TestSessionMonitoringService_SaveEnergyReadings_StoresWhenValueChanges(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy1 := &tasmota.EnergyData{Total: 1000, Power: 600, Voltage: 230, Current: 2.6}
	energy2 := &tasmota.EnergyData{Total: 1001, Power: 650, Voltage: 230, Current: 2.8}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy1)
	service.SaveEnergyReadings(context.Background(), testPlugID, energy2)

	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 2, "changed values should always be stored")
}

func TestSessionMonitoringService_SaveEnergyReadings_StoresWhenStale(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	staleReading := &models.PowerReading{
		ID:        "stale-id",
		SessionID: session.ID,
		Timestamp: time.Now().Add(-31 * time.Minute),
		Power:     600,
		Voltage:   230,
		Current:   2.6,
		EnergyKwh: 1000,
	}
	require.NoError(t, sessRepo.CreatePowerReading(context.Background(), staleReading))

	energy := &tasmota.EnergyData{Total: 1000, Power: 600, Voltage: 230, Current: 2.6}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 2, "stale readings should be refreshed regardless of value")
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_TransitionsToConditioning(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0

	session := &models.ChargeSession{

		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: models.MaxPercent,
		TargetKwh:     1.9,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 2000.0, Power: 600})

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusConditioning, updated.Status)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_TransitionsToHolding(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0
	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	require.NoError(t, ctrl.SetPower(context.Background(), testPlugID, true))
	// Plenty of energy to clear holdKwh (0.64*1.9=1.216); clamps at TargetKwh (1.52).
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 2000.0, Power: 600})

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status)
	assert.False(t, stopper.stopped)
	assert.False(t, ctrl.powerOn[testPlugID], "plug should be powered off while holding")
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_HoldNotYetReached(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0
	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	require.NoError(t, ctrl.SetPower(context.Background(), testPlugID, true))
	// Tiny amount of energy - nowhere near holdKwh (1.216).
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 0.05, Power: 600})

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, updated.Status)
	assert.True(t, ctrl.powerOn[testPlugID], "plug should remain on")
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_HoldPowerOffNotConfirmed(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	ctrl.setPowerErr = assert.AnError
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0
	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 2000.0, Power: 600})

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, updated.Status, "should retry next tick, not transition on unconfirmed power-off")
}

func TestSessionMonitoringService_CheckAndStopConditioningSession_BelowThreshold(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)


	session := &models.ChargeSession{

		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// rm1 vehicle has ChargerOutputW=600W; threshold=60W. Power=0 → below threshold.
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.0, Power: 0})

	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.True(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopConditioningSession_AboveThreshold(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)


	session := &models.ChargeSession{

		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// rm1 vehicle ChargerOutputW=600W; threshold=60W. Power=580W → above threshold.
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.0, Power: 580})

	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_NotActiveOrConditioning(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_NoStartedAt(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// Created directly as Active (bypassing ActivatePending) leaves StartedAt nil.
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_BelowMinDuration(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	// Started 2 minutes ago - below MinSessionDurationBeforeIdleCheck (10 min).
	require.NoError(t, sessRepo.ActivatePending(context.Background(), session.ID, time.Now().Add(-2*time.Minute)))

	now := time.Now()
	seedIdlePowerReadings(t, sessRepo, session.ID, now.Add(-20*time.Minute), now, 5)

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_NoReadings(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	require.NoError(t, sessRepo.ActivatePending(context.Background(), session.ID, time.Now().Add(-30*time.Minute)))

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_CurrentlyCharging(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	require.NoError(t, sessRepo.ActivatePending(context.Background(), session.ID, time.Now().Add(-30*time.Minute)))

	now := time.Now()
	// Idle for 20 minutes, but the most recent reading shows real current again.
	seedIdlePowerReadings(t, sessRepo, session.ID, now.Add(-20*time.Minute), now.Add(-time.Minute), 5)
	require.NoError(t, sessRepo.CreatePowerReading(context.Background(), &models.PowerReading{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Timestamp: now,
		Power:     900,
		Current:   4,
		Voltage:   240,
		EnergyKwh: 1.7,
	}))

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_IdleBelowTimeout(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	require.NoError(t, sessRepo.ActivatePending(context.Background(), session.ID, time.Now().Add(-30*time.Minute)))

	now := time.Now()
	// Idle streak only 5 minutes long - below IdleSessionTimeout (15 min).
	seedIdlePowerReadings(t, sessRepo, session.ID, now.Add(-5*time.Minute), now, 5)

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_IdleBeyondTimeout_Active(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	require.NoError(t, sessRepo.ActivatePending(context.Background(), session.ID, time.Now().Add(-90*time.Minute)))

	now := time.Now()
	// Real charging for the first hour, then idle for the last 20 minutes -
	// mirrors the production example (charging, then BMS stops before target).
	seedIdlePowerReadings(t, sessRepo, session.ID, now.Add(-60*time.Minute), now.Add(-21*time.Minute), 900)
	seedIdlePowerReadings(t, sessRepo, session.ID, now.Add(-20*time.Minute), now, 5)

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.True(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_IdleBeyondTimeout_Conditioning(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  80,
		StartKwh:      1.62,
		TargetPercent: 100,
		TargetKwh:     2.03,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	require.NoError(t, sessRepo.ActivatePending(context.Background(), session.ID, time.Now().Add(-90*time.Minute)))
	require.NoError(t, sessRepo.UpdateStatus(context.Background(), session.ID, models.SessionStatusConditioning))

	now := time.Now()
	seedIdlePowerReadings(t, sessRepo, session.ID, now.Add(-20*time.Minute), now, 5)

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.True(t, stopper.stopped)
}

func TestSessionMonitoringService_CheckAndStopIdleSession_GetActiveError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	require.NoError(t, db.Close())

	stopper := &mockSessionStopper{}
	service.CheckAndStopIdleSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

// seedIdlePowerReadings inserts evenly spaced power readings between start and
// end (inclusive), all below models.IdlePowerThresholdW, for the given power draw.
func seedIdlePowerReadings(t *testing.T, sessRepo *repository.ChargeSessionRepository, sessionID string, start, end time.Time, powerW float64) {
	t.Helper()
	const steps = 4
	span := end.Sub(start)
	for i := 0; i <= steps; i++ {
		ts := start.Add(time.Duration(i) * span / steps)
		require.NoError(t, sessRepo.CreatePowerReading(context.Background(), &models.PowerReading{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			Timestamp: ts,
			Power:     powerW,
			Current:   0.2,
			Voltage:   250,
			EnergyKwh: 1.7,
		}))
	}
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_NoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// Should not panic or error with no active session.
	service.CheckAndResumeHoldingSession(context.Background())
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_NotHolding(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, updated.Status)
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_EstimatorErrorFailsafeResumes(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 0, errors.New("no estimate")
	})

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusHolding,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, updated.Status)
	assert.Nil(t, updated.HoldPercent)
	assert.True(t, ctrl.powerOn[testPlugID])
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_WaitsBeforeLatestStart(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil // 30 minutes remaining
	})

	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC) // readyBy 23:59 - 30min = well after now
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusHolding,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status, "should still be waiting, far from latestStart")
	assert.False(t, ctrl.powerOn[testPlugID])
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_ResumesAtLatestStart(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil // 30 minutes remaining
	})

	mockNow := time.Date(2024, 1, 1, 23, 29, 0, 0, time.UTC) // readyBy 23:59 - 30min = 23:29
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusHolding,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, updated.Status)
	assert.Nil(t, updated.HoldPercent)
	assert.True(t, ctrl.powerOn[testPlugID])
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_CarbonAware_WaitsForBetterWindow(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	// Same worked scenario as TestScheduleService_CarbonAwareTwoStage_BetterWindowLater_Waits:
	// deadline=12:30, d=30min -> latestStart=12:00, candidates 10:00..12:00 balance to a
	// clear winner at 11:00, not the current bucket.
	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	buckets := makeBuckets(mockNow, []int{500, 300, 100, 300, 500})
	service.SetForecaster(&mockForecaster{buckets: buckets})

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "12:30"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		UserID:          testUserIDPtr,
		PlugID:          testPlugIDPtr,
		StartPercent:    20,
		StartKwh:        0.38,
		TargetPercent:   80,
		TargetKwh:       1.52,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status, "a better-balanced window exists later, should keep holding")
	assert.False(t, ctrl.powerOn[testPlugID])
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_CarbonAware_ResumesWhenOptimalIsNow(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	// deadline=10:45, d=30min -> latestStart=10:15: strictly after now (not deadline-guarded)
	// but less than one bucket away, so the search collapses to a single candidate (now).
	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	buckets := makeBuckets(mockNow, []int{300})
	service.SetForecaster(&mockForecaster{buckets: buckets})

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "10:45"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		UserID:          testUserIDPtr,
		PlugID:          testPlugIDPtr,
		StartPercent:    20,
		StartKwh:        0.38,
		TargetPercent:   80,
		TargetKwh:       1.52,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, updated.Status, "single feasible candidate should resume now via the forecast path")
	assert.Nil(t, updated.HoldPercent)
	assert.True(t, ctrl.powerOn[testPlugID])
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_CarbonAware_NoForecaster_FallsBackToDeadlineGuard(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	// No forecaster wired at all - carbon-aware-origin session should behave exactly like
	// a daily-origin one: plain deadline guard, well before latestStart -> keep holding.
	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		UserID:          testUserIDPtr,
		PlugID:          testPlugIDPtr,
		StartPercent:    20,
		StartKwh:        0.38,
		TargetPercent:   80,
		TargetKwh:       1.52,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status, "without a forecaster, should fall back to plain deadline guard")
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_CarbonAware_ForecastError_Defers(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	service.SetForecaster(&mockForecaster{err: errors.New("API down")})

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		UserID:          testUserIDPtr,
		PlugID:          testPlugIDPtr,
		StartPercent:    20,
		StartKwh:        0.38,
		TargetPercent:   80,
		TargetKwh:       1.52,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status, "forecast error should defer rather than resume early")
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_DailyOrigin_IgnoresForecaster(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// A forecaster that would say "resume now" if it were (incorrectly) consulted.
	service.SetForecaster(&mockForecaster{buckets: makeBuckets(mockNow, []int{100})})

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		UserID:          testUserIDPtr,
		PlugID:          testPlugIDPtr,
		StartPercent:    20,
		StartKwh:        0.38,
		TargetPercent:   80,
		TargetKwh:       1.52,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: false,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status, "daily-origin holds must ignore the forecaster entirely")
}

func TestSessionMonitoringService_CheckAndResumeHoldingSession_PowerOnNotConfirmed(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	ctrl.setPowerErr = assert.AnError
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 0, errors.New("no estimate")
	})

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusHolding,
		HoldPercent:   &holdPercent,
		ReadyByTime:   &readyByTime,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	service.CheckAndResumeHoldingSession(context.Background())

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusHolding, updated.Status, "should retry next tick, not resume on unconfirmed power-on")
	require.NotNil(t, updated.HoldPercent)
}

// --- EstimateResumeTime tests ---

func TestSessionMonitoringService_EstimateResumeTime_NilSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	_, ok := service.EstimateResumeTime(context.Background(), nil)
	assert.False(t, ok)
}

func TestSessionMonitoringService_EstimateResumeTime_NotHolding(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		Status:          models.SessionStatusActive,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}

	_, ok := service.EstimateResumeTime(context.Background(), session)
	assert.False(t, ok)
}

func TestSessionMonitoringService_EstimateResumeTime_DailyOrigin(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetForecaster(&mockForecaster{buckets: makeBuckets(time.Now(), []int{100})})

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: false,
	}

	_, ok := service.EstimateResumeTime(context.Background(), session)
	assert.False(t, ok, "daily-origin holds have no forecast-based resume estimate")
}

func TestSessionMonitoringService_EstimateResumeTime_NoForecaster(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}

	_, ok := service.EstimateResumeTime(context.Background(), session)
	assert.False(t, ok)
}

func TestSessionMonitoringService_EstimateResumeTime_PastDeadline_ReturnsLatestStart(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	service.SetForecaster(&mockForecaster{})

	mockNow := time.Date(2024, 1, 1, 23, 45, 0, 0, time.UTC) // readyBy 23:59 - 30min = 23:29, already past
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		TargetPercent:   80,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}

	resumeTime, ok := service.EstimateResumeTime(context.Background(), session)
	require.True(t, ok)
	assert.Equal(t, "23:29", resumeTime)
}

func TestSessionMonitoringService_EstimateResumeTime_ForecastPicksBalancedWindow(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	// Same worked scenario as the CheckAndResumeHoldingSession carbon-aware test:
	// deadline=12:30, d=30min -> latestStart=12:00, candidates 10:00..12:00 balance to 11:00.
	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	buckets := makeBuckets(mockNow, []int{500, 300, 100, 300, 500})
	service.SetForecaster(&mockForecaster{buckets: buckets})

	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "12:30"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		TargetPercent:   80,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}

	resumeTime, ok := service.EstimateResumeTime(context.Background(), session)
	require.True(t, ok)
	assert.Equal(t, "11:00", resumeTime)
}

func TestSessionMonitoringService_EstimateResumeTime_ForecastError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)
	service.SetEstimator(func(*models.Vehicle, float64, float64) (int, error) {
		return 30, nil
	})
	service.SetForecaster(&mockForecaster{err: errors.New("API down")})

	mockNow := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:59"
	session := &models.ChargeSession{
		VehicleID:       testVehicleID,
		Status:          models.SessionStatusHolding,
		HoldPercent:     &holdPercent,
		TargetPercent:   80,
		ReadyByTime:     &readyByTime,
		CarbonAwareHold: true,
	}

	_, ok := service.EstimateResumeTime(context.Background(), session)
	assert.False(t, ok)
}

func TestSessionMonitoringService_StoreSOCSnapshot_SkipsDuplicate(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy := &tasmota.EnergyData{Total: 0.5, Power: 600}
	require.NoError(t, service.StoreSOCSnapshot(context.Background(), session, energy))
	require.NoError(t, service.StoreSOCSnapshot(context.Background(), session, energy))

	snapshots, err := sessRepo.GetSOCSnapshots(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, snapshots, 1, "duplicate SOC snapshots with same value should be deduplicated")
}

func TestSessionMonitoringService_StoreSOCSnapshot_StoresWhenValueChanges(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy1 := &tasmota.EnergyData{Total: 0.5, Power: 600}
	energy2 := &tasmota.EnergyData{Total: 1.0, Power: 600}
	require.NoError(t, service.StoreSOCSnapshot(context.Background(), session, energy1))
	require.NoError(t, service.StoreSOCSnapshot(context.Background(), session, energy2))

	snapshots, err := sessRepo.GetSOCSnapshots(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, snapshots, 2, "changed SOC value should always be stored")
}

func TestSessionMonitoringService_StoreSOCSnapshot_StoresWhenStale(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	startTotal := 0.0
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:       testUserIDPtr,
		PlugID:       testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	staleSnapshot := &models.SOCSnapshot{
		ID:         "stale-snap",
		SessionID:  session.ID,
		SocPercent: 45.0,
		Timestamp:  time.Now().Add(-31 * time.Minute),
	}
	require.NoError(t, sessRepo.CreateSOCSnapshot(context.Background(), staleSnapshot))

	energy := &tasmota.EnergyData{Total: startTotal + 0.6647, Power: 600}
	require.NoError(t, service.StoreSOCSnapshot(context.Background(), session, energy))

	snapshots, err := sessRepo.GetSOCSnapshots(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, snapshots, 2, "stale SOC snapshot should be refreshed regardless of value")
}

type mockSessionStopper struct {
	stopped bool
}

func (m *mockSessionStopper) stopWithPercent(_ context.Context, _ *models.ChargeSession, _ float64, _ ...models.StopReason) (*StopResult, error) {
	m.stopped = true
	return &StopResult{Stopped: true}, nil
}

type mockCarbonIntensityFetcher struct {
	value int
}

func (m *mockCarbonIntensityFetcher) GetCurrent(_ context.Context) (*carbonintensity.CarbonIntensity, error) {
	return &carbonintensity.CarbonIntensity{Actual: m.value}, nil
}

func TestCarbonIntensityDiffers(t *testing.T) {
	a := float64(220)
	b := float64(220)
	c := float64(250)

	assert.True(t, carbonIntensityDiffers(nil, &a))
	assert.True(t, carbonIntensityDiffers(&a, nil))
	assert.False(t, carbonIntensityDiffers(nil, nil))
	assert.False(t, carbonIntensityDiffers(&a, &b))
	assert.True(t, carbonIntensityDiffers(&a, &c))
}

func TestCheckAndStopConditioningSession_NoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestCheckAndStopConditioningSession_NotConditioning(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestCheckAndStopConditioningSession_NoEnergy(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// No energy set on controller - should skip
	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestCheckAndStopConditioningSession_NoPlugController(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	// nil plug controller
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, nil, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSetPowerState_WithActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.SetPowerState(context.Background(), false)
	require.NoError(t, err)

	ctrl.mu.RLock()
	defer ctrl.mu.RUnlock()
	assert.Equal(t, false, ctrl.powerOn[testPlugID])
}

func TestSetPowerState_NilPlugController(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, nil, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.SetPowerState(context.Background(), true)
	require.NoError(t, err)
}

func TestSaveEnergyReadings_ConditioningNoSOCOffload(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	energy := &tasmota.EnergyData{Total: 1000, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	// Power reading should be saved
	readings, err := sessRepo.GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 1)
}

func TestSaveEnergyReadings_DBError_LogsAndContinues(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// First read succeeds
	energy := &tasmota.EnergyData{Total: 1000, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)

	// Close DB to force error on second call
	require.NoError(t, db.Close())

	// Should not panic
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)
}

func TestSessionMonitoringService_SetPowerState_GetActiveError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// Close DB to force GetActive error
	require.NoError(t, db.Close())

	err := service.SetPowerState(context.Background(), true)
	assert.Error(t, err)
}

func TestSessionMonitoringService_StoreSOCSnapshot_VehicleError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		StartKwh:  0.38,
	}
	energy := &tasmota.EnergyData{Total: 1000, Power: 600}

	// Close DB to force FindByID error
	require.NoError(t, db.Close())

	err := service.StoreSOCSnapshot(context.Background(), session, energy)
	assert.Error(t, err)
}

func TestSessionMonitoringService_CurrentCarbonIntensity_Error(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	// Mock carbon intensity that returns an error
	ciClient := &mockCarbonIntensityFetcherError{}
	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, ciClient, socWorker, lock)

	ci := service.currentCarbonIntensity(context.Background())
	assert.Nil(t, ci) // Error should be swallowed, nil returned
}

func TestSessionMonitoringService_CheckAndStopConditioningSession_VehicleError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// Close DB to force FindByID error
	require.NoError(t, db.Close())

	stopper := &mockSessionStopper{}
	service.CheckAndStopConditioningSession(context.Background(), stopper)
	assert.False(t, stopper.stopped) // Error path should not stop
}

func TestSessionMonitoringService_CheckAndAutoStopReachingSession_GetActiveError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// Close DB to force GetActive error
	require.NoError(t, db.Close())

	stopper := &mockSessionStopper{}
	service.CheckAndAutoStopReachingSession(context.Background(), stopper)
	assert.False(t, stopper.stopped)
}

func TestSessionMonitoringService_GetEnergy_GetActiveError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	socWorker := NewSOCWorker(nil)
	lock := newSessionLock()

	service := NewSessionMonitoringService(sessRepo, sessRepo, sessRepo, sessRepo, vehicleRepo, ctrl, nil, socWorker, lock)

	// Close DB to force GetActive error
	require.NoError(t, db.Close())

	energy, err := service.GetEnergy(context.Background())
	// GetEnergy swallows errors and returns nil
	require.NoError(t, err)
	assert.Nil(t, energy)
}

type mockCarbonIntensityFetcherError struct{}

func (m *mockCarbonIntensityFetcherError) GetCurrent(_ context.Context) (*carbonintensity.CarbonIntensity, error) {
	return nil, assert.AnError
}
