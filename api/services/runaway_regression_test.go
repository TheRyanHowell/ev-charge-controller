package services

// Regression tests for the "runaway charge session" family of bugs: a session
// whose wall-side energy baseline (StartTotalKwh) was never captured can never
// compute progress, so auto-stop never fires and the vehicle keeps charging
// past its target and past the schedule's ready-by window.

import (
	"context"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActivatePending_BackfillsMissingBaseline covers the poller activation
// path: a session created while the plug had no cached MQTT energy (nil
// StartTotalKwh) must capture the baseline at activation time, once energy is
// available - otherwise the session can never auto-stop.
func TestActivatePending_BackfillsMissingBaseline(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Pending session with no energy baseline (plug cache was empty at creation).
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		CreatedAt:     time.Now(),
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPercent:  20,
		TargetPercent: 80,
		Status:        models.SessionStatusPending,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// MQTT energy is now available (e.g. first SENSOR message arrived).
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 100.0, Power: 600})

	_, err := service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	loaded, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, loaded.Status)
	require.NotNil(t, loaded.StartTotalKwh, "ActivatePending must backfill a missing energy baseline")
	assert.InDelta(t, 100.0, *loaded.StartTotalKwh, 1e-9)
}

// TestCheckAndAutoStopReachingSession_BackfillsMissingBaseline covers the
// self-heal path: an already-active session with no baseline (e.g. activated
// inline before the plug's first MQTT message, or created before an API
// restart wiped the cache) must capture the baseline on the next monitoring
// tick, and then auto-stop normally once the target is reached.
func TestCheckAndAutoStopReachingSession_BackfillsMissingBaseline(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 100.0, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	insertActiveSession(t, db, "runaway_no_baseline", testVehicleID, 0.4, 1.6208, 20, 80, nil, nil)

	// First tick: baseline missing - must be backfilled from live energy, not skipped forever.
	service.CheckAndAutoStopReachingSession(context.Background())

	loaded, err := service.sessionReader.FindByID(context.Background(), "runaway_no_baseline")
	require.NoError(t, err)
	require.NotNil(t, loaded.StartTotalKwh, "auto-stop check must backfill a missing energy baseline")
	assert.InDelta(t, 100.0, *loaded.StartTotalKwh, 1e-9)
	assert.Equal(t, models.SessionStatusActive, loaded.Status, "session should keep charging until target")

	// Energy advances past the target: blended = 0.4 + (101.53-100.0)*0.8 = 1.624 >= 1.6208.
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 101.53, Power: 600})
	service.CheckAndAutoStopReachingSession(context.Background())

	loaded, err = service.sessionReader.FindByID(context.Background(), "runaway_no_baseline")
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCompleted, loaded.Status, "session must auto-stop once target is reached")
}

// TestCheckAndAutoStopReachingSession_StopsAllActiveSessions covers concurrent
// sessions on different plugs: the monitoring tick must evaluate every active
// session, not just the most recently created one. Before the fix, the older
// session was invisible to auto-stop and charged indefinitely.
func TestCheckAndAutoStopReachingSession_StopsAllActiveSessions(t *testing.T) {
	db := setupServiceTestDB(t)

	const secondPlugID = "test-plug-2"
	require.NoError(t, testdb.InsertPlug(db, secondPlugID, testUserID, "Second Plug", "ns2", "topic2"))

	ctrl := newMockPlugCtrl()
	// Older session's plug has reached target; newer session's plug has not.
	ctrl.SetEnergy(secondPlugID, &tasmota.EnergyData{Total: 101.53, Power: 600})
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 100.1, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	olderStart := time.Now().Add(-2 * time.Hour)
	newerStart := time.Now().Add(-1 * time.Hour)
	startTotal := 100.0

	// Older session (created first) on the second plug - already at target.
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:            "older_at_target",
		VehicleID:     testVehicleID2,
		UserID:        testUserID,
		PlugID:        secondPlugID,
		Status:        "active",
		CreatedAt:     olderStart,
		StartedAt:     &olderStart,
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPct:      20,
		TargetPct:     80,
		StartTotalKwh: &startTotal,
	}))

	// Newer session on the default plug - still below target.
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:            "newer_below_target",
		VehicleID:     testVehicleID,
		UserID:        testUserID,
		PlugID:        testPlugID,
		Status:        "active",
		CreatedAt:     newerStart,
		StartedAt:     &newerStart,
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPct:      20,
		TargetPct:     80,
		StartTotalKwh: &startTotal,
	}))

	service.CheckAndAutoStopReachingSession(context.Background())

	older, err := service.sessionReader.FindByID(context.Background(), "older_at_target")
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCompleted, older.Status, "older session at target must be auto-stopped even when a newer session exists")

	newer, err := service.sessionReader.FindByID(context.Background(), "newer_below_target")
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, newer.Status, "newer session below target must keep charging")
}

