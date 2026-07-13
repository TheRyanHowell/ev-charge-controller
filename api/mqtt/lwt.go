package mqtt

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
)

// lwtSessionReader is a narrow read interface used by LWTManager to look up
// the active session for a plug. It is a subset of internal.SessionReader.
type lwtSessionReader interface {
	GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error)
}

// lwtOfflineDebounce is how long to wait after an LWT Offline before acting.
// An Online within this window suppresses the transition (flap suppression).
const lwtOfflineDebounce = 60 * time.Second

// lwtOfflineCooldown is the minimum gap between plug-unavailable notifications
// for the same plug, to prevent spamming during repeated flaps.
const lwtOfflineCooldown = 15 * time.Minute

// LWTPayload constants (Tasmota publishes these verbatim).
const (
	lwtOnline  = "Online"
	lwtOffline = "Offline"
)

// lwtLifecycle cancels active sessions on disconnect.
type lwtLifecycle interface {
	CancelActiveSession(ctx context.Context, session *models.ChargeSession) error
}

// lwtInitializer pushes first-time config to a plug when it comes Online.
type lwtInitializer interface {
	OnPlugOnline(ctx context.Context, plugID string) error
}

// lwtNotifier sends plug-unavailable push notifications.
type lwtNotifier interface {
	NotifyPlugUnavailable(ctx context.Context, plug *models.Plug)
}

// lwtPowerController cuts relay power; a narrow slice of internal.PlugController.
type lwtPowerController interface {
	SetPower(ctx context.Context, plugID string, on bool) error
}

// LWTManager tracks per-plug LWT state and fires handlers on transitions.
type LWTManager struct {
	plugRepo        internal.PlugRepo
	sessionReader   lwtSessionReader
	lifecycle       lwtLifecycle
	notifier        lwtNotifier
	initializer     lwtInitializer
	powerCtrl       lwtPowerController
	offlineDebounce time.Duration
	offlineCooldown time.Duration
	timers          map[string]*time.Timer
	mu              sync.Mutex
}

// NewLWTManager creates a LWTManager.
func NewLWTManager(plugRepo internal.PlugRepo, sessionReader lwtSessionReader, lifecycle lwtLifecycle, notifier lwtNotifier, initializer lwtInitializer) *LWTManager {
	return &LWTManager{
		plugRepo:        plugRepo,
		sessionReader:   sessionReader,
		lifecycle:       lifecycle,
		notifier:        notifier,
		initializer:     initializer,
		offlineDebounce: lwtOfflineDebounce,
		offlineCooldown: lwtOfflineCooldown,
		timers:          make(map[string]*time.Timer),
	}
}

// HandleLWT processes an LWT message for the given plugID.
// payload should be "Online" or "Offline".
// retained indicates the message was retained (state sync on connect, not a live transition).
func (m *LWTManager) HandleLWT(ctx context.Context, plugID, payload string, retained bool) {
	switch payload {
	case lwtOnline:
		m.handleOnline(ctx, plugID)
	case lwtOffline:
		m.handleOffline(ctx, plugID, retained)
	default:
		slog.Warn("mqtt/lwt: unknown LWT payload", "plugID", plugID, "payload", payload)
	}
}

func (m *LWTManager) handleOnline(ctx context.Context, plugID string) {
	m.mu.Lock()
	if t, ok := m.timers[plugID]; ok {
		t.Stop()
		delete(m.timers, plugID)
		slog.Info("mqtt/lwt: Online within debounce window, flap suppressed", "plugID", plugID)
	}
	m.mu.Unlock()

	if err := m.plugRepo.SetOnline(ctx, plugID, true); err != nil {
		slog.Warn("mqtt/lwt: failed to mark plug online", "plugID", plugID, "err", err)
	}

	if m.initializer != nil {
		if err := m.initializer.OnPlugOnline(ctx, plugID); err != nil {
			slog.Warn("mqtt/lwt: plug initialization failed", "plugID", plugID, "err", err)
		}
	}

	m.reconcileRelay(ctx, plugID)
}

