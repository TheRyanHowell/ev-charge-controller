package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionLifecycleService_StartSession_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, "nonexistent", 20, 80)
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestSessionLifecycleService_StartSession_NoBatteryVehicle(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// A generic (battery-less) vehicle only supports 12V maintenance charging.
	insertRawVehicle(t, db, "no-battery", 0, 0, 0)

	session, err := service.StartSession(context.Background(), testPlugID, "no-battery", 20, 80)
	assert.ErrorIs(t, err, ErrVehicleHasNoBattery)
	assert.Nil(t, session)
}

func TestSessionLifecycleService_StartSession_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, testVehicleID, session.VehicleID)
	assert.Equal(t, models.SessionStatusActive, session.Status)
}

func TestSessionLifecycleService_StartSession_SendsChargeStartedNotification(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	push := &mockNotifierPushService{
		sendCh: make(chan struct{}),
		title:  new(string),
		body:   new(string),
	}
	notifier := newTestNotifier(push, vehicleRepo)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	require.NotNil(t, session)

	close(push.sendCh)
	notifier.Wait()

	title, body := push.GetTitleBody()
	assert.Equal(t, "Charge Started", title)
	assert.Contains(t, body, "80%")
}

func TestSessionLifecycleService_StartSession_ActiveSessionExists(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrActiveSessionExists)
	assert.Nil(t, session)
}

func TestSessionLifecycleService_StartTwoStageSession_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartTwoStageSession(context.Background(), testPlugID, testVehicleID, 20, 80, 64, "07:00", true)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, testVehicleID, session.VehicleID)
	assert.Equal(t, models.SessionStatusActive, session.Status)
	assert.Equal(t, 80.0, session.TargetPercent)
	require.NotNil(t, session.HoldPercent)
	assert.Equal(t, 64.0, *session.HoldPercent)
	require.NotNil(t, session.ReadyByTime)
	assert.Equal(t, "07:00", *session.ReadyByTime)
	assert.True(t, session.CarbonAwareHold)

	// Persisted values must round-trip, not just the in-memory struct.
	found, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found.HoldPercent)
	assert.Equal(t, 64.0, *found.HoldPercent)
	require.NotNil(t, found.ReadyByTime)
	assert.Equal(t, "07:00", *found.ReadyByTime)
	assert.True(t, found.CarbonAwareHold)
}

func TestSessionLifecycleService_StartTwoStageSession_DailyOriginNotCarbonAware(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartTwoStageSession(context.Background(), testPlugID, testVehicleID, 20, 80, 64, "07:00", false)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.False(t, session.CarbonAwareHold)

	found, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.False(t, found.CarbonAwareHold)
}

func TestSessionLifecycleService_StartTwoStageSession_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartTwoStageSession(context.Background(), testPlugID, "nonexistent", 20, 80, 64, "07:00", false)
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestSessionLifecycleService_ActivatePending_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	// Session is already active from StartSession, so ActivatePending returns early
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	baseline, err := service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)
	// Already active, baseline is 0
	assert.Equal(t, float64(0), baseline)
}

func TestSessionLifecycleService_ActivatePending_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	_, err := service.ActivatePending(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionLifecycleService_CancelPending_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "cancel-pending-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	err := service.CancelPending(context.Background(), sessionID)
	require.NoError(t, err)

	updated, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
}

func TestSessionLifecycleService_CancelPending_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	err := service.CancelPending(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionLifecycleService_CancelPendingIfTimedOut_NotTimedOut(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	_, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	cancelled, err := service.CancelPendingIfTimedOut(context.Background(), 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, cancelled)
}

func TestSessionLifecycleService_CancelPendingIfTimedOut_TimedOut(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "timedout-cancel-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	_, err := db.Exec("UPDATE charge_sessions SET created_at = ? WHERE id = ?", tenMinutesAgo, sessionID)
	require.NoError(t, err)

	cancelled, err := service.CancelPendingIfTimedOut(context.Background(), 5*time.Minute)
	require.NoError(t, err)
	assert.True(t, cancelled)
}

func TestSessionLifecycleService_UpdateTarget_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.UpdateTarget(context.Background(), session.ID, 90)
	require.NoError(t, err)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, 90.0, updated.TargetPercent)
}

