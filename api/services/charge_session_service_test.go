package services

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChargeSessionService_Stop_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	result, err := service.StopWithPercent(context.Background(), "nonexistent", 80.0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, result)
}

func TestChargeSessionService_GetActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session
	session, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.Nil(t, session)

	// Create active session
	pending, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	assert.NotNil(t, pending)

	// Get active session
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, pending.ID, active.ID)
}

// Regression: a session that has been charging for a while (persisted
// LastBlendedKwh) must report its accrued currentPercent even when the plug has
// no live energy reading at the moment of the request. Previously the read path
// returned a bare view (no currentPercent) whenever live energy was absent or
// reported 0W, so a fresh SSR page load showed the start percent and the gauge
// only jumped to the real value after the first client poll.
func TestChargeSessionService_GetActiveByVehicle_NoLiveEnergy_ReportsBlendedPercent(t *testing.T) {
	db := setupServiceTestDB(t)

	// Mock controller with no cached energy → LastEnergy returns nil, mirroring a
	// plug that is between MQTT ticks on a fresh page load.
	ctrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	vehicle, err := service.FindVehicleByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	require.Greater(t, vehicle.CapacityKwh, 0.0)

	// Session charged from 21% to ~29% (persisted blended kWh) but no live energy.
	const startPct, targetPct, blendedPct = 21.0, 80.0, 29.0
	startKwh := startPct / 100 * vehicle.CapacityKwh
	targetKwh := targetPct / 100 * vehicle.CapacityKwh
	blendedKwh := blendedPct / 100 * vehicle.CapacityKwh
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:             "blended-no-energy-session",
		VehicleID:      testVehicleID,
		UserID:         testUserID,
		PlugID:         testPlugID,
		Status:         models.SessionStatusActive,
		StartKwh:       startKwh,
		TargetKwh:      targetKwh,
		StartPct:       startPct,
		TargetPct:      targetPct,
		StartTotalKwh:  ptrFloat64(10.0),
		LastBlendedKwh: ptrFloat64(blendedKwh),
		StartedAt:      ptrTime(time.Now().Add(-30 * time.Minute)),
	}))

	view, err := service.GetActiveByVehicle(context.Background(), testVehicleID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.CurrentPercent,
		"currentPercent must reflect persisted blended kWh when no live energy reading is available")
	assert.InDelta(t, blendedPct, *view.CurrentPercent, 0.01)
}

func TestChargeSessionService_AddPowerReading(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	// Add a power reading
	reading := &models.PowerReading{
		SessionID: session.ID,
		Timestamp: time.Now(),
		Voltage:   230.0,
		Current:   2.6,
		Power:     600.0,
		EnergyKwh: 1500.0,
	}

	err = service.AddPowerReading(context.Background(), reading)
	require.NoError(t, err)

	// Verify reading was saved
	readings, err := repository.NewChargeSessionRepository(db).GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, readings, 1)
	assert.Equal(t, 600.0, readings[0].Power)
}

func TestChargeSessionService_Start_WithPercent(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 80)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, testVehicleID, session.VehicleID)
	assert.Equal(t, "pending", session.Status)
	assert.Equal(t, 80.0, session.TargetPercent)
	assert.Equal(t, 30.0, session.StartPercent)
	assert.InDelta(t, 0.6078, session.StartKwh, 0.0001) // 30% of 2.026 (rm1 model)
}

func TestChargeSessionService_Start_WithPercent_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, "nonexistent", 30, 80)
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, ErrVehicleNotFound)
}

func TestChargeSessionService_Stop_WithPercent(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session with percent
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 80)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Stop the session with endPercent=80.0
	// No energy in ctrl - service falls back to the passed endPercent.
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)
	_, err = service.StopWithPercent(context.Background(), session.ID, 80.0)
	require.NoError(t, err)

	// Verify session is completed
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
	// EndPercent should equal the passed endPercent since no MQTT energy is cached
	require.NotNil(t, found.EndPercent)
	assert.Equal(t, float64(80), *found.EndPercent)
}

func TestChargeSessionService_Stop_WithPercent_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	result, err := service.StopWithPercent(context.Background(), "nonexistent", 80.0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, result)
}

func TestChargeSessionService_GetActive_WithPercent(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session
	session, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.Nil(t, session)

	// Create active session with percent
	pending, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 75)
	require.NoError(t, err)
	assert.NotNil(t, pending)
	assert.Equal(t, 75.0, pending.TargetPercent)
	assert.Equal(t, 30.0, pending.StartPercent)

	// Get active session
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, pending.ID, active.ID)
	assert.Equal(t, 75.0, active.TargetPercent)
}

func TestChargeSessionService_UpdateTarget_Valid(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Activate pending session
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Try to update target to below start (no MQTT energy → falls back to ErrTargetBelowStart)
	err = service.UpdateTarget(context.Background(), session.ID, 30.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "charge target must be higher than the starting battery level")
}

func TestChargeSessionService_UpdateTarget_CompletedSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start and complete a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	_, err = service.StopWithPercent(context.Background(), session.ID, 70.0)
	require.NoError(t, err)

	// Try to update target on completed session (should fail)
	err = service.UpdateTarget(context.Background(), session.ID, 80.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session is not active")
}

func TestChargeSessionService_UpdateTarget_NonExistent(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Try to update target on non-existent session
	err := service.UpdateTarget(context.Background(), "non-existent-session-id", 80.0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestChargeSessionService_UpdateTarget_OutOfRange(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Activate pending session so it stays active
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Try to update target to 150% (should fail)
	err = service.UpdateTarget(context.Background(), session.ID, 150.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "charge target must be between 0 and 100")

	// Try to update target to -10% (should fail)
	err = service.UpdateTarget(context.Background(), session.ID, -10.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "charge target must be between 0 and 100")
}

func TestChargeSessionService_CheckAndAutoStopReachingSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session - should not panic
	service.CheckAndAutoStopReachingSession(context.Background())

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	// Activate pending session
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Should not panic when checking session; no MQTT energy → skips auto-stop
	service.CheckAndAutoStopReachingSession(context.Background())

	// Verify session is still active (target not reached - no energy data)
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", found.Status)
}

func TestChargeSessionService_AutoStop_ReachesTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	// Seed initial energy so StartSession captures StartTotalKwh = 1.092
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.092, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Start a session at 30%, target 70%; StartTotalKwh will be captured as 1.092
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	// Activate pending session
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Simulate energy readings that exceed the target.
	// RM1 capacity: 2.026 kWh. 70% = 1.4182 kWh.
	// Need (Total - 1.092) * 0.8 + 0.6078 >= 1.4182
	// => Total >= 2.105; use 2.15 for comfortable margin.
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 2.15, Power: 600})

	// Trigger auto-stop check
	service.CheckAndAutoStopReachingSession(context.Background())

	// Session should be auto-completed
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
}

func TestChargeSessionService_GetLastCompleted(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No completed sessions
	session, err := service.GetLastCompleted(context.Background())
	require.NoError(t, err)
	assert.Nil(t, session)

	// Create and complete a session
	created, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), created.ID)
	require.NoError(t, err)
	_, err = service.StopWithPercent(context.Background(), created.ID, 70.0)
	require.NoError(t, err)

	// Get last completed session
	last, err := service.GetLastCompleted(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, last)
	assert.Equal(t, created.ID, last.ID)
	assert.Equal(t, "completed", last.Status)
}

func TestChargeSessionService_GetActive_WithPowerDraw(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.092, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Activate pending session to simulate real charging
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// GetActive should include power draw from MQTT cache
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
	assert.NotNil(t, active.PowerDraw)
	assert.Greater(t, *active.PowerDraw, 0.0)
}

func TestChargeSessionService_GetActive_ReturnsActiveTier(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session with RM1 (has Time20to80Min → static tier)
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Get active session
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
}

