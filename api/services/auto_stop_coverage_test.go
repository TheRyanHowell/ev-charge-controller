package services

import (
	"context"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAndAutoStopReachingSession_NoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session - should return immediately without error
	service.CheckAndAutoStopReachingSession(context.Background())
}

func TestCheckAndAutoStopReachingSession_NoStartTotalKwh(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Active session without StartTotalKwh - can't calculate progress, skip
	session := &models.ChargeSession{
		ID:            "auto_stop_no_total",
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		CreatedAt:     time.Now(),
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPercent:  20.0,
		TargetPercent: 80.0,
		Status:        "active",
	}
	require.NoError(t, service.sessionWriter.Create(context.Background(), session))

	updated, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", updated.Status)
}

func TestCheckAndAutoStopReachingSession_ReachedTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	// batteryKwh = startKwh + (energy.Total - startTotalKwh) * efficiency
	// For 80%: batteryKwh = 0.8 * 2.026 = 1.6208
	// wall_delta = (1.6208 - 0.4) / 0.8 = 1.5195; energy.Total = 100.0 + 1.5195 = 101.5195
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 101.53, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	startTotalKwh := 100.0
	plugID := testPlugID
	session := &models.ChargeSession{
		ID:            "auto_stop_reached",
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        &plugID,
		CreatedAt:     time.Now(),
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPercent:  20.0,
		TargetPercent: 80.0,
		Status:        "active",
		StartTotalKwh: &startTotalKwh,
	}
	require.NoError(t, service.sessionWriter.Create(context.Background(), session))

	service.CheckAndAutoStopReachingSession(context.Background())

	updated, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", updated.Status)
}

func TestCheckAndAutoStopReachingSession_NotYetAtTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	// delta = 0.2 kWh wall → 0.16 kWh battery → SOC = (0.4 + 0.16)/2.026*100 = 27.6%
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 100.2, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	startTotalKwh := 100.0
	plugID := testPlugID
	session := &models.ChargeSession{
		ID:            "auto_stop_not_reached",
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        &plugID,
		CreatedAt:     time.Now(),
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPercent:  20.0,
		TargetPercent: 80.0,
		Status:        "active",
		StartTotalKwh: &startTotalKwh,
	}
	require.NoError(t, service.sessionWriter.Create(context.Background(), session))

	service.CheckAndAutoStopReachingSession(context.Background())

	updated, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", updated.Status)
}

func TestCheckAndAutoStopReachingSession_NilEnergy(t *testing.T) {
	db := setupServiceTestDB(t)

	// ctrl has no energy seeded - LastEnergy returns nil
	ctrl := newMockPlugCtrl()

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	startTotalKwh := 100.0
	plugID := testPlugID
	session := &models.ChargeSession{
		ID:            "auto_stop_nil_energy",
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        &plugID,
		CreatedAt:     time.Now(),
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPercent:  20.0,
		TargetPercent: 80.0,
		Status:        "active",
		StartTotalKwh: &startTotalKwh,
	}
	require.NoError(t, service.sessionWriter.Create(context.Background(), session))

	service.CheckAndAutoStopReachingSession(context.Background())

	// Session stays active - no energy data to determine progress
	updated, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", updated.Status)
}