func TestSessionLifecycleService_UpdateTarget_NotActive(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "not-active-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	err := service.UpdateTarget(context.Background(), sessionID, 90)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotActive)
}

func TestSessionLifecycleService_UpdateTarget_OutOfRange(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.UpdateTarget(context.Background(), session.ID, 101)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTargetOutOfRange)
}

func TestSessionLifecycleService_DeleteSession_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusCompleted,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.DeleteSession(context.Background(), session.ID)
	require.NoError(t, err)

	deleted, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestSessionLifecycleService_DeleteSession_Active(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.DeleteSession(context.Background(), session.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotDeleteActiveSession)
}

func TestSessionLifecycleService_CancelActiveSession_Active(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.CancelActiveSession(context.Background(), session)
	require.NoError(t, err)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
	assert.NotNil(t, updated.EndedAt)
}

func TestSessionLifecycleService_CancelActiveSession_Conditioning(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.CancelActiveSession(context.Background(), session)
	require.NoError(t, err)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
	assert.NotNil(t, updated.EndedAt)
}

func TestSessionLifecycleService_DeleteSession_Conditioning(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 100,
		TargetKwh:     1.9,
		Status:        models.SessionStatusConditioning,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.DeleteSession(context.Background(), session.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotDeleteActiveSession)
}

func TestSessionLifecycleService_validatePlugOwnership_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.validatePlugOwnership(ctx, testPlugID)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_validatePlugOwnership_PlugNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.validatePlugOwnership(ctx, "nonexistent-plug")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPlugNotFound)
}

func TestSessionLifecycleService_validatePlugOwnership_WrongUser(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), "wrong-user")
	err := service.validatePlugOwnership(ctx, testPlugID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPlugNotFound)
}

func TestSessionLifecycleService_validatePlugOwnership_NoUserContext(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := context.Background()
	err := service.validatePlugOwnership(ctx, testPlugID)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_validatePlugOwnership_EmptyPlugID(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.validatePlugOwnership(ctx, "")
	assert.NoError(t, err)
}

func TestSessionLifecycleService_validatePlugOwnership_NoPlugRepo(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.validatePlugOwnership(ctx, testPlugID)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_verifySessionOwnership_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		ID:        "test-session",
		UserID:    testUserIDPtr,
		VehicleID: testVehicleID,
		Status:    models.SessionStatusActive,
	}

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.verifySessionOwnership(ctx, session)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_verifySessionOwnership_WrongUser(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		ID:        "test-session",
		UserID:    testUserIDPtr,
		VehicleID: testVehicleID,
		Status:    models.SessionStatusActive,
	}

	ctx := internal.WithUserID(context.Background(), "wrong-user")
	err := service.verifySessionOwnership(ctx, session)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionLifecycleService_verifySessionOwnership_NoUserContext(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		ID:        "test-session",
		UserID:    testUserIDPtr,
		VehicleID: testVehicleID,
		Status:    models.SessionStatusActive,
	}

	ctx := context.Background()
	err := service.verifySessionOwnership(ctx, session)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_verifySessionOwnership_NoUserID(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		ID:        "test-session",
		UserID:    nil,
		VehicleID: testVehicleID,
		Status:    models.SessionStatusActive,
	}

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.verifySessionOwnership(ctx, session)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionLifecycleService_Stop_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	// Set energy BEFORE create so StartTotalKwh is captured
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)

	activeView := &models.ChargeSessionView{
		ChargeSession: *updated,
	}
	result, err := service.Stop(context.Background(), activeView)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestSessionLifecycleService_Stop_PendingSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	activeView := &models.ChargeSessionView{
		ChargeSession: *session,
	}
	result, err := service.Stop(context.Background(), activeView)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestSessionLifecycleService_Stop_NilSession(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	result, err := service.Stop(context.Background(), nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, result)
}

func TestSessionLifecycleService_StopWithPercent_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Set energy BEFORE create so StartTotalKwh is captured
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	result, err := service.StopWithPercent(context.Background(), session.ID, 75)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestSessionLifecycleService_StopWithPercent_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	result, err := service.StopWithPercent(context.Background(), "nonexistent", 75)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	assert.Nil(t, result)
}

