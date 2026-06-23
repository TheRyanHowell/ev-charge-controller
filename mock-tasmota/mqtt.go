package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	pahopkg "github.com/eclipse/paho.golang/paho"
)

// handleMQTTConfigCmd handles MQTT configuration commands sent via /cm.
// Returns true if the command was handled, false if it is not an MQTT config command.
func (h *TasmotaHandler) handleMQTTConfigCmd(w http.ResponseWriter, cmd string) bool {
	upper := strings.ToUpper(cmd)
	switch {
	case strings.HasPrefix(upper, "MQTTHOST "):
		h.mqttMu.Lock()
		h.mqttConf.Host = strings.TrimPrefix(cmd, "MQTTHost ")
		h.mqttConfDirty = true
		h.mqttMu.Unlock()
		h.saveMQTTConfig()
		_ = json.NewEncoder(w).Encode(map[string]string{"MQTTHost": h.mqttConf.Host})

	case strings.HasPrefix(upper, "MQTTPORT "):
		h.mqttMu.Lock()
		h.mqttConf.Port = strings.TrimPrefix(cmd, "MQTTPort ")
		h.mqttConfDirty = true
		h.mqttMu.Unlock()
		h.saveMQTTConfig()
		_ = json.NewEncoder(w).Encode(map[string]string{"MQTTPort": h.mqttConf.Port})

	case strings.HasPrefix(upper, "MQTTUSER "):
		h.mqttMu.Lock()
		h.mqttConf.Username = strings.TrimPrefix(cmd, "MQTTUser ")
		h.mqttConfDirty = true
		h.mqttMu.Unlock()
		h.saveMQTTConfig()
		_ = json.NewEncoder(w).Encode(map[string]string{"MQTTUser": h.mqttConf.Username})

	case strings.HasPrefix(upper, "MQTTPASSWORD "):
		h.mqttMu.Lock()
		h.mqttConf.Password = strings.TrimPrefix(cmd, "MQTTPassword ")
		h.mqttConfDirty = true
		h.mqttMu.Unlock()
		h.saveMQTTConfig()
		_ = json.NewEncoder(w).Encode(map[string]string{"MQTTPassword": "***"})

	case strings.HasPrefix(upper, "FULLTOPIC "):
		raw := strings.TrimPrefix(cmd, "FullTopic ")
		h.mqttMu.Lock()
		h.mqttConf.Namespace = parseNamespaceFromFullTopic(raw)
		h.mqttConfDirty = true
		h.mqttMu.Unlock()
		h.saveMQTTConfig()
		_ = json.NewEncoder(w).Encode(map[string]string{"FullTopic": raw})

	case strings.HasPrefix(upper, "TOPIC "):
		h.mqttMu.Lock()
		h.mqttConf.Slug = strings.TrimPrefix(cmd, "Topic ")
		h.mqttConfDirty = true
		h.mqttMu.Unlock()
		h.saveMQTTConfig()
		_ = json.NewEncoder(w).Encode(map[string]string{"Topic": h.mqttConf.Slug})

	default:
		return false
	}
	return true
}