func TestChargeSessionService_GetActive_NoPowerDraw(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Get active session - should work even without MQTT energy
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, session.ID, active.ID)
}

func TestCalcBlendedKwh_InterpolatesWhenNoTick(t *testing.T) {
	// Simulate 30 seconds of charging
	startTime := time.Now().Add(-30 * time.Second)

	energy := &tasmota.EnergyData{
		Total:   1.092, // Same as start (no tick)
		Power:   25.0,
		Voltage: 230.0,
	}

	blendedKwh, hasTick := CalcBlendedKwh(0.6078, 1.092, energy, startTime, 0.8)

	assert.False(t, hasTick)
	// Without a tick, interpolation is no longer used (stateless function can't
	// track delta between polls). Returns startKwh unchanged.
	assert.InDelta(t, 0.6078, blendedKwh, 0.0001)
}

func TestCalcBlendedKwh_UsesRawWhenTicked(t *testing.T) {
	startTime := time.Now()

	energy := &tasmota.EnergyData{
		Total: 1.100, // 1.100 - 1.092 = 0.008 > epsilonKwh (0.002)
		Power: 45000.0,
	}

	blendedKwh, hasTick := CalcBlendedKwh(0.6078, 1.092, energy, startTime, 0.8)

	assert.True(t, hasTick)
	// Raw: 0.6078 + (1.100 - 1.092) * 0.8 = 0.6078 + 0.0064 = 0.6142
	assert.InDelta(t, 0.6142, blendedKwh, 0.0001)
}

func TestCalcBlendedKwh_ZeroPower(t *testing.T) {
	startTime := time.Now()

	energy := &tasmota.EnergyData{
		Total:   1.092,
		Power:   0,
		Voltage: 230.0,
	}

	blendedKwh, hasTick := CalcBlendedKwh(0.6078, 1.092, energy, startTime, 0.8)

	assert.False(t, hasTick)
	assert.Equal(t, 0.6078, blendedKwh)
}

// --- SOCGenerator tests ---.

func TestSOCGenerator_CalculateSOC(t *testing.T) {
	// stateless calculator
	gen := NewSOCGenerator()

	session := &models.ChargeSession{
		StartKwh:      0.6078, // 30% of 2.026
		StartTotalKwh: ptrFloat64(1.092),
		StartedAt:     ptrTime(time.Now()),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}

	soc, lastBlended, err := gen.CalculateSOC(session, energy, vehicle)
	require.NoError(t, err)
	// Raw delta: (1.100 - 1.092) / 0.8 = 0.01. Kwh = 0.6078 + 0.0064 = 0.6142
	// Percent = 0.6178 / 2.026 * 100 ≈ 30.32
	assert.Greater(t, soc, 30.0)
	assert.Less(t, soc, 31.0)
	assert.Greater(t, lastBlended, 0.0)
}

func TestSOCGenerator_CalculateSOC_ZeroCapacity(t *testing.T) {
	// stateless calculator
	gen := NewSOCGenerator()

	session := &models.ChargeSession{}
	vehicle := &models.Vehicle{CapacityKwh: 0}
	energy := &tasmota.EnergyData{Total: 1.0, Power: 100.0}

	_, _, err := gen.CalculateSOC(session, energy, vehicle)
	assert.Error(t, err)
}

func TestSOCGenerator_BuildSnapshot(t *testing.T) {
	// stateless calculator
	gen := NewSOCGenerator()

	sn := gen.BuildSnapshot("sess-1", 45.5)

	assert.Equal(t, "sess-1", sn.SessionID)
	assert.Equal(t, 45.5, sn.SocPercent)
	assert.NotEmpty(t, sn.ID)
	assert.True(t, sn.Timestamp.Before(time.Now()))
}

// --- ChargeSessionService.Stop (non-percent) tests ---.

func TestChargeSessionService_Stop(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Start a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 80)
	require.NoError(t, err)

	// Activate pending session so Stop doesn't cancel it
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Stop (uses energy-based endPercent calculation; falls back to target with nil ctrl)
	result, err := service.Stop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Stopped)

	// Verify session completed
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
}

func TestChargeSessionService_Stop_NoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	result, err := service.Stop(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, result)
}

func TestChargeSessionService_StoreSOCSnapshot(t *testing.T) {
	db := setupServiceTestDB(t)

	// Seed energy so StartSession captures StartTotalKwh = 1.092
	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.092, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Start a session - StartTotalKwh will be captured as 1.092 from MQTT cache
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 80)
	require.NoError(t, err)

	// Store SOC snapshot with higher Total to simulate energy accumulation
	energy := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}
	err = service.StoreSOCSnapshot(context.Background(), session, energy)
	require.NoError(t, err)

	// Verify snapshot was saved
	snapshots, err := repository.NewChargeSessionRepository(db).GetSOCSnapshots(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Len(t, snapshots, 1)
	assert.Greater(t, snapshots[0].SocPercent, 30.0)
}

func TestChargeSessionService_StoreSOCSnapshot_NoVehicle(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create a session with a nonexistent vehicle ID
	session := &models.ChargeSession{
		ID:            "fake-session",
		VehicleID:     "nonexistent",
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartKwh:      1.0,
		StartTotalKwh: ptrFloat64(1.0),
		StartedAt:     ptrTime(time.Now()),
	}

	err := service.StoreSOCSnapshot(context.Background(), session, &tasmota.EnergyData{})
	assert.NoError(t, err) // Silently skips when vehicle is missing
}

// --- EnergyCalculator.CalculateEndPercent test ---.

func TestEnergyCalculator_CalculateEndPercent(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078, // 30% of 2.026
		StartTotalKwh: ptrFloat64(1.092),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100}

	pct := CalculateEndPercent(session, energy, vehicle)
	// Wall delta: (1.100 - 1.092) = 0.008 kWh. Battery: 0.008/0.8 = 0.01 kWh
	// End Kwh: 0.6078 + 0.0064 = 0.6142. Percent: 0.6178/2.026*100 = 30.32%
	assert.InDelta(t, 30.324, pct, 0.01)
}

// --- ResolveChartSession test ---.

func TestResolveChartSession(t *testing.T) {
	db := setupServiceTestDB(t)

	repo := repository.NewChargeSessionRepository(db)

	// Verify no active session
	s, err := repo.ResolveChartSession(context.Background(), "", testVehicleID)
	require.NoError(t, err)
	assert.Nil(t, s)

	// Create a completed session - Create overwrites ID with generateID()
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		Status:        "completed",
		StartPercent:  30,
		TargetPercent: 70,
		CreatedAt:     time.Now().Add(-2 * time.Hour),
		EndedAt:       ptrTime(time.Now()),
	}
	err = repo.Create(context.Background(), session)
	require.NoError(t, err)
	require.NotEmpty(t, session.ID)

	// Verify it can be found
	found, err := repo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "completed", found.Status)

	// Query by session ID - should return that specific session
	s, err = repo.ResolveChartSession(context.Background(), session.ID, "")
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, session.ID, s.ID)
}

// Helper: pointer utilities.
func ptrInt(i int) *int             { return &i }
func ptrFloat64(f float64) *float64 { return &f }
func ptrTime(t time.Time) *time.Time { return &t }

// --- Coverage tests for previously uncovered paths ---.

func TestChargeSessionService_GetActive_TasmotaError(t *testing.T) {
	db := setupServiceTestDB(t)

	// Seed an active session directly without plug_id - no energy lookup will occur
	insertActiveSession(t, db, "tasmota-error-session", testVehicleID, 0.5, 1.5, 20.0, 80.0, ptrFloat64(1.092), ptrTime(time.Now()))

	// nil plugCtrl - no MQTT energy cache; same effect as an unreachable device
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// GetActive should return session but without PowerDraw/CurrentPercent
	// because no MQTT energy is available
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
	assert.Nil(t, active.PowerDraw)
	assert.Nil(t, active.CurrentPercent)
}

