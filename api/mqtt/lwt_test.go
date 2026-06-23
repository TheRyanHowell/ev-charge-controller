package mqtt

import (
	"context"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Minimal mocks ---

type mockPlugRepo struct {
	mu                      sync.Mutex
	setOnlineCalls          []setOnlineCall
	updateNotifiedCalls     []string
	findByIDResult          *models.Plug
	findByIDErr             error
	setOnlineErr            error
	updateNotifiedErr       error
}

type setOnlineCall struct {
	plugID string
	online bool
}

func (r *mockPlugRepo) SetOnline(_ context.Context, plugID string, online bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setOnlineCalls = append(r.setOnlineCalls, setOnlineCall{plugID, online})
	return r.setOnlineErr
}

func (r *mockPlugRepo) UpdateLastOfflineNotifiedAt(_ context.Context, plugID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateNotifiedCalls = append(r.updateNotifiedCalls, plugID)
	return r.updateNotifiedErr
}

func (r *mockPlugRepo) FindByID(_ context.Context, _ string) (*models.Plug, error) {
	return r.findByIDResult, r.findByIDErr
}

// Satisfy remaining PlugRepo methods (unused by LWT).
func (r *mockPlugRepo) Create(_ context.Context, _ *models.Plug) error              { return nil }
func (r *mockPlugRepo) FindByNamespaceAndSlug(_ context.Context, _, _ string) (*models.Plug, error) {
	return nil, nil
}
func (r *mockPlugRepo) ListNamespacesByUserID(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (r *mockPlugRepo) List(_ context.Context, _ string) ([]models.Plug, error) { return nil, nil }
func (r *mockPlugRepo) Update(_ context.Context, _ *models.Plug) error           { return nil }
func (r *mockPlugRepo) Delete(_ context.Context, _, _ string) error { return nil }
func (r *mockPlugRepo) SetInitialized(_ context.Context, _ string) error { return nil }
func (r *mockPlugRepo) SetPowerState(_ context.Context, _ string, _ bool) error { return nil }

type mockSessionReader struct {
	mu            sync.Mutex
	activeSession *models.ChargeSession
}

func (r *mockSessionReader) GetActiveByPlug(_ context.Context, _ string) (*models.ChargeSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeSession, nil
}

type mockLifecycle struct {
	mu             sync.Mutex
	cancelledIDs   []string
}

func (l *mockLifecycle) CancelActiveSession(_ context.Context, session *models.ChargeSession) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancelledIDs = append(l.cancelledIDs, session.ID)
	return nil
}

type mockNotifier struct {
	mu    sync.Mutex
	plugs []*models.Plug
}

func (n *mockNotifier) NotifyPlugUnavailable(_ context.Context, plug *models.Plug) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.plugs = append(n.plugs, plug)
}

// newFastLWTManager builds an LWTManager with short durations for fast tests.
func newFastLWTManager(plugRepo *mockPlugRepo, sessionReader *mockSessionReader, lifecycle *mockLifecycle, notifier *mockNotifier) *LWTManager {
	m := NewLWTManager(plugRepo, sessionReader, lifecycle, notifier, nil)
	m.offlineDebounce = 20 * time.Millisecond
	m.offlineCooldown = 50 * time.Millisecond
	return m
}

const testPlugID = "plug-test-1"

// --- Tests ---

func TestLWT_Online_SetsOnline(t *testing.T) {
	plugRepo := &mockPlugRepo{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, nil)

	m.HandleLWT(context.Background(), testPlugID, "Online", false)

	plugRepo.mu.Lock()
	calls := plugRepo.setOnlineCalls
	plugRepo.mu.Unlock()

	require.Len(t, calls, 1)
	assert.Equal(t, testPlugID, calls[0].plugID)
	assert.True(t, calls[0].online)
}

func TestLWT_Offline_SetsOfflineAfterDebounce(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)

	// Before debounce - should not be offline yet
	plugRepo.mu.Lock()
	callsBefore := len(plugRepo.setOnlineCalls)
	plugRepo.mu.Unlock()
	assert.Equal(t, 0, callsBefore)

	// After debounce
	time.Sleep(50 * time.Millisecond)

	plugRepo.mu.Lock()
	calls := plugRepo.setOnlineCalls
	plugRepo.mu.Unlock()

	require.Len(t, calls, 1)
	assert.Equal(t, testPlugID, calls[0].plugID)
	assert.False(t, calls[0].online)

	// Notification should have been sent
	notifier.mu.Lock()
	plugs := notifier.plugs
	notifier.mu.Unlock()
	require.Len(t, plugs, 1)
	assert.Equal(t, "Garage", plugs[0].Name)
}

