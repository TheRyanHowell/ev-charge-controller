package services

// Tests for graceful handling of MANUAL relay changes (physical button press,
// Tasmota web UI, third-party MQTT command). A manual OFF must complete the
// running session with its real progress; a manual ON must start a tracked
// session (or resume a held one) instead of letting energy flow untracked.

import (
	"context"
	"database/sql"
	"testing"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newManualToggleService(t *testing.T) (*ChargeSessionService, *mockPlugController, *sql.DB) {
	t.Helper()
	db := setupServiceTestDB(t)
	ctrl := newMockPlugCtrl()
	service := NewChargeSessionService(context.Background(),
		repository.NewChargeSessionRepository(db),
		repository.NewVehicleRepository(db),
		repository.NewPlugRepository(db),
		ctrl, nil, nil)
	t.Cleanup(service.Shutdown)
	return service, ctrl, db
}

func TestHandleManualPowerToggle_Off_CompletesActiveSessionWithProgress(t *testing.T) {
	service, ctrl, _ := newManualToggleService(t)
	// Live meter: blended = 0.4 + (101.0-100.0)*0.8 = 1.2 kWh ≈ 59.2% of 2.026.
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 101.0, Power: 600})

	startTotal := 100.0
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPercent:  20,
		TargetPercent: 80,
		Status:        models.SessionStatusActive,
		StartTotalKwh: &startTotal,
	}
	require.NoError(t, service.sessionWriter.Create(context.Background(), session))

	// User presses the plug's button: relay went OFF outside the app.
	service.HandleManualPowerToggle(context.Background(), testPlugID, false)

	stopped, err := service.sessionReader.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCompleted, stopped.Status, "manual power-off must complete the running session")
	require.NotNil(t, stopped.EndPercent)
	assert.InDelta(t, 59.2, *stopped.EndPercent, 0.2)

	vehicle, err := service.vehicleRepo.FindByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	assert.InDelta(t, 59.2, vehicle.CurrentPercent, 0.2, "vehicle percent must reflect the energy delivered before the button press")
}

func TestHandleManualPowerToggle_On_StartsTrackedSession(t *testing.T) {
	service, ctrl, db := newManualToggleService(t)
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 100.0, Power: 0})

	// The seeded plug has no vehicle assigned - link the default vehicle.
	_, err := db.Exec(`UPDATE plugs SET vehicle_id = ? WHERE id = ?`, testVehicleID, testPlugID)
	require.NoError(t, err)

	// User presses the plug's button: relay went ON with no session.
	service.HandleManualPowerToggle(context.Background(), testPlugID, true)

	session, err := service.sessionReader.GetActiveByPlug(context.Background(), testPlugID)
	require.NoError(t, err)
	require.NotNil(t, session, "manual power-on must start a tracked session so auto-stop protects the battery")
	assert.Equal(t, models.SessionStatusActive, session.Status)
	assert.Equal(t, testVehicleID, session.VehicleID)
	assert.InDelta(t, 20, session.StartPercent, 0.01)
	assert.InDelta(t, 80, session.TargetPercent, 0.01)
	require.NotNil(t, session.StartTotalKwh, "manual session must capture the energy baseline")
}

func TestHandleManualPowerToggle_On_ResumesHoldingSession(t *testing.T) {
	service, _, db := newManualToggleService(t)

	startTotal := 100.0
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:            "manual_resume",
		VehicleID:     testVehicleID,
		UserID:        testUserID,
		PlugID:        testPlugID,
		Status:        models.SessionStatusHolding,
		StartKwh:      0.4,
		TargetKwh:     1.6208,
		StartPct:      20,
		TargetPct:     80,
		StartTotalKwh: &startTotal,
	}))
	// Hold fields aren't part of ChargeSessionOpts - set them directly.
	_, err := db.Exec(`UPDATE charge_sessions SET hold_percent = 64, ready_by_time = '07:00' WHERE id = 'manual_resume'`)
	require.NoError(t, err)

	// User presses the button while the session is holding: they want to
	// charge NOW - resume stage 2 instead of leaving untracked energy flowing.
	service.HandleManualPowerToggle(context.Background(), testPlugID, true)

	resumed, err := service.sessionReader.FindByID(context.Background(), "manual_resume")
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, resumed.Status, "manual power-on during a hold must resume the session")
	assert.Nil(t, resumed.HoldPercent, "resume must clear the hold point so auto-stop targets the real target")
}

func TestHandleManualPowerToggle_On_VehicleAtTarget_CutsPowerBack(t *testing.T) {
	service, ctrl, _ := newManualToggleService(t)
	// Vehicle already at its target.
	require.NoError(t, service.vehicleRepo.UpdatePercents(context.Background(), testVehicleID, 80, 80))

	service.HandleManualPowerToggle(context.Background(), testPlugID, true)

	session, err := service.sessionReader.GetActiveByPlug(context.Background(), testPlugID)
	require.NoError(t, err)
	assert.Nil(t, session, "no session should start when the vehicle is already at target")

	ctrl.mu.RLock()
	on, tracked := ctrl.powerOn[testPlugID]
	ctrl.mu.RUnlock()
	assert.True(t, tracked, "relay must be commanded back off")
	assert.False(t, on, "relay must be turned back off to avoid untracked charging")
}

func TestHandleManualPowerToggle_MaintenancePlug_Ignored(t *testing.T) {
	service, ctrl, db := newManualToggleService(t)

	const maintenancePlugID = "maintenance-plug"
	_, err := db.Exec(`INSERT INTO plugs (id, user_id, name, namespace, mqtt_topic, type, created_at) VALUES (?, ?, 'Maint', 'ns-m', 'topic-m', 'maintenance', CURRENT_TIMESTAMP)`,
		maintenancePlugID, testUserID)
	require.NoError(t, err)

	service.HandleManualPowerToggle(context.Background(), maintenancePlugID, true)
	service.HandleManualPowerToggle(context.Background(), maintenancePlugID, false)

	session, err := service.sessionReader.GetActiveByPlug(context.Background(), maintenancePlugID)
	require.NoError(t, err)
	assert.Nil(t, session, "maintenance plugs must never get charge sessions from manual toggles")

	ctrl.mu.RLock()
	_, touched := ctrl.powerOn[maintenancePlugID]
	ctrl.mu.RUnlock()
	assert.False(t, touched, "maintenance plug relay must not be touched")
}