func TestChargeSessionService_GetActive_ZeroPower(t *testing.T) {
	db := setupServiceTestDB(t)

	// Seed an active session directly without plug_id
	insertActiveSession(t, db, "zero-power-session", testVehicleID, 0.5, 1.5, 20.0, 80.0, ptrFloat64(1.092), ptrTime(time.Now()))

	// nil plugCtrl - session has no PlugID, so no energy lookup regardless
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// GetActive returns session but no CurrentPercent because no energy data
	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, active)
	assert.Nil(t, active.CurrentPercent)
}

func TestChargeSessionService_Stop_FallbackToTargetPercent(t *testing.T) {
	db := setupServiceTestDB(t)

	// Seed an active session with target 85% and no start_total_kwh
	insertActiveSession(t, db, "fallback-session", testVehicleID, 0.6, 1.6, 30.0, 85.0, nil, ptrTime(time.Now()))

	// nil plugCtrl - same effect as unreachable device (no energy → fallback to TargetPercent)
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	_, err := service.Stop(context.Background())
	require.NoError(t, err)

	found, err := service.sessionReader.FindByID(context.Background(), "fallback-session")
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
	require.NotNil(t, found.EndPercent)
	assert.Greater(t, *found.EndPercent, 0.0)
}

func TestChargeSessionService_UpdateTarget_NoTasmotaFallback(t *testing.T) {
	db := setupServiceTestDB(t)

	// Seed an active session at 30%
	insertActiveSession(t, db, "notasmota-fallback-session", testVehicleID, 0.6, 1.6, 30.0, 70.0, ptrFloat64(1.092), ptrTime(time.Now()))

	// nil plugCtrl - no energy → falls back to startPercent for current level check
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// UpdateTarget with value below start percent should fail (no-energy path)
	err := service.UpdateTarget(context.Background(), "notasmota-fallback-session", 20.0)

	assert.Contains(t, err.Error(), "charge target must be higher than the starting battery level")
}

func TestChargeSessionService_StoreSOCSnapshot_ZeroCapacityVehicle(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Insert a vehicle model+instance with CapacityKwh=0 directly into DB
	insertRawVehicle(t, db, "zero-cap", 0, 0, 0)

	session := &models.ChargeSession{
		ID:            "zero-cap-session",
		VehicleID:     "zero-cap",
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartKwh:      0.0,
		StartTotalKwh: ptrFloat64(1.0),
		StartedAt:     ptrTime(time.Now()),
	}

	err := service.StoreSOCSnapshot(context.Background(), session, &tasmota.EnergyData{})
	assert.NoError(t, err) // Silently skips when CapacityKwh <= 0

	// Verify no snapshot was created
	snapshots, err := repository.NewChargeSessionRepository(db).GetSOCSnapshots(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Empty(t, snapshots)
}

func TestEnergyCalculator_CalculateEndPercent_SensorDrift(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078, // ~30% of 2.026
		StartTotalKwh: ptrFloat64(2.0),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	// energy.Total < startTotalKwh simulates sensor drift/reset
	energy := &tasmota.EnergyData{Total: 1.0}

	pct := CalculateEndPercent(session, energy, vehicle)
	// Wall delta: (1.0 - 2.0) = -1.0 kWh. Battery: -1.0/0.8 = -1.25 kWh
	// End Kwh: 0.6078 - 1.25 = -0.6422. Percent: -31.70% → clamped to 0.0
	assert.Equal(t, 0.0, pct)
}

func TestChargeSessionService_CheckAndAutoStopReachingSession_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session - should return cleanly without panic
	assert.NotPanics(t, func() {
		service.CheckAndAutoStopReachingSession(context.Background())
	})
}

func TestChargeSessionService_CheckAndAutoStopReachingSession_TasmotaError(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// failService uses nil plugCtrl - LastEnergy returns nil (equivalent to unreachable device)
	failService := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer failService.Shutdown()

	assert.NotPanics(t, func() {
		failService.CheckAndAutoStopReachingSession(context.Background())
	})

	// Session should still be active (auto-stop didn't happen - no energy data)
	active, err := failService.GetActive(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, "active", active.Status)
}

func TestChargeSessionService_StartSession_TasmotaFailure(t *testing.T) {
	db := setupServiceTestDB(t)

	// ctrl.setPowerErr causes SetPower to fail - equivalent to unreachable Tasmota
	ctrl := newMockPlugCtrl()
	ctrl.setPowerErr = errors.New("connection refused")
	failService := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer failService.Shutdown()

	_, err := failService.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "power confirmation failed")
}

func TestChargeSessionService_StartSession_GetEnergyError(t *testing.T) {
	db := setupServiceTestDB(t)

	// ctrl with no energy - SetPower succeeds, captureEnergyBaseline returns nil
	ctrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Should succeed - SetPower works, no MQTT energy cached yet → StartTotalKwh is nil
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Nil(t, session.StartTotalKwh) // energy was unavailable from MQTT cache
}

func TestChargeSessionService_StartSession_DBFailureDoesNotTurnOnOutlet(t *testing.T) {
	// This test verifies that the DB write happens BEFORE the plug power-on.
	// If the DB validation fails, the plug should NOT be turned on.
	ctrl := newMockPlugCtrl()

	// Set up a working DB but close it before calling StartSession
	// so that vehicleRepo validation fails
	db := setupServiceTestDB(t)
	db.Close() // Close DB - validateVehicleExists will fail

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	assert.Error(t, err) // DB validation should fail
	assert.False(t, ctrl.powerOn[testPlugID], "plug outlet should NOT be turned on when DB validation fails")
}

func TestChargeSessionService_CheckAndAutoStop_ReachesTarget_StopError(t *testing.T) {
	db := setupServiceTestDB(t)

	// Create an active session without plug_id - no energy lookup occurs
	startTime := time.Now().Add(-time.Hour)
	insertActiveSession(t, db, "session-auto-stop", testVehicleID, 0.38, 1.9, 20, 50, ptrFloat64(1000.0), &startTime)

	// nil plugCtrl - no energy → auto-stop won't trigger
	failService := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer failService.Shutdown()

	// Should not panic - just returns early due to missing energy
	assert.NotPanics(t, func() {
		failService.CheckAndAutoStopReachingSession(context.Background())
	})
}

func TestChargeSessionService_StoreSOCSnapshot_DBError(t *testing.T) {
	db := setupServiceTestDB(t)

	// Create a session
	startTime := time.Now()
	insertActiveSession(t, db, "session-soc-error", testVehicleID, 0.38, 1.9, 20, 80, ptrFloat64(1000.0), &startTime)

	// Find the session
	session, err := repository.NewChargeSessionRepository(db).FindByID(context.Background(), "session-soc-error")
	require.NoError(t, err)
	require.NotNil(t, session)

	// Close the DB to force a DB error
	db.Close()

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// StoreSOCSnapshot should fail with DB error
	energy := &tasmota.EnergyData{Total: 1500.0, Power: 600}
	err = service.StoreSOCSnapshot(context.Background(), session, energy)
	assert.Error(t, err)
}

func TestChargeSessionService_UpdateTarget_DBError(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create an active session (needs open DB)
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Close the DB to force a DB error
	db.Close()

	// UpdateTarget should fail with DB error when FindByID is called
	err = service.UpdateTarget(context.Background(), session.ID, 80.0)
	assert.Error(t, err)
}

func TestChargeSessionService_StoreSOCSnapshot_VehicleRepoDBError(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create an active session (needs open DB)
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Close the DB to force a VehicleRepo DB error
	db.Close()

	energy := &tasmota.EnergyData{Total: 1500.0, Power: 600}
	err = service.StoreSOCSnapshot(context.Background(), session, energy)
	assert.Error(t, err)
}