// parseNamespaceFromFullTopic extracts the namespace segment from
// "evcc/<namespace>/%prefix%/%topic%/" patterns.
func parseNamespaceFromFullTopic(fullTopic string) string {
	fullTopic = strings.TrimSpace(fullTopic)
	if !strings.HasPrefix(fullTopic, "evcc/") {
		return ""
	}
	rest := strings.TrimPrefix(fullTopic, "evcc/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

// reconnectMQTT tears down any existing MQTT connection and reconnects with
// the current mqttConf. Called asynchronously after "Restart 1".
func (h *TasmotaHandler) reconnectMQTT() {
	h.mqttMu.Lock()
	if h.mqttCancel != nil {
		h.mqttCancel()
	}
	conf := h.mqttConf
	ctx, cancel := context.WithCancel(context.Background())
	h.mqttCancel = cancel
	h.mqttMu.Unlock()

	if conf.Host == "" || conf.Port == "" || conf.Namespace == "" || conf.Slug == "" {
		return
	}

	brokerURL, err := url.Parse(fmt.Sprintf("tcp://%s:%s", conf.Host, conf.Port))
	if err != nil {
		log.Printf("mqtt: bad broker URL: %v", err)
		return
	}

	lwtTopic := fmt.Sprintf("evcc/%s/tele/%s/LWT", conf.Namespace, conf.Slug)
	cmndTopic := fmt.Sprintf("evcc/%s/cmnd/%s/POWER", conf.Namespace, conf.Slug)

	ccfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{brokerURL},
		KeepAlive:         30,
		ConnectRetryDelay: 2 * time.Second,
		WillMessage: &pahopkg.WillMessage{
			Retain:  true,
			QoS:     1,
			Topic:   lwtTopic,
			Payload: []byte("Offline"),
		},
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *pahopkg.Connack) {
			h.mqttMu.Lock()
			h.mqttConn = cm
			h.mqttMu.Unlock()

			_, _ = cm.Publish(ctx, &pahopkg.Publish{
				Topic:   lwtTopic,
				QoS:     1,
				Retain:  true,
				Payload: []byte("Online"),
			})
			_, _ = cm.Subscribe(ctx, &pahopkg.Subscribe{
				Subscriptions: []pahopkg.SubscribeOptions{{Topic: cmndTopic, QoS: 1}},
			})
			log.Printf("mqtt: connected as %s, namespace=%s slug=%s", conf.Username, conf.Namespace, conf.Slug)
			go h.publishSensorLoop(ctx, cm, conf.Namespace, conf.Slug)
		},
		OnConnectError: func(err error) {
			log.Printf("mqtt: connect error: %v", err)
		},
		ClientConfig: pahopkg.ClientConfig{
			ClientID: "mock-tasmota-" + conf.Slug,
			OnPublishReceived: []func(pahopkg.PublishReceived) (bool, error){
				func(pr pahopkg.PublishReceived) (bool, error) {
					h.handleMQTTPower(ctx, pr.Packet.Topic, pr.Packet.Payload)
					return true, nil
				},
			},
		},
	}
	if conf.Username != "" {
		ccfg.ConnectUsername = conf.Username
		ccfg.ConnectPassword = []byte(conf.Password)
	}

	cm, err := autopaho.NewConnection(ctx, ccfg)
	if err != nil {
		log.Printf("mqtt: failed to create connection: %v", err)
		return
	}
	h.mqttMu.Lock()
	h.mqttConn = cm
	h.mqttMu.Unlock()
}

// handleMQTTPower processes cmnd/<slug>/POWER messages.
func (h *TasmotaHandler) handleMQTTPower(ctx context.Context, topic string, payload []byte) {
	h.mqttMu.RLock()
	conf := h.mqttConf
	h.mqttMu.RUnlock()

	cmd := strings.ToUpper(strings.TrimSpace(string(payload)))
	log.Printf("mqtt: received cmnd/POWER %q on topic %s", cmd, topic)

	h.mu.Lock()
	switch cmd {
	case "ON":
		h.powerState = true
		h.startTime = time.Now()
		h.energyData.Power = h.maxPowerWatts
		h.energyData.Current = h.maxPowerWatts / h.voltage
	case "OFF":
		h.powerState = false
		h.energyData.Power = 0
		h.energyData.Current = 0
	}
	state := h.getPowerState()
	h.lastUpdate = time.Now()
	h.mu.Unlock()

	h.mqttMu.RLock()
	cm := h.mqttConn
	h.mqttMu.RUnlock()
	if cm == nil {
		log.Printf("mqtt: cannot publish stat/POWER, no MQTT connection")
		return
	}

	statTopic := fmt.Sprintf("evcc/%s/stat/%s/POWER", conf.Namespace, conf.Slug)
	log.Printf("mqtt: publishing stat/POWER %q to %s", state, statTopic)
	// Publish as retained so that late subscribers receive the current power state.
	resp, err := cm.Publish(ctx, &pahopkg.Publish{Topic: statTopic, QoS: 1, Retain: true, Payload: []byte(state)})
	if err != nil {
		log.Printf("mqtt: stat/POWER: failed to publish: %v", err)
	} else if resp != nil && resp.ReasonCode != 0 {
		log.Printf("mqtt: stat/POWER: publish rejected, reason=%d", resp.ReasonCode)
	} else {
		log.Printf("mqtt: stat/POWER published successfully to %s", statTopic)
	}
}

// publishSensorLoop publishes tele/SENSOR every 5 seconds while ctx is active.
func (h *TasmotaHandler) publishSensorLoop(ctx context.Context, cm *autopaho.ConnectionManager, ns, slug string) {
	topic := fmt.Sprintf("evcc/%s/tele/%s/SENSOR", ns, slug)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	h.publishSensor(ctx, cm, topic)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.publishSensor(ctx, cm, topic)
		}
	}
}