func TestSessionLifecycleService_StopWithPercent_Pending(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	result, err := service.StopWithPercent(context.Background(), session.ID, 75)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestSessionLifecycleService_createSessionFromPercent_NoPlugFallback(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	session, err := service.createSessionFromPercent(ctx, "", testVehicleID, 20, 80, nil, nil, nil, false)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.NotNil(t, session.PlugID)
}

func TestSessionLifecycleService_createSessionFromPercent_WithPlug(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, plugRepo, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	session, err := service.createSessionFromPercent(ctx, testPlugID, testVehicleID, 20, 80, nil, nil, nil, false)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, testPlugID, *session.PlugID)
}

func TestSessionLifecycleService_calculateEndPercent_WithEnergy(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Set energy BEFORE create so StartTotalKwh is captured
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)

	activeView := &models.ChargeSessionView{
		ChargeSession: *updated,
	}
	percent := service.calculateEndPercent(context.Background(), activeView)
	assert.Greater(t, percent, 0.0)
}

func TestSessionLifecycleService_calculateEndPercent_NoEnergy(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	activeView := &models.ChargeSessionView{
		ChargeSession: *session,
	}
	percent := service.calculateEndPercent(context.Background(), activeView)
	assert.Equal(t, float64(80), percent)
}

func TestSessionLifecycleService_calculateEndPercent_CurrentPercentFallback(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	currentPercent := 55.0
	activeView := &models.ChargeSessionView{
		ChargeSession: *session,
		CurrentPercent: &currentPercent,
	}
	percent := service.calculateEndPercent(context.Background(), activeView)
	assert.Equal(t, currentPercent, percent)
}

func TestSessionLifecycleService_ValidateVehicleExists_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.validateVehicleExists(ctx, testVehicleID)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_ValidateVehicleExists_NotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), testUserID)
	err := service.validateVehicleExists(ctx, "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrVehicleNotFound)
}

func TestSessionLifecycleService_ValidateVehicleExists_WrongUser(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	ctx := internal.WithUserID(context.Background(), "wrong-user")
	err := service.validateVehicleExists(ctx, testVehicleID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrVehicleNotFound)
}

func TestSessionLifecycleService_ValidateVehicleExists_NoUserContext(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	ctx := context.Background()
	err := service.validateVehicleExists(ctx, testVehicleID)
	assert.NoError(t, err)
}

func TestSessionLifecycleService_UpdateTarget_BelowStart(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.UpdateTarget(context.Background(), session.ID, 15)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTargetBelowStart)
}

func TestSessionLifecycleService_UpdateTarget_BelowCurrent(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Set energy BEFORE create so StartTotalKwh is captured
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1000, Power: 600})
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.UpdateTarget(context.Background(), session.ID, 5)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTargetBelowCurrent)
}

func TestSessionLifecycleService_UpdateTarget_Negative(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	err = service.UpdateTarget(context.Background(), session.ID, -1)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTargetOutOfRange)
}

func TestSessionLifecycleService_CancelActiveSession_PendingStatus(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	err = service.CancelActiveSession(context.Background(), session)
	require.NoError(t, err)

	// CancelActiveSession doesn't check status, it cancels any session
	updated, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
}

func TestSessionLifecycleService_CancelActiveSession_DBError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	require.NoError(t, db.Close())

	err := service.CancelActiveSession(context.Background(), session)
	assert.Error(t, err)
}

func TestSessionLifecycleService_CancelActiveSession_SetPowerError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	require.NoError(t, sessRepo.UpdateStatus(context.Background(), session.ID, models.SessionStatusActive))

	// Set error AFTER session creation so CancelActiveSession hits it
	ctrl.setPowerErr = fmt.Errorf("plug unavailable")

	err = service.CancelActiveSession(context.Background(), session)
	assert.NoError(t, err) // SetPower failure is best-effort, doesn't return error
}

func TestSessionLifecycleService_CancelActiveSession_OwnershipError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	// Context with different user ID
	otherUserID := "other-user"
	ctx := internal.WithUserID(context.Background(), otherUserID)

	err := service.CancelActiveSession(ctx, session)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestSessionLifecycleService_CancelPendingSession_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "cancel-pending-stop-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	pendingSession, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)
	activeView := &models.ChargeSessionView{
		ChargeSession: *pendingSession,
	}
	result, err := service.Stop(context.Background(), activeView)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)

	updated, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
}