func TestLWT_FlapSuppression_OnlineWithinDebounce(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Driveway"}}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	// Online arrives before debounce fires - should cancel the timer
	m.HandleLWT(context.Background(), testPlugID, "Online", false)

	// Wait past debounce window
	time.Sleep(50 * time.Millisecond)

	plugRepo.mu.Lock()
	calls := plugRepo.setOnlineCalls
	plugRepo.mu.Unlock()

	// Only the Online SetOnline(true) call; no Offline call
	require.Len(t, calls, 1)
	assert.True(t, calls[0].online)

	notifier.mu.Lock()
	assert.Empty(t, notifier.plugs)
	notifier.mu.Unlock()
}

func TestLWT_Offline_NoDoubleDebounce(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Driveway"}}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, &mockNotifier{})

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	m.HandleLWT(context.Background(), testPlugID, "Offline", false) // duplicate - must not reset timer or double-fire

	time.Sleep(50 * time.Millisecond)

	plugRepo.mu.Lock()
	calls := plugRepo.setOnlineCalls
	plugRepo.mu.Unlock()

	// Should be exactly one SetOnline(false), not two
	require.Len(t, calls, 1)
	assert.False(t, calls[0].online)
}

func TestLWT_Offline_CancelsActiveSession(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	sessionReader := &mockSessionReader{
		activeSession: &models.ChargeSession{ID: "sess-1"},
	}
	lifecycle := &mockLifecycle{}
	m := newFastLWTManager(plugRepo, sessionReader, lifecycle, &mockNotifier{})

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	lifecycle.mu.Lock()
	cancelled := lifecycle.cancelledIDs
	lifecycle.mu.Unlock()

	assert.Equal(t, []string{"sess-1"}, cancelled)
}

func TestLWT_Offline_NoSession_NoCancel(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	lifecycle := &mockLifecycle{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{activeSession: nil}, lifecycle, &mockNotifier{})

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	lifecycle.mu.Lock()
	assert.Empty(t, lifecycle.cancelledIDs)
	lifecycle.mu.Unlock()
}

func TestLWT_Retained_SkipsNotification(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	// retained=true - stale state sync on reconnect, must not notify
	m.HandleLWT(context.Background(), testPlugID, "Offline", true)
	time.Sleep(50 * time.Millisecond)

	notifier.mu.Lock()
	assert.Empty(t, notifier.plugs, "retained offline must not send notification")
	notifier.mu.Unlock()
}

func TestLWT_Cooldown_SuppressesRepeatedNotification(t *testing.T) {
	now := time.Now()
	plugRepo := &mockPlugRepo{
		findByIDResult: &models.Plug{
			ID:                    testPlugID,
			Name:                  "Garage",
			LastOfflineNotifiedAt: &now, // just notified
		},
	}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// SetOnline(false) still fires, but notification is suppressed
	plugRepo.mu.Lock()
	calls := plugRepo.setOnlineCalls
	plugRepo.mu.Unlock()
	require.Len(t, calls, 1)
	assert.False(t, calls[0].online)

	notifier.mu.Lock()
	assert.Empty(t, notifier.plugs, "cooldown must suppress notification")
	notifier.mu.Unlock()
}

func TestLWT_CancelAll_StopsPendingTimers(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)

	m.mu.Lock()
	assert.Len(t, m.timers, 1)
	m.mu.Unlock()

	m.CancelAll()

	m.mu.Lock()
	assert.Empty(t, m.timers)
	m.mu.Unlock()

	// Debounce must not fire after CancelAll
	time.Sleep(50 * time.Millisecond)

	plugRepo.mu.Lock()
	assert.Empty(t, plugRepo.setOnlineCalls)
	plugRepo.mu.Unlock()

	notifier.mu.Lock()
	assert.Empty(t, notifier.plugs)
	notifier.mu.Unlock()
}

func TestLWT_UnknownPayload_Warns(t *testing.T) {
	plugRepo := &mockPlugRepo{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, nil)

	// Unknown payload should be handled gracefully (no panic, no action)
	m.HandleLWT(context.Background(), testPlugID, "UnknownPayload", false)

	plugRepo.mu.Lock()
	assert.Empty(t, plugRepo.setOnlineCalls)
	plugRepo.mu.Unlock()
}

func TestLWT_HandleOnline_SetOnlineError(t *testing.T) {
	plugRepo := &mockPlugRepo{setOnlineErr: assert.AnError}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, nil)

	m.HandleLWT(context.Background(), testPlugID, "Online", false)

	// Should still attempt SetOnline despite error (error is logged, not returned)
	plugRepo.mu.Lock()
	require.Len(t, plugRepo.setOnlineCalls, 1)
	plugRepo.mu.Unlock()
}

func TestLWT_HandleOnline_WithInitializer(t *testing.T) {
	plugRepo := &mockPlugRepo{}
	init := &mockInitializer{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, nil)
	m.initializer = init

	m.HandleLWT(context.Background(), testPlugID, "Online", false)

	assert.Equal(t, testPlugID, init.lastPlugID)
}