func (h *TasmotaHandler) publishSensor(ctx context.Context, cm *autopaho.ConnectionManager, topic string) {
	h.mu.Lock()
	if h.energyData.Power > 0 {
		elapsed := time.Since(h.lastUpdate).Hours()
		kwh := (h.energyData.Power * elapsed) / 1000
		h.energyData.Total += kwh
		h.energyData.Today += kwh
		h.lastUpdate = time.Now()
	}
	energy := h.energyData
	h.mu.Unlock()

	factor := 0.70
	apparent := math.Round(energy.Power / factor)
	reactive := 0.0
	if apparent > energy.Power {
		reactive = math.Sqrt(apparent*apparent - energy.Power*energy.Power)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"Time": time.Now().Format(time.RFC3339),
		"ENERGY": map[string]interface{}{
			"TotalStartTime": "2024-03-19T13:49:14",
			"Total":          energy.Total,
			"Yesterday":      energy.Yesterday / 1000,
			"Today":          energy.Today,
			"Power":          energy.Power,
			"ApparentPower":  apparent,
			"ReactivePower":  reactive,
			"Factor":         factor,
			"Voltage":        energy.Voltage,
			"Current":        energy.Current,
		},
	})
	if err != nil {
		return
	}
	_, err = cm.Publish(ctx, &pahopkg.Publish{Topic: topic, QoS: 0, Payload: payload})
	if err != nil {
		log.Printf("mqtt: SENSOR publish failed on %s: %v", topic, err)
	} else {
		log.Printf("mqtt: SENSOR published to %s (power=%.0fW, total=%.3fkWh)", topic, energy.Power, energy.Total)
	}
}

// publishPowerState publishes stat/POWER if MQTT is connected.
func (h *TasmotaHandler) publishPowerState(state string) {
	h.mqttMu.RLock()
	conf := h.mqttConf
	cm := h.mqttConn
	h.mqttMu.RUnlock()
	if conf.Namespace == "" || conf.Slug == "" || cm == nil {
		return
	}
	topic := fmt.Sprintf("evcc/%s/stat/%s/POWER", conf.Namespace, conf.Slug)
	log.Printf("mqtt: publishing stat/POWER %q to %s (HTTP trigger)", state, topic)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Publish as retained so that late subscribers receive the current power state.
	resp, err := cm.Publish(ctx, &pahopkg.Publish{Topic: topic, QoS: 1, Retain: true, Payload: []byte(state)})
	if err != nil {
		log.Printf("mqtt: stat/POWER: failed to publish: %v", err)
	} else if resp != nil && resp.ReasonCode != 0 {
		log.Printf("mqtt: stat/POWER: publish rejected, reason=%d", resp.ReasonCode)
	} else {
		log.Printf("mqtt: stat/POWER published successfully to %s", topic)
	}
}

// saveMQTTConfig writes the current MQTT config to disk if dirty.
func (h *TasmotaHandler) saveMQTTConfig() {
	h.mqttMu.Lock()
	defer h.mqttMu.Unlock()
	if !h.mqttConfDirty || h.mqttStateFile == "" {
		return
	}
	data, err := json.Marshal(h.mqttConf)
	if err != nil {
		log.Printf("mqtt: failed to marshal config: %v", err)
		return
	}
	if err := os.WriteFile(h.mqttStateFile, data, 0644); err != nil {
		log.Printf("mqtt: failed to write state file %s: %v", h.mqttStateFile, err)
		return
	}
	h.mqttConfDirty = false
	log.Printf("mqtt: persisted config to %s", h.mqttStateFile)
}

// loadMQTTConfig reads persisted MQTT config from disk.
func (h *TasmotaHandler) loadMQTTConfig() bool {
	if h.mqttStateFile == "" {
		return false
	}
	data, err := os.ReadFile(h.mqttStateFile)
	if err != nil {
		log.Printf("mqtt: no persisted config at %s: %v", h.mqttStateFile, err)
		return false
	}
	var conf mqttConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		log.Printf("mqtt: failed to parse state file %s: %v", h.mqttStateFile, err)
		return false
	}
	h.mqttMu.Lock()
	h.mqttConf = conf
	h.mqttMu.Unlock()
	log.Printf("mqtt: loaded persisted config: host=%s namespace=%s slug=%s", conf.Host, conf.Namespace, conf.Slug)
	return true
}