func TestSessionLifecycleService_CancelPendingSession_DBError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "cancel-pending-dberr-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	pendingSession, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)

	require.NoError(t, db.Close())

	activeView := &models.ChargeSessionView{
		ChargeSession: *pendingSession,
	}
	result, err := service.Stop(context.Background(), activeView)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSessionLifecycleService_CancelPendingSession_SetPowerError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "cancel-pending-stop-err-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	pendingSession, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)

	// Set error AFTER session creation so Stop hits it
	ctrl.setPowerErr = fmt.Errorf("plug unavailable")

	activeView := &models.ChargeSessionView{
		ChargeSession: *pendingSession,
	}
	result, err := service.Stop(context.Background(), activeView)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Stopped)
	assert.Equal(t, "plug unavailable", result.TasmotaErr)

	// Session should still be cancelled in DB despite SetPower failure
	updated, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
}

func TestSessionLifecycleService_CancelPending_WrongState(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	err := service.CancelPending(context.Background(), session.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestGetChargingEfficiency_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	eff := service.getChargingEfficiency(context.Background(), "nonexistent-vehicle")
	assert.Equal(t, models.DefaultChargingEfficiency, eff)
}

func TestGetChargingEfficiency_InvalidEfficiency(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Set vehicle model efficiency to 0 (invalid)
	_, err := db.Exec("UPDATE vehicle_models SET charging_efficiency = 0 WHERE id = 'rm1'")
	require.NoError(t, err)

	eff := service.getChargingEfficiency(context.Background(), testVehicleID)
	assert.Equal(t, models.DefaultChargingEfficiency, eff)
}

func TestGetChargingEfficiency_ValidEfficiency(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Set vehicle model efficiency to a custom value
	_, err := db.Exec("UPDATE vehicle_models SET charging_efficiency = 0.9 WHERE id = 'rm1'")
	require.NoError(t, err)

	eff := service.getChargingEfficiency(context.Background(), testVehicleID)
	assert.Equal(t, 0.9, eff)
}

func TestGetSessionCarbon_NilStats(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	// nil powerReadingStats
	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, nil, notifier, lock)

	avgCarbon, co2Grams := service.getSessionCarbon(context.Background(), "some-session-id", 1.0)
	assert.Nil(t, avgCarbon)
	assert.Equal(t, float64(0), co2Grams)
}

func TestGetSessionCarbon_ZeroWallKwh(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	avgCarbon, co2Grams := service.getSessionCarbon(context.Background(), "some-session-id", 0)
	assert.Nil(t, avgCarbon)
	assert.Equal(t, float64(0), co2Grams)
}

func TestComputeEndFromPercent_VehicleNotFound(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Session with nonexistent vehicle
	session := &models.ChargeSession{
		VehicleID:     "nonexistent",
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}

	endKwh := service.computeEndFromPercent(context.Background(), session, 80)
	assert.Equal(t, float64(0), endKwh)
}

func TestComputeEndFromPercent_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// rm1 vehicle has capacityKwh=2.026
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}

	endKwh := service.computeEndFromPercent(context.Background(), session, 80)
	assert.InDelta(t, 2.026*80/100, endKwh, 0.001)
}

func TestStopWithPercent_DBError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

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

	// Close DB to force error on UpdateEndWithStats
	require.NoError(t, db.Close())

	result, err := service.StopWithPercent(context.Background(), session.ID, 75)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestStopWithPercent_BatteryKwhZero(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Session where endKwh < startKwh (negative batteryKwh)
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:        testUserIDPtr,
		PlugID:        testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      2.0,
		TargetPercent: 25,
		TargetKwh:     0.5,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	result, err := service.StopWithPercent(context.Background(), session.ID, 5)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestVerifySessionOwnership_NoUserInContext(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		Status:    models.SessionStatusActive,
	}

	// No user in context - should pass (background worker)
	err := service.verifySessionOwnership(context.Background(), session)
	assert.NoError(t, err)
}