func TestChargeSessionService_ConstructorInjectPushService(t *testing.T) {
	db := setupServiceTestDB(t)

	// Without PushService
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()
	assert.Nil(t, service.notifier.pushService)

	// With PushService
	ps := NewPushService(&mockPushRepo{}, "pub", "priv", nil)
	serviceWithPush := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, ps)
	defer serviceWithPush.Shutdown()
	assert.NotNil(t, serviceWithPush.notifier.pushService)
	assert.Equal(t, ps, serviceWithPush.notifier.pushService)
}

func TestBuildNotificationBody_WithRangeModes(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  30,
		TargetPercent: 80,
	}

	vehicle, _ := service.vehicleRepo.FindByID(context.Background(), session.VehicleID)
	body := service.notifier.buildNotificationBody("RM2", 80.0, vehicle)
	assert.Contains(t, body, "RM2 Charge Complete")
	assert.Contains(t, body, "80%")
	assert.Contains(t, body, "~")
	assert.Contains(t, body, "mi")
}

func TestBuildNotificationBody_NoRangeModes(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	body := service.notifier.buildNotificationBody("TestVehicle", 80.0, nil)
	assert.Equal(t, "TestVehicle reached 80%", body)
}

func TestBuildNotificationBody_CurrentEqualsTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create a vehicle model+instance with equal min/max range so notification shows single value
	insertRawVehicle(t, db, "single-vehicle", 5.0, 100, 100)

	vehicle, err := service.vehicleRepo.FindByID(context.Background(), "single-vehicle")
	require.NoError(t, err)

	body := service.notifier.buildNotificationBody("EV", 50.0, vehicle)
	assert.Contains(t, body, "EV Charge Complete")
	assert.Contains(t, body, "50%")
	assert.Contains(t, body, "~")
	assert.Contains(t, body, "50mi")
	// Should NOT contain a dash (no range, single value)
	assert.NotContains(t, body, "-")
}

func TestChargeSessionService_Stop_SendsPushNotification(t *testing.T) {
	db := setupServiceTestDB(t)

	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/test",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	callCount := 0
	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK"}, nil
		},
	}
	ps := NewPushService(repo, "pub", "priv", client)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, ps)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	_, err = service.StopWithPercent(context.Background(), session.ID, 70.0)
	require.NoError(t, err)

	// Drain the async push notification goroutine; Shutdown waits on the
	// notifier's WaitGroup, establishing happens-before for the callCount read.
	service.Shutdown()

	assert.Greater(t, callCount, 0, "push notification should have been sent")
}

func TestChargeSessionService_Stop_CalculateEndPercentFallback(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Activate pending session
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// nil plugCtrl → no energy → StopWithPercent uses passed endPercent as fallback
	result, err := service.StopWithPercent(context.Background(), session.ID, 70.0)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)

	// Verify the session was marked completed with the fallback endPercent
	vehicle, err := service.vehicleRepo.FindByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	assert.NotNil(t, vehicle)
	assert.InDelta(t, 70.0, vehicle.CurrentPercent, 0.5, "should fall back to passed endPercent when energy is unavailable")
}

func TestCalcBlendedKwh_Interpolation_DoesNotCompound(t *testing.T) {
	// Simulates RM2 charging from 20% to 80% at ~1510W.
	// With no tick on Tasmota's cumulative counter, CalcBlendedKwh returns
	// startKwh unchanged - phantom energy must NOT compound across polls.
	// RM2: CapacityKwh=5.46, T20to80=150min
	startKwh := 5.46 * 0.20 // 1.092 kWh (20% of 5.46)
	startTotalKwh := 100.0   // arbitrary baseline
	powerW := 1510.0         // mock-tasmota POWER_WATTS
	efficiency := 0.8

	startTime := time.Now()

	// Poll 1: immediately after start (0 elapsed)
	energy1 := &tasmota.EnergyData{Total: startTotalKwh, Power: powerW}
	kwh1, _ := CalcBlendedKwh(startKwh, startTotalKwh, energy1, startTime, efficiency)
	assert.InDelta(t, startKwh, kwh1, 0.0001, "poll 1 (0 elapsed) should return startKwh")

	// Poll 2: no tick (Total unchanged)
	energy2 := &tasmota.EnergyData{Total: startTotalKwh, Power: powerW}
	kwh2, _ := CalcBlendedKwh(startKwh, startTotalKwh, energy2, startTime, efficiency)

	// Poll 3: still no tick
	energy3 := &tasmota.EnergyData{Total: startTotalKwh, Power: powerW}
	kwh3, _ := CalcBlendedKwh(startKwh, startTotalKwh, energy3, startTime, efficiency)

	// Calculate what the SOC would be at each poll
	// stateless calculator
	vehicle := &models.Vehicle{
		ID:                 "rm2",
		CapacityKwh:        5.46,
		ChargerOutputW:     1200,
		ChargingEfficiency: 0.8,
		Time20to80Min:      ptrInt(150),
	}

	pct1 := CalculateCurrentPercent(
		&models.ChargeSession{StartKwh: startKwh, StartTotalKwh: &startTotalKwh, StartedAt: &startTime},
		energy1, vehicle,
	)
	pct2 := CalculateCurrentPercent(
		&models.ChargeSession{StartKwh: startKwh, StartTotalKwh: &startTotalKwh, StartedAt: &startTime},
		energy2, vehicle,
	)
	pct3 := CalculateCurrentPercent(
		&models.ChargeSession{StartKwh: startKwh, StartTotalKwh: &startTotalKwh, StartedAt: &startTime},
		energy3, vehicle,
	)

	// With no tick, all three polls return startKwh → SOC stays at startPercent (20%)
	assert.InDelta(t, 20.0, pct1, 0.5)
	assert.Less(t, pct2, 21.0, "SOC jumped too much after poll 2: got %.2f%%, expected < 21%%", pct2)
	assert.Less(t, pct3, 21.5, "SOC jumped too much after poll 3: got %.2f%%, expected < 21.5%%", pct3)

	// Each poll returns startKwh when no tick - deltas are zero, no compounding
	tickDelta2 := kwh2 - startKwh
	tickDelta3 := kwh3 - startKwh
	assert.LessOrEqual(t, tickDelta2, tickDelta3, "interpolation should not regress")
	assert.Less(t, tickDelta3-tickDelta2, 0.005, "interpolation increment per poll should be small")
}

func TestChargeSessionService_CancelPending(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "pending", session.Status)

	err = service.CancelPending(context.Background(), session.ID)
	require.NoError(t, err)

	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", found.Status)
	require.NotNil(t, found.EndedAt)
}

func TestChargeSessionService_CancelPending_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	err := service.CancelPending(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestChargeSessionService_CancelPending_AlreadyActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.CancelPending(context.Background(), session.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound) // Not found for non-pending sessions
}

func TestChargeSessionService_GetPending(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	pending, err := service.GetPending(context.Background())
	require.NoError(t, err)
	assert.Nil(t, pending)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	pending, err = service.GetPending(context.Background())
	require.NoError(t, err)
	require.NotNil(t, pending)
	assert.Equal(t, session.ID, pending.ID)
	assert.Equal(t, "pending", pending.Status)
}

func TestChargeSessionService_DeleteSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)
	_, err = service.StopWithPercent(context.Background(), session.ID, 70.0)
	require.NoError(t, err)

	err = service.DeleteSession(context.Background(), session.ID)
	require.NoError(t, err)

	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestChargeSessionService_DeleteSession_Active(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.DeleteSession(context.Background(), session.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotDeleteActiveSession)
}

func TestChargeSessionService_DeleteSession_Pending(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	err = service.DeleteSession(context.Background(), session.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotDeleteActiveSession)
}

