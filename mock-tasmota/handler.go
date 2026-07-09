package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
)

// TasmotaHandler owns all mock Tasmota state and implements http.Handler.
type TasmotaHandler struct {
	mu            sync.RWMutex
	powerState    bool
	energyData    EnergyData
	lastUpdate    time.Time
	startTime     time.Time
	maxPowerWatts float64
	voltage       float64
	frequency     float64
	authUsername  string
	authPassword  string
	energyRes     int

	// MQTT state
	mqttMu        sync.RWMutex
	mqttConf      mqttConfig
	mqttConn      *autopaho.ConnectionManager
	mqttCancel    context.CancelFunc
	mqttConfDirty bool
	mqttStateFile string
}

var defaultHandler = &TasmotaHandler{
	energyData: EnergyData{
		Total: 1.092, Voltage: 230.0, Freq: 50.0,
	},
	voltage:   230.0,
	frequency: 50.0,
	energyRes: 4,
}

// Reset resets only the power and energy state. The MQTT config and connection
// are intentionally preserved: the seed flow pushes new credentials via
// "Restart 1" before calling /reset, so by the time /reset is called the
// reconnectMQTT goroutine is already running. Clearing the MQTT connection here
// would kill that goroutine and leave the device unable to publish power
// confirmations, causing PowerConfirmationTimeout errors on subsequent charges.
func (h *TasmotaHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.powerState = false
	h.energyData = EnergyData{
		Total: 1.092, LastTotal: 0,
		Freq: h.frequency, Voltage: h.voltage,
	}
	h.lastUpdate = time.Now()
	h.startTime = time.Time{}
}

func (h *TasmotaHandler) SetPower(on bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.powerState = on
	if on {
		h.startTime = time.Now()
		h.energyData.Power = h.maxPowerWatts
		h.energyData.Current = h.maxPowerWatts / h.voltage
	} else {
		h.energyData.Power = 0
		h.energyData.Current = 0
	}
	h.lastUpdate = time.Now()
}

func (h *TasmotaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r, w) {
		return
	}
	switch r.URL.Path {
	case "/cm":
		h.handleCM(w, r)
	case "/":
		h.handleRoot(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *TasmotaHandler) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<html><body><h1>LB_12</h1></body></html>")
}

func (h *TasmotaHandler) calculateRealisticPower() float64 {
	return h.maxPowerWatts
}

func (h *TasmotaHandler) getPowerState() string {
	if h.powerState {
		return "ON"
	}
	return "OFF"
}

// mqttDisconnectTimeout bounds how long Close waits for a graceful MQTT
// disconnect before giving up and just cancelling the connection's context.
const mqttDisconnectTimeout = 2 * time.Second

// Close disconnects any active MQTT connection and cancels its context. Call
// this in test cleanup to prevent leaked autopaho connections from still
// being mid-(re)connect when a test's embedded broker starts shutting down -
// mochi-mqtt's Close locks its client registry to tear down existing
// clients, and a connection attempt still in flight can contend that same
// lock long enough to look like a hang under `go test -race` on a
// constrained CI runner. Disconnecting synchronously first, before the
// broker closes, avoids that race.
func (h *TasmotaHandler) Close() {
	h.mqttMu.Lock()
	cm := h.mqttConn
	cancel := h.mqttCancel
	h.mqttConn = nil
	h.mqttCancel = nil
	h.mqttMu.Unlock()

	if cm != nil {
		ctx, done := context.WithTimeout(context.Background(), mqttDisconnectTimeout)
		_ = cm.Disconnect(ctx)
		done()
	}
	if cancel != nil {
		cancel()
	}
}