func TestLWT_HandleOnline_InitializerError(t *testing.T) {
	plugRepo := &mockPlugRepo{}
	init := &mockInitializer{err: assert.AnError}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, nil)
	m.initializer = init

	m.HandleLWT(context.Background(), testPlugID, "Online", false)

	// Should still call SetOnline despite initializer error
	plugRepo.mu.Lock()
	require.Len(t, plugRepo.setOnlineCalls, 1)
	plugRepo.mu.Unlock()
}

func TestLWT_SetInitializer(t *testing.T) {
	plugRepo := &mockPlugRepo{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, nil)
	assert.Nil(t, m.initializer)

	init := &mockInitializer{}
	m.SetInitializer(init)
	assert.Equal(t, init, m.initializer)
}

func TestLWT_ConfirmOffline_SetOnlineError(t *testing.T) {
	plugRepo := &mockPlugRepo{
		setOnlineErr:   assert.AnError,
		findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"},
	}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// SetOnline was attempted despite error
	plugRepo.mu.Lock()
	require.Len(t, plugRepo.setOnlineCalls, 1)
	plugRepo.mu.Unlock()
}

func TestLWT_ConfirmOffline_SessionReaderError(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	sessionReader := &mockSessionReaderWithErr{err: assert.AnError}
	lifecycle := &mockLifecycle{}
	notifier := &mockNotifier{}
	m := NewLWTManager(plugRepo, sessionReader, lifecycle, notifier, nil)
	m.offlineDebounce = 20 * time.Millisecond
	m.offlineCooldown = 50 * time.Millisecond

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// No session cancelled because reader errored
	lifecycle.mu.Lock()
	assert.Empty(t, lifecycle.cancelledIDs)
	lifecycle.mu.Unlock()
}

func TestLWT_ConfirmOffline_CancelActiveSessionError(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	sessionReader := &mockSessionReader{
		activeSession: &models.ChargeSession{ID: "sess-fail"},
	}
	lifecycle := &mockLifecycleWithErr{err: assert.AnError}
	notifier := &mockNotifier{}
	m := NewLWTManager(plugRepo, sessionReader, lifecycle, notifier, nil)
	m.offlineDebounce = 20 * time.Millisecond
	m.offlineCooldown = 50 * time.Millisecond

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// Session was still attempted to be cancelled despite error
	lifecycle.mu.Lock()
	assert.Equal(t, []string{"sess-fail"}, lifecycle.cancelledIDs)
	lifecycle.mu.Unlock()
}

func TestLWT_ConfirmOffline_PlugNotFound(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: nil, findByIDErr: assert.AnError}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// No notification sent when plug not found
	notifier.mu.Lock()
	assert.Empty(t, notifier.plugs)
	notifier.mu.Unlock()
}

func TestLWT_ConfirmOffline_NotifierNil(t *testing.T) {
	plugRepo := &mockPlugRepo{findByIDResult: &models.Plug{ID: testPlugID, Name: "Garage"}}
	notifier := lwtNotifier(nil) // true nil interface, not a typed nil pointer
	m := NewLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier, nil)
	m.offlineDebounce = 20 * time.Millisecond
	m.offlineCooldown = 50 * time.Millisecond

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// Should not panic with nil notifier
	plugRepo.mu.Lock()
	require.Len(t, plugRepo.setOnlineCalls, 1)
	plugRepo.mu.Unlock()
}

func TestLWT_ConfirmOffline_UpdateNotifiedError(t *testing.T) {
	plugRepo := &mockPlugRepo{
		findByIDResult:      &models.Plug{ID: testPlugID, Name: "Garage"},
		updateNotifiedErr:   assert.AnError,
	}
	notifier := &mockNotifier{}
	m := newFastLWTManager(plugRepo, &mockSessionReader{}, &mockLifecycle{}, notifier)

	m.HandleLWT(context.Background(), testPlugID, "Offline", false)
	time.Sleep(50 * time.Millisecond)

	// Notification should still be sent despite UpdateLastOfflineNotifiedAt error
	notifier.mu.Lock()
	plugs := notifier.plugs
	notifier.mu.Unlock()
	require.Len(t, plugs, 1)
	assert.Equal(t, "Garage", plugs[0].Name)
}

// ---------------------------------------------------------------------------
// Additional mock types for edge-case tests
// ---------------------------------------------------------------------------

type mockInitializer struct {
	mu       sync.Mutex
	lastPlugID string
	err      error
}

func (i *mockInitializer) OnPlugOnline(_ context.Context, plugID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.lastPlugID = plugID
	return i.err
}

type mockSessionReaderWithErr struct {
	err error
}

func (r *mockSessionReaderWithErr) GetActiveByPlug(_ context.Context, _ string) (*models.ChargeSession, error) {
	return nil, r.err
}

type mockLifecycleWithErr struct {
	mu             sync.Mutex
	cancelledIDs   []string
	err            error
}

func (l *mockLifecycleWithErr) CancelActiveSession(_ context.Context, session *models.ChargeSession) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancelledIDs = append(l.cancelledIDs, session.ID)
	return l.err
}