func TestChargeSessionService_DeleteSession_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	err := service.DeleteSession(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestChargeSessionService_Stop_CancelPendingSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	assert.Equal(t, "pending", session.Status)

	result, err := service.Stop(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)

	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", found.Status)
}

func TestChargeSessionService_StartSession_FailsWhenActiveExists(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	// A second session on the same plug must be rejected.
	_, err = service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrActiveSessionExists)
}

func TestChargeSessionService_StartSession_SucceedsWhenNoActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
}

func TestChargeSessionService_CheckAndAutoStop_PendingSessionSkipped(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	service.CheckAndAutoStopReachingSession(context.Background())

	active, err := service.sessionReader.GetActive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pending", active.Status)
}

func TestChargeSessionService_CheckAndAutoStop_NotYetAtTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	err = repository.NewChargeSessionRepository(db).UpdateStatus(context.Background(), session.ID, "active")
	require.NoError(t, err)

	// No MQTT energy in ctrl → auto-stop skips (StartTotalKwh=nil from StartSession without energy)
	service.CheckAndAutoStopReachingSession(context.Background())

	active, err := service.sessionReader.GetActive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "active", active.Status)
}

func TestChargeSessionService_CheckAndAutoStop_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	// Vehicle with zero capacity - would fail auto-stop calculation if reached
	insertRawVehicle(t, db, "v1", 0, 0, 0)

	insertSession(t, db, "test-session", "v1", "active", 0.57, 1.33, 30, 70, ptrFloat64(1.0))

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No PlugID in session insert → no energy lookup → auto-stop skips
	service.CheckAndAutoStopReachingSession(context.Background())

	active, err := service.sessionReader.GetActive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "active", active.Status)
}

func TestChargeSessionService_StartSession_NilEnergyBaselineWhenNoPlug(t *testing.T) {
	db := setupServiceTestDB(t)

	// nil plugCtrl - equivalent of unreachable MQTT; StartTotalKwh must remain nil.
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	assert.Nil(t, session.StartTotalKwh, "StartTotalKwh must be nil when plug controller is unavailable")
}

func TestChargeSessionService_GetLastCompleted_NoCompletedSessions(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	last, err := service.GetLastCompleted(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, last)
}

func TestChargeSessionService_GetLastCompleted_WithCancelledOnly(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	insertRawVehicle(t, db, "v1", 1.9, 0, 0)

	insertSession(t, db, "cancelled-session", "v1", "cancelled", 0.38, 1.52, 20, 80, nil)

	last, err := service.GetLastCompleted(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, last)
}

func TestChargeSessionService_StartSession_ActiveSessionExists(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	_, err = service.StartSession(context.Background(), testPlugID, testVehicleID, 40, 80)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrActiveSessionExists)
}

func TestChargeSessionService_StartSession_PendingSessionExists(t *testing.T) {
	db := setupServiceTestDB(t)

	repo := repository.NewChargeSessionRepository(db)
	// Insert a pending session directly so the relay is never turned on
	insertSession(t, db, "pending-test", testVehicleID, "pending", 0, 0, 30, 70, nil)

	service := NewChargeSessionService(context.Background(), repo, repository.NewVehicleRepository(db), nil, nil, nil, nil)

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 40, 80)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrActiveSessionExists)
}

func TestChargeSessionService_StopWithPercent_SessionNotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	_, err := service.StopWithPercent(context.Background(), "nonexistent", 50)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestChargeSessionService_StopWithPercent_PendingSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	assert.Equal(t, "pending", session.Status)

	result, err := service.StopWithPercent(context.Background(), session.ID, 50)
	require.NoError(t, err)
	assert.True(t, result.Stopped)

	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", found.Status)
}

func TestChargeSessionService_ActivatePending_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	_, err := service.ActivatePending(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestChargeSessionService_ActivatePending_AlreadyActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	err = repository.NewChargeSessionRepository(db).UpdateStatus(context.Background(), session.ID, "active")
	require.NoError(t, err)

	total, err := service.ActivatePending(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), total)

	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", found.Status)
}

func TestCalcBlendedKwh_SessionNil(t *testing.T) {
	energy := &tasmota.EnergyData{Total: 2.0, Power: 500}
	startTime := time.Now().Add(-time.Minute)

	blendedKwh, hasTick := CalcBlendedKwh(0.6078, 1.0, energy, startTime, 0.8)
	assert.True(t, hasTick)
	assert.Greater(t, blendedKwh, 0.6)
}

func TestChargeSessionService_UpdateTarget_LessThanCurrentPercent(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	insertRawVehicle(t, db, "v1", 1.9, 0, 0)

	insertSession(t, db, "active-session", "v1", "active", 0.57, 1.33, 30, 70, ptrFloat64(1.0))

	// No MQTT energy (no PlugID, nil ctrl) → target(25) <= startPercent(30) → error
	err := service.UpdateTarget(context.Background(), "active-session", 25)
	assert.Error(t, err)
}

func TestChargeSessionService_UpdateTarget_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	// Vehicle with zero capacity - causes ErrVehicleConfigMissing in UpdateTarget
	insertRawVehicle(t, db, "v1", 0, 0, 0)

	// Session with plug_id so MQTT energy lookup occurs
	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.5, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	insertSession(t, db, "active-session", "v1", "active", 0.57, 1.33, 30, 70, ptrFloat64(1.0))

	err := service.UpdateTarget(context.Background(), "active-session", 80)
	assert.Error(t, err) // ErrVehicleConfigMissing - v1 has CapacityKwh=0
}

// orderingTasmotaMock captures the session status at the moment SetPower is called.
// Used to verify that DB is updated before power is turned off.
type orderingTasmotaMock struct {
	energy           *tasmota.EnergyData
	db               *sql.DB
	sessionID        string
	statusAtPowerOff string
}

func (m *orderingTasmotaMock) LastEnergy(_ string) *tasmota.EnergyData {
	return m.energy
}

func (m *orderingTasmotaMock) SetPower(_ context.Context, _ string, on bool) error {
	if !on {
		var status string
		err := m.db.QueryRow(`SELECT status FROM charge_sessions WHERE id = ?`, m.sessionID).Scan(&status)
		if err == nil {
			m.statusAtPowerOff = status
		}
	}
	return nil
}

func (m *orderingTasmotaMock) SetPowerAndWait(_ context.Context, _ string, on bool, _ time.Duration) (bool, error) {
	if !on {
		var status string
		err := m.db.QueryRow(`SELECT status FROM charge_sessions WHERE id = ?`, m.sessionID).Scan(&status)
		if err == nil {
			m.statusAtPowerOff = status
		}
	}
	return true, nil
}

func TestChargeSessionService_Stop_DBBeforePowerOff(t *testing.T) {
	db := setupServiceTestDB(t)

	insertRawVehicle(t, db, "v1", 1.9, 0, 0)

	sessionID := "stop-ordering-test"
	insertSession(t, db, sessionID, "v1", "active", 0.57, 1.33, 30, 70, ptrFloat64(1.0))

	mock := &orderingTasmotaMock{
		db:        db,
		sessionID: sessionID,
		energy: &tasmota.EnergyData{
			Total:   2.0,
			Power:   0,
			Voltage: 230,
		},
	}

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, mock, nil, nil)
	defer service.Shutdown()

	result, err := service.StopWithPercent(context.Background(), sessionID, 60.0)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)

	assert.Equal(t, "completed", mock.statusAtPowerOff,
		"session must be marked completed in DB before plug power is turned off")
}

func TestChargeSessionService_CancelPending_DBBeforePowerOff(t *testing.T) {
	db := setupServiceTestDB(t)

	insertRawVehicle(t, db, "v1", 1.9, 0, 0)

	sessionID := "cancel-ordering-test"
	insertSession(t, db, sessionID, "v1", "pending", 0.57, 1.33, 30, 70, nil)

	mock := &orderingTasmotaMock{
		db:        db,
		sessionID: sessionID,
	}

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, mock, nil, nil)
	defer service.Shutdown()

	result, err := service.StopWithPercent(context.Background(), sessionID, 30.0)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)

	assert.Equal(t, "cancelled", mock.statusAtPowerOff,
		"pending session must be marked cancelled in DB before plug power is turned off")
}

