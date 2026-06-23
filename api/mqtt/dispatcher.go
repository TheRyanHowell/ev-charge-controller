package mqtt

import (
	"context"
	"log/slog"
	"sync"

	"ev-charge-controller/api/tasmota"
)

// EnergyHandler is called by the dispatcher when a SENSOR message arrives.
type EnergyHandler func(ctx context.Context, plugID string, energy *tasmota.EnergyData)

// powerConfirmer provides a one-shot signal channel for stat/POWER confirmation.
type powerConfirmer struct {
	done chan bool // closed when stat/POWER arrives
	once sync.Once
}

// powerStatePersister persists the last-known relay state for a plug.
type powerStatePersister interface {
	SetPowerState(ctx context.Context, plugID string, on bool) error
}

// powerStateEntry holds the cached relay state for a plug.
type powerStateEntry struct {
	on    bool
	known bool
}

// Dispatcher routes incoming MQTT messages to the correct per-plug handler.
// It caches the last energy reading and relay power state per plug and
// serialises per-plug processing through a per-plug mutex so concurrent
// SENSOR messages for the same plug don't race.
type Dispatcher struct {
	plugCache  *PlugCache
	onSENSOR   EnergyHandler
	lwtManager *LWTManager
	powerRepo  powerStatePersister

	mu              sync.RWMutex
	lastEnergy      map[string]*tasmota.EnergyData // keyed by plugID
	lastPowerStates map[string]powerStateEntry      // keyed by plugID
	plugLocks       map[string]*sync.Mutex
	locksMu         sync.Mutex

	confirmersMu sync.Mutex
	confirmers   map[string]*powerConfirmer // plugID -> confirmer
}

// NewDispatcher creates a Dispatcher that calls onSENSOR for each energy message.
// lwtManager may be nil (LWT handling is skipped).
// powerRepo may be nil (relay state will not be persisted to DB).
func NewDispatcher(plugCache *PlugCache, onSENSOR EnergyHandler, lwtManager *LWTManager, powerRepo powerStatePersister) *Dispatcher {
	return &Dispatcher{
		plugCache:       plugCache,
		onSENSOR:        onSENSOR,
		lwtManager:      lwtManager,
		powerRepo:       powerRepo,
		lastEnergy:      make(map[string]*tasmota.EnergyData),
		lastPowerStates: make(map[string]powerStateEntry),
		plugLocks:       make(map[string]*sync.Mutex),
		confirmers:      make(map[string]*powerConfirmer),
	}
}

// RegisterPowerConfirm registers a one-shot confirmation channel for plugID.
// The returned channel is closed when the next stat/POWER message arrives for
// that plug. Callers should select on the channel with a timeout.
func (d *Dispatcher) RegisterPowerConfirm(plugID string) <-chan bool {
	d.confirmersMu.Lock()
	defer d.confirmersMu.Unlock()
	pc := &powerConfirmer{done: make(chan bool, 1)}
	d.confirmers[plugID] = pc
	return pc.done
}

// RemovePowerConfirm removes a pending confirmation channel for plugID.
func (d *Dispatcher) RemovePowerConfirm(plugID string) {
	d.confirmersMu.Lock()
	defer d.confirmersMu.Unlock()
	delete(d.confirmers, plugID)
}

// Dispatch routes a raw MQTT message by topic.
func (d *Dispatcher) Dispatch(ctx context.Context, topic string, payload []byte, retained bool) {
	pt, err := ParseTopic(topic)
	if err != nil {
		slog.Warn("mqtt: unroutable topic", "topic", topic, "err", err)
		return
	}

	switch {
	case pt.Prefix == "tele" && pt.Leaf == "SENSOR":
		d.dispatchSENSOR(ctx, topic, *pt, payload)
	case pt.Prefix == "tele" && pt.Leaf == "LWT":
		d.dispatchLWT(ctx, *pt, string(payload), retained)
	case pt.Prefix == "tele" && pt.Leaf == "STATE":
		d.dispatchSTATE(ctx, *pt, payload)
	case pt.Prefix == "stat" && pt.Leaf == "POWER":
		d.dispatchSTAT_POWER(ctx, *pt, payload)
	}
}

func (d *Dispatcher) dispatchSENSOR(ctx context.Context, topic string, pt ParsedTopic, payload []byte) {
	energy, err := ParseSENSOR(payload)
	if err != nil {
		slog.Warn("mqtt: bad SENSOR payload", "topic", topic, "err", err)
		return
	}

	plugID, ok := d.plugCache.Lookup(pt.Namespace, pt.Slug)
	if !ok {
		slog.Warn("mqtt: unknown plug", "namespace", pt.Namespace, "slug", pt.Slug)
		return
	}

	slog.Debug("mqtt: SENSOR received", "plugID", plugID, "power_w", energy.Power, "total_kwh", energy.Total)

	mu := d.plugLock(plugID)
	mu.Lock()
	defer mu.Unlock()

	d.mu.Lock()
	d.lastEnergy[plugID] = energy
	d.mu.Unlock()

	if d.onSENSOR != nil {
		d.onSENSOR(ctx, plugID, energy)
	}
}