func TestVerifySessionOwnership_SessionNoUserID(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		UserID:    nil,
		Status:    models.SessionStatusActive,
	}

	ctx := internal.WithUserID(context.Background(), "some-user")
	err := service.verifySessionOwnership(ctx, session)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestVerifySessionOwnership_UserMismatch(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	otherUser := "other-user"
	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		UserID:    &otherUser,
		Status:    models.SessionStatusActive,
	}

	ctx := internal.WithUserID(context.Background(), "some-user")
	err := service.verifySessionOwnership(ctx, session)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestVerifySessionOwnership_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	uid := "matching-user"
	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		UserID:    &uid,
		Status:    models.SessionStatusActive,
	}

	ctx := internal.WithUserID(context.Background(), "matching-user")
	err := service.verifySessionOwnership(ctx, session)
	assert.NoError(t, err)
}

func TestCancelPending_DBError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	// Create a session first
	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)
	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)

	// Close DB to force error on CancelPending
	require.NoError(t, db.Close())

	err = service.CancelPending(context.Background(), session.ID)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrSessionNotFound) // Should be a DB error, not session not found
}

func TestCancelPendingIfTimedOut_SetPowerError_BestEffort(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create a pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "timedout-power-error-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)

	// Backdate the session so it's timed out
	_, err := db.Exec(`UPDATE charge_sessions SET created_at = datetime('now', '-2 hours') WHERE id = ?`, sessionID)
	require.NoError(t, err)

	// Set SetPower error - CancelPendingIfTimedOut should still succeed (best-effort)
	ctrl.setPowerErr = fmt.Errorf("relay failure")

	cancelled, err := service.CancelPendingIfTimedOut(context.Background(), time.Minute)
	require.NoError(t, err)
	assert.True(t, cancelled)

	// Verify session was actually cancelled
	updated, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
}

func TestDeleteSession_DBError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "delete-dberr-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)
	require.NoError(t, service.CancelPending(context.Background(), sessionID))

	// Close DB to force error on Delete
	require.NoError(t, db.Close())

	err := service.DeleteSession(context.Background(), sessionID)
	assert.Error(t, err)
}

func TestDeleteSession_UserMismatch(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	// Create pending session directly in DB (bypass StartSession which auto-activates)
	sessionID := "delete-usermismatch-test"
	insertSession(t, db, sessionID, testVehicleID, "pending", 20, 80, 0, 0, nil)
	require.NoError(t, service.CancelPending(context.Background(), sessionID))

	// Different user tries to delete
	ctx := internal.WithUserID(context.Background(), "other-user")
	err := service.DeleteSession(ctx, sessionID)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestUpdateTarget_DBError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Close DB to force error on UpdateTarget
	require.NoError(t, db.Close())

	err = service.UpdateTarget(context.Background(), session.ID, 90)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrSessionNotActive) // Should be a DB error
}

func TestUpdateTarget_VehicleConfigMissing(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	// Set energy BEFORE creating the service so ActivatePending captures it
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 100.0})
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)

	session, err := service.StartSession(context.Background(), testPlugID, testVehicleID, 20, 80)
	require.NoError(t, err)
	_, err = service.ActivatePending(context.Background(), session.ID)
	require.NoError(t, err)

	// Verify session has StartTotalKwh set (from ActivatePending energy capture)
	loaded, err := sessRepo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.StartTotalKwh, "StartTotalKwh should be set by ActivatePending")

	// Set capacity_kwh to 0 on the vehicle model to trigger ErrVehicleConfigMissing
	// (Can't delete the vehicle instance because ON DELETE CASCADE would also delete the session)
	_, err = db.Exec(`UPDATE vehicle_models SET capacity_kwh = 0 WHERE id = 'rm1'`)
	require.NoError(t, err)

	err = service.UpdateTarget(context.Background(), session.ID, 90)
	assert.ErrorIs(t, err, ErrVehicleConfigMissing)
}

func TestStopWithPercent_SetPowerError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	ctrl.setPowerErr = fmt.Errorf("relay failure")
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, nil, notifier, lock)

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

	result, err := service.StopWithPercent(context.Background(), session.ID, 75)
	require.NoError(t, err) // SetPower failure is best-effort
	require.NotNil(t, result)
	assert.False(t, result.Stopped)
	assert.Contains(t, result.TasmotaErr, "relay failure")
}

