package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
)

type mockNotifierVehicleRepo struct {
	vehicle  *models.Vehicle
	callCount int
	mu       sync.Mutex
}

func (m *mockNotifierVehicleRepo) FindByID(_ context.Context, _ string) (*models.Vehicle, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()
	return m.vehicle, nil
}

func (m *mockNotifierVehicleRepo) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

type mockNotifierPushService struct {
	sendCh chan struct{}
	title  *string
	body   *string
	mu     sync.Mutex
}

func (m *mockNotifierPushService) SendNotification(_ context.Context, title, body string) error {
	if m.sendCh != nil {
		<-m.sendCh // block until signaled
	}
	m.mu.Lock()
	*m.title = title
	*m.body = body
	m.mu.Unlock()
	return nil
}

func (m *mockNotifierPushService) GetTitleBody() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.title, *m.body
}

func newTestNotifier(push pushNotifier, repo internal.VehicleReader) *ChargeNotifier {
	return &ChargeNotifier{
		pushService: push,
		vehicleRepo: repo,
		baseCtx:     context.Background(),
	}
}

func TestChargeNotifier_BuildNotificationBody_WithRangeModes(t *testing.T) {
	vehicle := &models.Vehicle{
		Name:       "Test Car",
		RangeMinMi: 100,
		RangeMaxMi: 150,
	}
	notifier := NewChargeNotifier(context.Background(), nil, &mockNotifierVehicleRepo{}, nil)

	body := notifier.buildNotificationBody("Test Car", 80, vehicle)
	assert.Equal(t, "Test Car Charge Complete (80%, ~80-120mi)", body)
}

func TestChargeNotifier_BuildNotificationBody_SingleRangeMode(t *testing.T) {
	vehicle := &models.Vehicle{
		Name:       "Test Car",
		RangeMinMi: 100,
		RangeMaxMi: 100,
	}
	notifier := NewChargeNotifier(context.Background(), nil, &mockNotifierVehicleRepo{}, nil)

	body := notifier.buildNotificationBody("Test Car", 80, vehicle)
	assert.Equal(t, "Test Car Charge Complete (80%, ~80mi)", body)
}

func TestChargeNotifier_BuildNotificationBody_NoVehicle(t *testing.T) {
	notifier := NewChargeNotifier(context.Background(), nil, &mockNotifierVehicleRepo{}, nil)

	body := notifier.buildNotificationBody("Unknown Car", 75, nil)
	assert.Equal(t, "Unknown Car reached 75%", body)
}

func TestChargeNotifier_BuildNotificationBody_Rounding(t *testing.T) {
	vehicle := &models.Vehicle{
		Name:       "Test Car",
		RangeMinMi: 250,
		RangeMaxMi: 320,
	}
	notifier := NewChargeNotifier(context.Background(), nil, &mockNotifierVehicleRepo{}, nil)

	body := notifier.buildNotificationBody("Test Car", 67, vehicle)
	assert.Equal(t, "Test Car Charge Complete (67%, ~168-214mi)", body)
}

func TestChargeNotifier_NotifyChargeComplete_SingleGoroutine(t *testing.T) {
	vehicle := &models.Vehicle{
		Name:                 "Test Car",
		RangeMinMi:           100,
		RangeMaxMi:           150,
		NotifyChargeComplete: true,
	}
	repo := &mockNotifierVehicleRepo{vehicle: vehicle}
	push := &mockNotifierPushService{
		sendCh: make(chan struct{}),
		title:  new(string),
		body:   new(string),
	}
	notifier := newTestNotifier(push, repo)

	session := &models.ChargeSession{
		ID:        "s1",
		VehicleID: "v1",
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
	}

	notifier.NotifyChargeComplete(context.Background(), session, 80)

	// Allow goroutine to start and call FindByID
	time.Sleep(50 * time.Millisecond)

	// Should only call FindByID once (synchronously, before goroutine)
	assert.Equal(t, 1, repo.getCallCount(), "FindByID should be called exactly once, not twice")

	// Complete the notification
	close(push.sendCh)
	time.Sleep(50 * time.Millisecond)

	pushTitle, pushBody := push.GetTitleBody()
	assert.Equal(t, "Charge Complete", pushTitle)
	assert.Equal(t, "Test Car Charge Complete (80%, ~80-120mi)", pushBody)
}

func TestChargeNotifier_NotifyChargeComplete_NoPushService(t *testing.T) {
	repo := &mockNotifierVehicleRepo{vehicle: nil}
	notifier := NewChargeNotifier(context.Background(), nil, repo, nil)

	session := &models.ChargeSession{
		ID:        "s1",
		VehicleID: "v1",
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
	}

	notifier.NotifyChargeComplete(context.Background(), session, 80)
	time.Sleep(50 * time.Millisecond)

	// Should return early, no DB query, no goroutine spawned
	assert.Equal(t, 0, repo.getCallCount())
}

func TestChargeNotifier_NotifyChargeComplete_NoVehicle(t *testing.T) {
	repo := &mockNotifierVehicleRepo{vehicle: nil}
	push := &mockNotifierPushService{
		sendCh: make(chan struct{}),
		title:  new(string),
		body:   new(string),
	}
	notifier := newTestNotifier(push, repo)

	session := &models.ChargeSession{
		ID:        "s1",
		VehicleID: "v1",
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
	}

	notifier.NotifyChargeComplete(context.Background(), session, 75)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, repo.getCallCount(), "FindByID should be called exactly once")

	close(push.sendCh)
	time.Sleep(50 * time.Millisecond)

	pushTitle, pushBody := push.GetTitleBody()
	assert.Equal(t, "Charge Complete", pushTitle)
	assert.Equal(t, " reached 75%", pushBody)
}

func TestChargeNotifier_NotifyPlugUnavailable_Success(t *testing.T) {
	push := &mockNotifierPushService{
		sendCh: make(chan struct{}),
		title:  new(string),
		body:   new(string),
	}
	notifier := newTestNotifier(push, nil)

	plug := &models.Plug{Name: "Garage Plug", Type: models.PlugTypeCharging}
	notifier.NotifyPlugUnavailable(context.Background(), plug)

	// Allow goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Complete the notification
	close(push.sendCh)
	notifier.Wait()

	pushTitle, pushBody := push.GetTitleBody()
	assert.Equal(t, "Charger Offline", pushTitle)
	assert.Equal(t, "Garage Plug is unavailable", pushBody)
}

func TestChargeNotifier_NotifyPlugUnavailable_NoPushService(t *testing.T) {
	notifier := NewChargeNotifier(context.Background(), nil, nil, nil)

	notifier.NotifyPlugUnavailable(context.Background(), &models.Plug{Name: "Test Plug", Type: models.PlugTypeCharging})
	time.Sleep(50 * time.Millisecond)

	// Should return early, no goroutine spawned
	notifier.Wait()
}