func TestChargeSessionService_Shutdown(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Shutdown should complete without hanging (channel closed, worker drains and exits)
	done := make(chan struct{})
	go func() {
		service.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Success: shutdown completed
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out - SOC worker may have leaked")
	}
}

func TestChargeSessionService_StartSession_TasmotaFailure_NoOrphanedSession(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()

	insertRawVehicle(t, db, "v1", 1.9, 0, 0)

	// ctrl with setPowerErr - SetPower fails, session must not be created
	ctrl := newMockPlugCtrl()
	ctrl.setPowerErr = errors.New("connection refused")

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	_, err := service.StartSession(context.Background(), testPlugID, "v1", 20, 80)
	assert.Error(t, err, "StartSession should fail when plug rejects power-on")

	// Verify no orphaned session was created in the database
	repo := repository.NewChargeSessionRepository(db)
	active, err := repo.GetActive(context.Background())
	require.NoError(t, err)
	assert.Nil(t, active, "no session should exist in DB when plug power-on fails")
}

func TestChargeSessionService_Shutdown_DrainsPendingRequests(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()

	insertRawVehicle(t, db, "v1", 1.9, 0, 0)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Queue some SOC requests (they will be processed by the worker)
	energy := &tasmota.EnergyData{
		Total:   100.0,
		Power:   600,
		Voltage: 230,
		Current: 2.6,
	}

	// Send a request that will be processed
	service.socWorker.Send(socRequest{
		sessionID:     "test-session",
		vehicleID:     "v1",
		startKwh:      0.38,
		startTotalKwh: 99.0,
		targetKwh:     1.9,
		createdAt:     time.Now(),
		energy:        energy,
	})

	// Shutdown should drain the channel before completing
	done := make(chan struct{})
	go func() {
		service.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out - pending SOC requests may not have been drained")
	}
}

func TestChargeSessionService_SetPlugController(t *testing.T) {
	db := setupServiceTestDB(t)

	mockCtrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	service.SetPlugController(mockCtrl)

	// Create a session so monitoring can look up energy by plug
	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	energy := &tasmota.EnergyData{Total: 100, Power: 600}
	mockCtrl.SetEnergy(testPlugID, energy)

	got, err := service.GetEnergy(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 100.0, got.Total)
}

func TestChargeSessionService_GetEnergy_NoController(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	energy, err := service.GetEnergy(context.Background())
	require.NoError(t, err)
	assert.Nil(t, energy)
}

func TestChargeSessionService_SetPowerState(t *testing.T) {
	db := setupServiceTestDB(t)

	mockCtrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()
	service.SetPlugController(mockCtrl)

	// Create a session so SetPowerState has context
	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	err = service.SetPowerState(context.Background(), true)
	require.NoError(t, err)

	mockCtrl.mu.RLock()
	assert.True(t, mockCtrl.powerOn[testPlugID])
	mockCtrl.mu.RUnlock()
}

func TestChargeSessionService_GetActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	session, err := service.GetActiveSession(context.Background())
	require.NoError(t, err)
	assert.Nil(t, session)

	_, err = service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	session, err = service.GetActiveSession(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, session)
}

func TestChargeSessionService_FindVehicleByID(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	vehicle, err := service.FindVehicleByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	require.NotNil(t, vehicle)
	assert.Equal(t, testVehicleID, vehicle.ID)

	notFound, err := service.FindVehicleByID(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestChargeSessionService_UpdateActiveTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session
	err := service.UpdateActiveTarget(context.Background(), 90)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)

	// Create and activate session
	pending, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), pending.ID)
	require.NoError(t, err)

	err = service.UpdateActiveTarget(context.Background(), 90)
	require.NoError(t, err)

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, 90.0, active.TargetPercent)
}

func TestChargeSessionService_CancelActiveSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	pending, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	active, err := service.GetActiveSession(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, "pending", active.Status)

	err = service.CancelActiveSession(context.Background(), active)
	require.NoError(t, err)

	// After cancel, GetActiveSession returns nil since cancelled is not an active status
	updated, err := service.GetActiveSession(context.Background())
	require.NoError(t, err)
	assert.Nil(t, updated)

	// Verify session was cancelled via repo
	sessionRepo := repository.NewChargeSessionRepository(db)
	cancelled, err := sessionRepo.FindByID(context.Background(), pending.ID)
	require.NoError(t, err)
	require.NotNil(t, cancelled)
	assert.Equal(t, "cancelled", cancelled.Status)
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session - should be a no-op
	service.CheckAndCancelDisconnectedSession(context.Background())
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_NoEnergy(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create active session but no energy data (LastBlendedKwh is nil)
	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	service.CheckAndCancelDisconnectedSession(context.Background())

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, "pending", active.Status)
}

func TestChargeSessionService_HandleSensorMessage_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	energy := &tasmota.EnergyData{Total: 100, Power: 600}
	service.HandleSensorMessage(context.Background(), testPlugID, energy)
	// Should be a no-op when no active session
}

func TestChargeSessionService_SaveEnergyReadings_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	energy := &tasmota.EnergyData{Total: 100, Power: 600}
	service.SaveEnergyReadings(context.Background(), testPlugID, energy)
	// Should be a no-op when no active session
}

func TestChargeSessionService_CheckAndStopConditioningSession_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	service.CheckAndStopConditioningSession(context.Background())
	// Should be a no-op when no active session
}

func TestChargeSessionService_CheckAndStopIdleSession_NoActive(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	service.CheckAndStopIdleSession(context.Background())
	// Should be a no-op when no active session
}

func TestChargeSessionService_CheckAndStopIdleSession_TasmotaError(t *testing.T) {
	db := setupServiceTestDB(t)

	// ctrl.setPowerErr causes the power-off confirmation to fail - equivalent
	// to an unreachable Tasmota device at stop time.
	ctrl := newMockPlugCtrl()
	ctrl.setPowerErr = errors.New("connection refused")
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	startTime := time.Now().Add(-30 * time.Minute)
	insertActiveSession(t, db, "session-idle-tasmota-err", testVehicleID, 1.62, 2.03, 80, 100, ptrFloat64(1.7), &startTime)

	sessRepo := repository.NewChargeSessionRepository(db)
	now := time.Now()
	seedIdlePowerReadings(t, sessRepo, "session-idle-tasmota-err", now.Add(-20*time.Minute), now, 5)

	assert.NotPanics(t, func() {
		service.CheckAndStopIdleSession(context.Background())
	})

	// The DB write (session end + stats) happens regardless of whether the
	// plug's power-off is confirmed - only the Tasmota confirmation failed.
	completed, err := sessRepo.FindByID(context.Background(), "session-idle-tasmota-err")
	require.NoError(t, err)
	require.NotNil(t, completed)
	assert.Equal(t, models.SessionStatusCompleted, completed.Status)
}

func TestChargeSessionService_CancelPendingIfTimedOut_NoPending(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	canceled, err := service.CancelPendingIfTimedOut(context.Background(), 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, canceled)
}

func TestChargeSessionService_NotifyPlugUnavailable(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// With nil push service, should be a no-op
	service.NotifyPlugUnavailable(context.Background(), &models.Plug{Name: "Test Plug", Type: models.PlugTypeCharging})
}

func TestChargeSessionService_GetActive_PendingSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create pending session - GetActive skips energy calculation for pending
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, session.ID, active.ID)
	assert.Equal(t, "pending", active.Status)
	// Pending sessions should have no energy data
	assert.Nil(t, active.PowerDraw)
	assert.Nil(t, active.CurrentPercent)
	assert.Nil(t, active.EnergyAddedKwh)
}