// TestCancelActiveSession_PersistsProgress covers the "cancellation loses
// delivered energy" bug: a session cancelled mid-charge (plug offline, MQTT
// drop) had delivered real energy, but neither the session's end percent nor
// the vehicle's current percent recorded it. The next scheduled session then
// started from the stale pre-charge percent, double-charging the battery and
// under-reading the gauge for the rest of the day.
func TestCancelActiveSession_PersistsProgress(t *testing.T) {
	db := setupServiceTestDB(t)

	// Plug controller with NO cached energy - the plug just went offline,
	// which is exactly when sessions get cancelled. Progress must come from
	// the persisted LastBlendedKwh.
	ctrl := newMockPlugCtrl()

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	// rm1: capacity 2.026 kWh. Blended 1.5 kWh ≈ 74.04%.
	lastBlended := 1.5
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:             "cancel_with_progress",
		VehicleID:      testVehicleID,
		UserID:         testUserID,
		PlugID:         testPlugID,
		Status:         "active",
		StartKwh:       0.4,
		TargetKwh:      1.6208,
		StartPct:       20,
		TargetPct:      80,
		LastBlendedKwh: &lastBlended,
	}))

	session, err := service.sessionReader.FindByID(context.Background(), "cancel_with_progress")
	require.NoError(t, err)
	require.NoError(t, service.CancelActiveSession(context.Background(), session))

	// Session records how far it actually got.
	cancelled, err := service.sessionReader.FindByID(context.Background(), "cancel_with_progress")
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, cancelled.Status)
	require.NotNil(t, cancelled.EndPercent, "cancelled session must record its end percent")
	assert.InDelta(t, 74.04, *cancelled.EndPercent, 0.1)

	// Vehicle keeps the delivered energy - the next scheduled session must
	// start from ~74%, not the stale 20%.
	vehicle, err := service.vehicleRepo.FindByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	assert.InDelta(t, 74.04, vehicle.CurrentPercent, 0.1, "vehicle current percent must reflect energy delivered before cancellation")
}

// TestCancelActiveSession_NoProgress_LeavesVehicleUntouched: a session
// cancelled before any energy flowed must not disturb the vehicle's percent.
func TestCancelActiveSession_NoProgress_LeavesVehicleUntouched(t *testing.T) {
	db := setupServiceTestDB(t)
	ctrl := newMockPlugCtrl()

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "cancel_no_progress",
		VehicleID: testVehicleID,
		UserID:    testUserID,
		PlugID:    testPlugID,
		Status:    "active",
		StartKwh:  0.4,
		TargetKwh: 1.6208,
		StartPct:  20,
		TargetPct: 80,
	}))

	session, err := service.sessionReader.FindByID(context.Background(), "cancel_no_progress")
	require.NoError(t, err)
	require.NoError(t, service.CancelActiveSession(context.Background(), session))

	cancelled, err := service.sessionReader.FindByID(context.Background(), "cancel_no_progress")
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, cancelled.Status)
	assert.Nil(t, cancelled.EndPercent)

	vehicle, err := service.vehicleRepo.FindByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	assert.InDelta(t, 20, vehicle.CurrentPercent, 0.01, "vehicle percent must be untouched when no energy flowed")
}

// TestCancelActiveSession_PlugStillOnline_UsesLiveEnergy: when the plug is
// still reporting at cancellation time (it may have kept delivering energy
// right up to this moment), the end percent must come from the LIVE meter
// reading, not the older persisted blended value.
func TestCancelActiveSession_PlugStillOnline_UsesLiveEnergy(t *testing.T) {
	db := setupServiceTestDB(t)

	ctrl := newMockPlugCtrl()
	// Live meter is ahead of the last persisted blended value:
	// live blended = 0.4 + (101.5-100.0)*0.8 = 1.6 kWh ≈ 78.97% of 2.026.
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 101.5, Power: 600})

	service := NewChargeSessionService(context.Background(), repository.NewChargeSessionRepository(db), repository.NewVehicleRepository(db), nil, ctrl, nil, nil)
	defer service.Shutdown()

	startTotal := 100.0
	lastBlended := 1.0 // ≈49.4% - stale relative to the live meter
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:             "cancel_live_energy",
		VehicleID:      testVehicleID,
		UserID:         testUserID,
		PlugID:         testPlugID,
		Status:         "active",
		StartKwh:       0.4,
		TargetKwh:      1.6208,
		StartPct:       20,
		TargetPct:      80,
		StartTotalKwh:  &startTotal,
		LastBlendedKwh: &lastBlended,
	}))

	session, err := service.sessionReader.FindByID(context.Background(), "cancel_live_energy")
	require.NoError(t, err)
	require.NoError(t, service.CancelActiveSession(context.Background(), session))

	cancelled, err := service.sessionReader.FindByID(context.Background(), "cancel_live_energy")
	require.NoError(t, err)
	require.NotNil(t, cancelled.EndPercent)
	assert.InDelta(t, 78.97, *cancelled.EndPercent, 0.1, "live meter reading must win over the stale persisted blended value")

	vehicle, err := service.vehicleRepo.FindByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	assert.InDelta(t, 78.97, vehicle.CurrentPercent, 0.1)
}