func (d *Dispatcher) dispatchLWT(ctx context.Context, pt ParsedTopic, payload string, retained bool) {
	if d.lwtManager == nil {
		return
	}
	plugID, ok := d.plugCache.Lookup(pt.Namespace, pt.Slug)
	if !ok {
		slog.Warn("mqtt: LWT for unknown plug", "namespace", pt.Namespace, "slug", pt.Slug)
		return
	}
	d.lwtManager.HandleLWT(ctx, plugID, payload, retained)
}

func (d *Dispatcher) dispatchSTATE(ctx context.Context, pt ParsedTopic, payload []byte) {
	on, err := ParseSTATE(payload)
	if err != nil {
		slog.Warn("mqtt: bad tele/STATE payload", "namespace", pt.Namespace, "slug", pt.Slug, "err", err)
		return
	}

	plugID, ok := d.plugCache.Lookup(pt.Namespace, pt.Slug)
	if !ok {
		slog.Warn("mqtt: tele/STATE for unknown plug", "namespace", pt.Namespace, "slug", pt.Slug)
		return
	}

	slog.Debug("mqtt: tele/STATE received", "plugID", plugID, "power_on", on)
	d.cachePowerState(plugID, on)
	d.persistPowerState(ctx, plugID, on)
}

func (d *Dispatcher) dispatchSTAT_POWER(ctx context.Context, pt ParsedTopic, payload []byte) {
	on, err := ParsePowerState(payload)
	if err != nil {
		slog.Warn("mqtt: bad stat/POWER payload", "namespace", pt.Namespace, "slug", pt.Slug, "err", err)
		return
	}

	plugID, ok := d.plugCache.Lookup(pt.Namespace, pt.Slug)
	if !ok {
		slog.Warn("mqtt: stat/POWER for unknown plug", "namespace", pt.Namespace, "slug", pt.Slug)
		return
	}

	slog.Info("mqtt: stat/POWER received", "plugID", plugID, "power_on", on, "namespace", pt.Namespace, "slug", pt.Slug, "payload", string(payload))

	// Persist relay state so app-driven and external toggles both update the DB.
	d.cachePowerState(plugID, on)
	d.persistPowerState(ctx, plugID, on)

	d.confirmersMu.Lock()
	pc, exists := d.confirmers[plugID]
	if exists {
		delete(d.confirmers, plugID)
	}
	d.confirmersMu.Unlock()

	if exists {
		pc.once.Do(func() {
			slog.Info("mqtt: stat/POWER signalling confirmer", "plugID", plugID, "power_on", on)
			pc.done <- on
		})
		slog.Info("mqtt: stat/POWER confirmed", "plugID", plugID, "power_on", on)
	} else {
		slog.Warn("mqtt: stat/POWER no pending confirmation", "plugID", plugID, "power_on", on)
	}
}

// cachePowerState stores the relay state in memory.
func (d *Dispatcher) cachePowerState(plugID string, on bool) {
	d.mu.Lock()
	d.lastPowerStates[plugID] = powerStateEntry{on: on, known: true}
	d.mu.Unlock()
}

// persistPowerState writes the relay state to the database asynchronously.
func (d *Dispatcher) persistPowerState(ctx context.Context, plugID string, on bool) {
	if d.powerRepo == nil {
		return
	}
	if err := d.powerRepo.SetPowerState(ctx, plugID, on); err != nil {
		slog.Warn("mqtt: failed to persist power state", "plugID", plugID, "err", err)
	}
}

// LastEnergy returns the most recent cached energy for a plug, or nil if none.
func (d *Dispatcher) LastEnergy(plugID string) *tasmota.EnergyData {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastEnergy[plugID]
}

// LastPowerState returns the most recent cached relay state for a plug.
// known is false if no state has been received yet.
func (d *Dispatcher) LastPowerState(plugID string) (on, known bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	e := d.lastPowerStates[plugID]
	return e.on, e.known
}

// plugLock returns (creating if needed) the per-plug mutex for plugID.
func (d *Dispatcher) plugLock(plugID string) *sync.Mutex {
	d.locksMu.Lock()
	defer d.locksMu.Unlock()
	if mu, ok := d.plugLocks[plugID]; ok {
		return mu
	}
	mu := new(sync.Mutex)
	d.plugLocks[plugID] = mu
	return mu
}