func TestChargeSessionService_GetActive_WithEnergy(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 2.5, Power: 600, Voltage: 230, Current: 2.6})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Create and activate session - StartTotalKwh captured from ctrl = 2.5
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Update energy to simulate charging progress
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 3.0, Power: 600, Voltage: 230, Current: 2.6})

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, "active", active.Status)
	// Should have energy data from plug controller
	assert.NotNil(t, active.PowerDraw)
	assert.Greater(t, *active.PowerDraw, 0.0)
	assert.NotNil(t, active.Voltage)
	assert.NotNil(t, active.Current)
	// Should have energy added and current percent from calculation
	assert.NotNil(t, active.EnergyAddedKwh)
	assert.Greater(t, *active.EnergyAddedKwh, 0.0)
	assert.NotNil(t, active.CurrentPercent)
}

func TestChargeSessionService_GetActive_HoldingCarbonAware_AttachesEstimatedResumeTime(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewChargeSessionService(context.Background(), sessRepo, repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()
	service.SetCarbonAwareForecaster(&mockForecaster{})

	// readyBy is only 1 minute away - any realistic estimated duration puts
	// latestStart well before now, forcing the deadline-guard branch so the
	// estimate doesn't depend on the exact charge-duration math.
	mockNow := time.Date(2024, 1, 1, 23, 45, 0, 0, time.UTC)
	old := scheduleNowFunc
	scheduleNowFunc = func() time.Time { return mockNow }
	t.Cleanup(func() { scheduleNowFunc = old })

	holdPercent := 64.0
	readyByTime := "23:46"
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

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	require.NotNil(t, active.EstimatedResumeTime, "holding carbon-aware session should expose an estimated resume time")
	assert.Regexp(t, `^([01]\d|2[0-3]):[0-5]\d$`, *active.EstimatedResumeTime)
}

func TestChargeSessionService_GetActive_HoldingDailyOrigin_NoEstimatedResumeTime(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewChargeSessionService(context.Background(), sessRepo, repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()
	service.SetCarbonAwareForecaster(&mockForecaster{})

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

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Nil(t, active.EstimatedResumeTime, "daily-origin holds have no forecast-based resume estimate")
}

func TestChargeSessionService_GetActive_VehicleConfigMissing(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 2.0, Power: 600, Voltage: 230, Current: 2.6})

	// Insert vehicle with zero capacity
	insertRawVehicle(t, db, "zero-vehicle", 0, 0, 0)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Battery-less vehicles cannot start sessions (ErrVehicleHasNoBattery), but
	// a session may pre-date a vehicle's config being lost - insert one directly
	// to exercise the degraded view path.
	startTotalKwh := 2.0
	insertActiveSession(t, db, "zero-session", "zero-vehicle", 0, 0, 30, 70, &startTotalKwh, nil)

	active, err := service.GetActive(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)
	// Should have power draw from plug controller
	assert.NotNil(t, active.PowerDraw)
	// But no energy calculation since vehicle config is missing
	assert.Nil(t, active.EnergyAddedKwh)
	assert.Nil(t, active.CurrentPercent)
}

func TestChargeSessionService_Stop_DBError(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate a session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Close DB to force error
	db.Close()

	// Stop should fail because GetActive can't read from closed DB
	result, err := service.Stop(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_NoSession(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// No active session - should return without error or panic
	assert.NotPanics(t, func() {
		service.CheckAndCancelDisconnectedSession(context.Background())
	})
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_PendingStatus(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create pending session - not active/conditioning, should not cancel
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	service.CheckAndCancelDisconnectedSession(context.Background())

	// Session should still be pending
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", found.Status)
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_NoBlendedKwh(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate session - LastBlendedKwh will be nil
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Verify LastBlendedKwh is nil
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Nil(t, found.LastBlendedKwh)

	service.CheckAndCancelDisconnectedSession(context.Background())

	// Session should still be active - no blended kWh means it never charged
	found, err = service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", found.Status)
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_Success(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Set LastBlendedKwh to a value > epsilonKwh (0.002) to simulate charging occurred
	blendedKwh := 0.5
	repo := repository.NewChargeSessionRepository(db)
	err = repo.UpdateLastBlendedKwh(context.Background(), session.ID, blendedKwh)
	require.NoError(t, err)

	// Verify LastBlendedKwh is set
	found, err := repo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found.LastBlendedKwh)
	assert.Greater(t, *found.LastBlendedKwh, epsilonKwh)

	service.CheckAndCancelDisconnectedSession(context.Background())

	// Session should be cancelled
	found, err = repo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", found.Status)
}

func TestChargeSessionService_HandleSensorMessage_SessionExists(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.092, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	energy := &tasmota.EnergyData{Total: 1.5, Power: 600, Voltage: 230, Current: 2.6}
	// Should not panic and should process the energy reading
	assert.NotPanics(t, func() {
		service.HandleSensorMessage(context.Background(), testPlugID, energy)
	})

	// Verify the session is still active (not auto-stopped by this call)
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", found.Status)
}

func TestChargeSessionService_HandleSensorMessage_ErrorPath(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Close DB to force error in GetActiveByPlug
	db.Close()

	energy := &tasmota.EnergyData{Total: 1.5, Power: 600}
	// Should not panic even with DB error
	assert.NotPanics(t, func() {
		service.HandleSensorMessage(context.Background(), testPlugID, energy)
	})
}

func TestChargeSessionService_ProcessSOC_Success(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create a session in the DB so UpdateLastBlendedKwh doesn't fail
	startTime := time.Now()
	insertSession(t, db, "process-soc-session", testVehicleID, "active", 0.6078, 1.4182, 30, 70, ptrFloat64(1.092))

	req := socRequest{
		sessionID:     "process-soc-session",
		vehicleID:     testVehicleID,
		startKwh:      0.6078,
		startTotalKwh: 1.092,
		targetKwh:     1.4182,
		createdAt:     startTime,
		startedAt:     &startTime,
		energy: &tasmota.EnergyData{
			Total:   1.100,
			Power:   600,
			Voltage: 230,
			Current: 2.6,
		},
	}

	err := service.ProcessSOC(context.Background(), req)
	require.NoError(t, err)

	// Verify snapshot was created
	repo := repository.NewChargeSessionRepository(db)
	snapshots, err := repo.GetSOCSnapshots(context.Background(), "process-soc-session")
	require.NoError(t, err)
	assert.Len(t, snapshots, 1)
	assert.Greater(t, snapshots[0].SocPercent, 30.0)
	assert.Less(t, snapshots[0].SocPercent, 32.0)

	// Verify LastBlendedKwh was updated
	found, err := repo.FindByID(context.Background(), "process-soc-session")
	require.NoError(t, err)
	require.NotNil(t, found.LastBlendedKwh)
	assert.Greater(t, *found.LastBlendedKwh, 0.0)
}

func TestChargeSessionService_ProcessSOC_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	req := socRequest{
		sessionID:     "orphan-session",
		vehicleID:     "nonexistent-vehicle",
		startKwh:      0.5,
		startTotalKwh: 1.0,
		targetKwh:     1.5,
		createdAt:     time.Now(),
		energy: &tasmota.EnergyData{
			Total:   1.1,
			Power:   600,
			Voltage: 230,
			Current: 2.6,
		},
	}

	err := service.ProcessSOC(context.Background(), req)
	// ProcessSOC returns nil when vehicle is not found (repo returns nil, nil)
	assert.NoError(t, err)
}

func TestChargeSessionService_CheckAndStopConditioningSession_ReachesTarget(t *testing.T) {
	db := setupServiceTestDB(t)

	// Insert vehicle with ChargerOutputW=600
	insertRawVehicle(t, db, "cond-vehicle", 5.0, 100, 200)
	// Update vehicle model to have charger_output_w > 0
	require.NoError(t, testdb.SetChargerOutput(db, "cond-vehicle", 600))

	// Insert a conditioning session directly
	insertSession(t, db, "cond-session", "cond-vehicle", "conditioning", 1.5, 5.0, 30, 100, ptrFloat64(100.0))

	// ChargerOutputW=600, threshold = 600 * 0.10 = 60W. Power below threshold triggers stop.
	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 105.0, Power: 30, Voltage: 230, Current: 0.13})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	service.CheckAndStopConditioningSession(context.Background())

	// Session should be completed
	found, err := service.sessionReader.FindByID(context.Background(), "cond-session")
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
}

func TestChargeSessionService_CheckAndStopConditioningSession_AboveThreshold(t *testing.T) {
	db := setupServiceTestDB(t)

	insertRawVehicle(t, db, "cond-vehicle2", 5.0, 100, 200)
	require.NoError(t, testdb.SetChargerOutput(db, "cond-vehicle2", 600))

	insertSession(t, db, "cond-session2", "cond-vehicle2", "conditioning", 1.5, 5.0, 30, 100, ptrFloat64(100.0))

	// Power above threshold (60W) - should NOT stop
	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 105.0, Power: 200, Voltage: 230, Current: 0.87})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	service.CheckAndStopConditioningSession(context.Background())

	// Session should still be conditioning
	found, err := service.sessionReader.FindByID(context.Background(), "cond-session2")
	require.NoError(t, err)
	assert.Equal(t, "conditioning", found.Status)
}