// reconcileRelay forces a charging plug's relay OFF when it comes online with
// no active session. A plug that dropped offline mid-session (session
// cancelled) can come back with its relay still on - or restore it via
// SaveState after a reboot - and would then deliver energy no session tracks.
// Maintenance plugs default ON by design and are never touched; a legitimate
// session start racing this reconciliation simply powers the relay back on
// itself as part of its own power-confirmation flow.
func (m *LWTManager) reconcileRelay(ctx context.Context, plugID string) {
	if m.powerCtrl == nil {
		return
	}
	plug, err := m.plugRepo.FindByID(ctx, plugID)
	if err != nil || plug == nil || plug.Type != models.PlugTypeCharging {
		return
	}
	active, err := m.sessionReader.GetActiveByPlug(ctx, plugID)
	if err != nil || active != nil {
		return
	}
	if err := m.powerCtrl.SetPower(ctx, plugID, false); err != nil {
		slog.Warn("mqtt/lwt: relay reconciliation power-off failed", "plugID", plugID, "err", err)
		return
	}
	slog.Info("mqtt/lwt: reconciled relay off (online with no active session)", "plugID", plugID)
}

// SetPowerController wires in relay control after construction (the MQTT
// controller is built after the LWTManager, same pattern as SetInitializer).
func (m *LWTManager) SetPowerController(ctrl lwtPowerController) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.powerCtrl = ctrl
}

func (m *LWTManager) handleOffline(ctx context.Context, plugID string, retained bool) {
	m.mu.Lock()
	if _, ok := m.timers[plugID]; ok {
		// Debounce already running - don't reset it
		m.mu.Unlock()
		return
	}
	t := time.AfterFunc(m.offlineDebounce, func() {
		m.confirmOffline(ctx, plugID, retained)
	})
	m.timers[plugID] = t
	m.mu.Unlock()
	slog.Info("mqtt/lwt: Offline received, starting debounce", "plugID", plugID, "retained", retained)
}

// confirmOffline fires after the debounce window with no intervening Online.
func (m *LWTManager) confirmOffline(ctx context.Context, plugID string, retained bool) {
	m.mu.Lock()
	delete(m.timers, plugID)
	m.mu.Unlock()

	slog.Info("mqtt/lwt: confirmed offline after debounce", "plugID", plugID)

	if err := m.plugRepo.SetOnline(ctx, plugID, false); err != nil {
		slog.Warn("mqtt/lwt: failed to mark plug offline", "plugID", plugID, "err", err)
	}

	// Cancel any active session on this plug
	active, err := m.sessionReader.GetActiveByPlug(ctx, plugID)
	if err != nil {
		slog.Warn("mqtt/lwt: failed to get active session for offline plug", "plugID", plugID, "err", err)
	} else if active != nil {
		slog.Info("mqtt/lwt: cancelling active session due to plug offline", "plugID", plugID, "sessionID", active.ID)
		if err := m.lifecycle.CancelActiveSession(ctx, active); err != nil {
			slog.Warn("mqtt/lwt: failed to cancel active session", "err", err)
		}
	}

	// Gate notification on cooldown to suppress flap noise; skip on retained (stale state).
	if retained {
		return
	}

	plug, err := m.plugRepo.FindByID(ctx, plugID)
	if err != nil || plug == nil {
		return
	}
	if plug.LastOfflineNotifiedAt != nil && time.Since(*plug.LastOfflineNotifiedAt) < m.offlineCooldown {
		slog.Info("mqtt/lwt: plug unavailable notification suppressed by cooldown", "plugID", plugID)
		return
	}

	if err := m.plugRepo.UpdateLastOfflineNotifiedAt(ctx, plugID); err != nil {
		slog.Warn("mqtt/lwt: failed to update last_offline_notified_at", "err", err)
	}

	if m.notifier != nil {
		m.notifier.NotifyPlugUnavailable(ctx, plug)
	}
}

// SetInitializer wires in a plug initializer after construction (breaks the
// circular dependency between the MQTT client, dispatcher, and publisher).
func (m *LWTManager) SetInitializer(init lwtInitializer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initializer = init
}

// CancelAll stops all pending debounce timers (call on shutdown).
func (m *LWTManager) CancelAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.timers {
		t.Stop()
		delete(m.timers, id)
	}
}