// mockPowerReadingStats implements internal.PowerReadingStats for tests.
type mockPowerReadingStats struct {
	err  error
	resp map[string]*float64
}

func (m *mockPowerReadingStats) GetAvgCarbonIntensityForSessions(_ context.Context, _ []string) (map[string]*float64, error) {
	return m.resp, m.err
}

// errorVehicleRepo wraps a real repo but injects errors on specific calls.
type errorVehicleRepo struct {
	*repository.VehicleRepository
	failUpdatePercents       bool
	failIncrementLifetime    bool
}

func (r *errorVehicleRepo) UpdatePercents(ctx context.Context, id string, currentPercent, targetPercent float64) error {
	if r.failUpdatePercents {
		return fmt.Errorf("simulated DB error on update percents")
	}
	return r.VehicleRepository.UpdatePercents(ctx, id, currentPercent, targetPercent)
}

func (r *errorVehicleRepo) IncrementLifetimeStats(ctx context.Context, id string, batteryKwh, wallKwh, co2Grams, costPence float64, sessionAt time.Time) error {
	if r.failIncrementLifetime {
		return fmt.Errorf("simulated DB error on increment lifetime")
	}
	return r.VehicleRepository.IncrementLifetimeStats(ctx, id, batteryKwh, wallKwh, co2Grams, costPence, sessionAt)
}

func TestStopWithPercent_UpdatePercentsError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	errRepo := &errorVehicleRepo{VehicleRepository: vehicleRepo, failUpdatePercents: true}

	service := NewSessionLifecycleService(sessRepo, sessRepo, errRepo, nil, ctrl, nil, notifier, lock)

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

	result, err := service.StopWithPercent(context.Background(), session.ID, 75)
	require.NoError(t, err) // UpdatePercents failure is best-effort (logged as warning)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestStopWithPercent_IncrementLifetimeStatsError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	errRepo := &errorVehicleRepo{VehicleRepository: vehicleRepo, failIncrementLifetime: true}

	service := NewSessionLifecycleService(sessRepo, sessRepo, errRepo, nil, ctrl, nil, notifier, lock)

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

	result, err := service.StopWithPercent(context.Background(), session.ID, 75)
	require.NoError(t, err) // IncrementLifetimeStats failure is best-effort (logged as warning)
	require.NotNil(t, result)
	assert.True(t, result.Stopped)
}

func TestGetSessionCarbon_Success(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	avg := 450.0
	mockStats := &mockPowerReadingStats{
		resp: map[string]*float64{"session-1": &avg},
	}

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, mockStats, notifier, lock)

	avgCarbon, co2Grams := service.getSessionCarbon(context.Background(), "session-1", 2.0)
	require.NotNil(t, avgCarbon)
	assert.Equal(t, 450.0, *avgCarbon)
	assert.Equal(t, 900.0, co2Grams) // 2.0 * 450.0
}

func TestGetSessionCarbon_GetAvgError(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	mockStats := &mockPowerReadingStats{
		err: fmt.Errorf("stats unavailable"),
	}

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, mockStats, notifier, lock)

	avgCarbon, co2Grams := service.getSessionCarbon(context.Background(), "session-1", 1.0)
	assert.Nil(t, avgCarbon)
	assert.Equal(t, float64(0), co2Grams)
}

func TestGetSessionCarbon_SessionNotInMap(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	avg := 450.0
	mockStats := &mockPowerReadingStats{
		resp: map[string]*float64{"other-session": &avg},
	}

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, mockStats, notifier, lock)

	avgCarbon, co2Grams := service.getSessionCarbon(context.Background(), "session-1", 1.0)
	assert.Nil(t, avgCarbon)
	assert.Equal(t, float64(0), co2Grams)
}

func TestGetSessionCarbon_NilValueInMap(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	mockStats := &mockPowerReadingStats{
		resp: map[string]*float64{"session-1": nil},
	}

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, mockStats, notifier, lock)

	avgCarbon, co2Grams := service.getSessionCarbon(context.Background(), "session-1", 1.0)
	assert.Nil(t, avgCarbon)
	assert.Equal(t, float64(0), co2Grams)
}