func TestChargeSessionService_CheckAndStopConditioningSession_NoEnergy(t *testing.T) {
	db := setupServiceTestDB(t)

	insertRawVehicle(t, db, "cond-vehicle3", 5.0, 100, 200)
	require.NoError(t, testdb.SetChargerOutput(db, "cond-vehicle3", 600))

	insertSession(t, db, "cond-session3", "cond-vehicle3", "conditioning", 1.5, 5.0, 30, 100, ptrFloat64(100.0))

	// nil plugCtrl - no energy data available
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	service.CheckAndStopConditioningSession(context.Background())

	// Session should still be conditioning (no energy to check)
	found, err := service.sessionReader.FindByID(context.Background(), "cond-session3")
	require.NoError(t, err)
	assert.Equal(t, "conditioning", found.Status)
}

func TestChargeSessionService_CheckAndStopConditioningSession_NotConditioning(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create an active (not conditioning) session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	service.CheckAndStopConditioningSession(context.Background())

	// Session should still be active
	found, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", found.Status)
}

func TestChargeSessionService_CheckAndCancelDisconnectedSession_CancelError(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Set LastBlendedKwh > epsilonKwh to simulate charging occurred
	blendedKwh := 0.5
	repo := repository.NewChargeSessionRepository(db)
	err = repo.UpdateLastBlendedKwh(context.Background(), session.ID, blendedKwh)
	require.NoError(t, err)

	// Close DB to force error in CancelActiveSession's UpdateCancelData
	db.Close()

	// Should not panic even when CancelActiveSession fails
	assert.NotPanics(t, func() {
		service.CheckAndCancelDisconnectedSession(context.Background())
	})
}

func TestChargeSessionService_HandleSensorMessage_SavesPowerReading(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.092, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	energy := &tasmota.EnergyData{Total: 1.5, Power: 600, Voltage: 230, Current: 2.6}
	service.HandleSensorMessage(context.Background(), testPlugID, energy)

	// Verify a power reading was saved
	readings, err := repository.NewChargeSessionRepository(db).GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, readings, "should have saved a power reading")
	assert.Equal(t, 600.0, readings[0].Power)
	assert.Equal(t, 230.0, readings[0].Voltage)
	assert.Equal(t, 2.6, readings[0].Current)
}

func TestChargeSessionService_HandleSensorMessage_WrongPlugID(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1.092, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Create and activate session on testPlugID
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	energy := &tasmota.EnergyData{Total: 1.5, Power: 600, Voltage: 230, Current: 2.6}
	// Send message for a different plug - should be a no-op
	service.HandleSensorMessage(context.Background(), "different-plug-id", energy)

	// No power reading should be saved for this session
	readings, err := repository.NewChargeSessionRepository(db).GetPowerReadings(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestChargeSessionService_NotifyPlugUnavailable_WithPushService(t *testing.T) {
	db := setupServiceTestDB(t)

	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/test",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	callCount := 0
	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK"}, nil
		},
	}
	ps := NewPushService(repo, "pub", "priv", client)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, ps)

	service.NotifyPlugUnavailable(context.Background(), &models.Plug{Name: "Test Plug", Type: models.PlugTypeCharging})

	// Wait for the async notification to complete
	service.Shutdown()

	assert.Greater(t, callCount, 0, "push notification should have been sent for plug unavailable")
}

func TestChargeSessionService_CancelActiveSession_DBError(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Fetch the active session to pass to CancelActiveSession
	active, err := service.GetActiveSession(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)

	// Close DB to force error in UpdateCancelData
	db.Close()

	err = service.CancelActiveSession(context.Background(), active)
	assert.Error(t, err, "CancelActiveSession should fail when DB is closed")
}

func TestChargeSessionService_CancelActiveSession_OwnershipMismatch(t *testing.T) {
	db := setupServiceTestDB(t)

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, nil, nil, nil)
	defer service.Shutdown()

	// Create and activate session
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 30, 70)
	require.NoError(t, err)
	require.NotNil(t, session)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Fetch the active session
	active, err := service.GetActiveSession(context.Background())
	require.NoError(t, err)
	require.NotNil(t, active)

	// Call with a context that has a different user ID
	ctx := internal.WithUserID(context.Background(), "different-user-id")
	err = service.CancelActiveSession(ctx, active)
	assert.Error(t, err, "CancelActiveSession should fail when user ID doesn't match")
}

func TestChargeSessionService_CancelPendingIfTimedOut_TimedOut(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Insert a pending session with a created_at in the past (2 minutes ago)
	oldTime := time.Now().Add(-2 * time.Minute)
	insertSession(t, db, "timedout-session", testVehicleID, "pending", 0.6, 1.4, 30, 70, nil)
	require.NoError(t, testdb.BackdateSession(db, "timedout-session", oldTime))

	// Timeout of 1 minute - session is older, should be cancelled
	canceled, err := service.CancelPendingIfTimedOut(context.Background(), 1*time.Minute)
	require.NoError(t, err)
	assert.True(t, canceled, "session should be cancelled because it exceeded the timeout")

	// Verify session is cancelled
	found, err := repository.NewChargeSessionRepository(db).FindByID(context.Background(), "timedout-session")
	require.NoError(t, err)
	assert.Equal(t, "cancelled", found.Status)

	// Verify power was cut
	ctrl.mu.RLock()
	assert.False(t, ctrl.powerOn[testPlugID], "plug power should be cut after cancelling timed out session")
	ctrl.mu.RUnlock()
}

func TestChargeSessionService_CancelPendingIfTimedOut_NotTimedOut(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// Insert a pending session created just now
	insertSession(t, db, "fresh-session", testVehicleID, "pending", 0.6, 1.4, 30, 70, nil)

	// Timeout of 10 minutes - session is fresh, should NOT be cancelled
	canceled, err := service.CancelPendingIfTimedOut(context.Background(), 10*time.Minute)
	require.NoError(t, err)
	assert.False(t, canceled, "session should NOT be cancelled because it hasn't exceeded the timeout")

	// Verify session is still pending
	found, err := repository.NewChargeSessionRepository(db).FindByID(context.Background(), "fresh-session")
	require.NoError(t, err)
	assert.Equal(t, "pending", found.Status)
}


