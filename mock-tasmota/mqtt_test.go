package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	pahopkg "github.com/eclipse/paho.golang/paho"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
)

// startTestBroker starts an embedded Mochi MQTT broker on a free port and
// returns the tcp:// URL. The broker is stopped when t cleans up.
func startTestBroker(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	srv := mochi.New(nil)
	require.NoError(t, srv.AddHook(new(auth.AllowHook), nil))
	tcp := listeners.NewTCP(listeners.Config{ID: "test", Address: fmt.Sprintf(":%d", port)})
	require.NoError(t, srv.AddListener(tcp))
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })
	return fmt.Sprintf("tcp://127.0.0.1:%d", port)
}

// subscribeAndCollect connects a test MQTT client to brokerURL, subscribes to
// topic, and returns a channel that receives every matching payload.
func subscribeAndCollect(t *testing.T, brokerURL, topic string) <-chan []byte {
	t.Helper()
	ch := make(chan []byte, 32)
	u, err := url.Parse(brokerURL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{u},
		KeepAlive:         5,
		ConnectRetryDelay: 100 * time.Millisecond,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *pahopkg.Connack) {
			_, _ = cm.Subscribe(ctx, &pahopkg.Subscribe{
				Subscriptions: []pahopkg.SubscribeOptions{{Topic: topic, QoS: 0}},
			})
		},
		ClientConfig: pahopkg.ClientConfig{
			ClientID: "test-collector-" + topic,
			OnPublishReceived: []func(pahopkg.PublishReceived) (bool, error){
				func(pr pahopkg.PublishReceived) (bool, error) {
					payload := make([]byte, len(pr.Packet.Payload))
					copy(payload, pr.Packet.Payload)
					ch <- payload
					return true, nil
				},
			},
		},
	}
	cm, err := autopaho.NewConnection(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, cm.AwaitConnection(ctx))
	return ch
}

// configureMQTT sends the same HTTP commands ConfigureTasmotaDevice would send.
func configureMQTT(t *testing.T, srvURL, brokerHost, brokerPort, user, password, fullTopic, slug string) {
	t.Helper()
	cmds := []string{
		"MQTTHost " + brokerHost,
		"MQTTPort " + brokerPort,
		"MQTTUser " + user,
		"MQTTPassword " + password,
		"FullTopic " + fullTopic,
		"Topic " + slug,
		"Restart 1",
	}
	for _, cmd := range cmds {
		resp, err := http.Get(srvURL + "/cm?cmnd=" + url.QueryEscape(cmd))
		require.NoError(t, err)
		_ = resp.Body.Close()
	}
}

func TestTasmotaHandler_MQTTConfigCommands(t *testing.T) {
	h := &TasmotaHandler{maxPowerWatts: 600, voltage: 230, frequency: 50}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cmds := map[string]string{
		"MQTTHost 192.168.1.10":         "",
		"MQTTPort 1883":                 "",
		"MQTTUser plug-ns-abc":          "",
		"MQTTPassword secretpass":       "",
		"FullTopic evcc/ns-abc/%prefix%/%topic%/": "",
		"Topic garage":                  "",
	}
	for cmd := range cmds {
		resp, err := http.Get(srv.URL + "/cm?cmnd=" + url.QueryEscape(cmd))
		require.NoError(t, err, "cmd=%s", cmd)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "cmd=%s", cmd)
		_ = resp.Body.Close()
	}

	h.mqttMu.RLock()
	defer h.mqttMu.RUnlock()
	assert.Equal(t, "192.168.1.10", h.mqttConf.Host)
	assert.Equal(t, "1883", h.mqttConf.Port)
	assert.Equal(t, "plug-ns-abc", h.mqttConf.Username)
	assert.Equal(t, "secretpass", h.mqttConf.Password)
	assert.Equal(t, "ns-abc", h.mqttConf.Namespace)
	assert.Equal(t, "garage", h.mqttConf.Slug)
}

func TestTasmotaHandler_MQTTConnect_PublishesOnlineAndSensor(t *testing.T) {
	brokerURL := startTestBroker(t)

	h := &TasmotaHandler{maxPowerWatts: 600, voltage: 230, frequency: 50}
	h.energyData = EnergyData{Total: 1.5, Voltage: 230, Freq: 50}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Cleanup(h.Close)

	// Subscribe to LWT and SENSOR before configuring the mock.
	u, err := url.Parse(brokerURL)
	require.NoError(t, err)
	host := u.Hostname()
	port := u.Port()

	lwtCh := subscribeAndCollect(t, brokerURL, "evcc/ns-test/tele/garage/LWT")
	sensorCh := subscribeAndCollect(t, brokerURL, "evcc/ns-test/tele/garage/SENSOR")

	configureMQTT(t, srv.URL, host, port, "user1", "pass1",
		"evcc/ns-test/%prefix%/%topic%/", "garage")

	// Wait for LWT Online.
	select {
	case lwt := <-lwtCh:
		assert.Equal(t, "Online", string(lwt))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for LWT Online")
	}

	// Turn power on so SENSOR carries non-zero data.
	resp, err := http.Get(srv.URL + "/cm?cmnd=" + url.QueryEscape("Power ON"))
	require.NoError(t, err)
	_ = resp.Body.Close()

	// Wait for a SENSOR with non-zero Power - the first publish on connect may be zero.
	deadline := time.After(10 * time.Second)
	for {
		select {
		case payload := <-sensorCh:
			var msg struct {
				ENERGY struct {
					Power   float64 `json:"Power"`
					Voltage float64 `json:"Voltage"`
				} `json:"ENERGY"`
			}
			require.NoError(t, json.Unmarshal(payload, &msg))
			if msg.ENERGY.Power > 0 {
				assert.Equal(t, 230.0, msg.ENERGY.Voltage)
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for SENSOR message with Power > 0")
		}
	}
}

func TestTasmotaHandler_MQTTCommand_PowerOnOff(t *testing.T) {
	brokerURL := startTestBroker(t)

	h := &TasmotaHandler{maxPowerWatts: 600, voltage: 230, frequency: 50}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Cleanup(h.Close)

	u, err := url.Parse(brokerURL)
	require.NoError(t, err)
	host, port := u.Hostname(), u.Port()

	statCh := subscribeAndCollect(t, brokerURL, "evcc/ns-cmd/stat/plug1/POWER")

	configureMQTT(t, srv.URL, host, port, "u1", "p1",
		"evcc/ns-cmd/%prefix%/%topic%/", "plug1")

	// Wait for MQTT to come up (LWT Online).
	lwtCh := subscribeAndCollect(t, brokerURL, "evcc/ns-cmd/tele/plug1/LWT")
	select {
	case <-lwtCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for MQTT connect")
	}

	// Publish Power ON via MQTT.
	mqttURL, _ := url.Parse(brokerURL)
	conn, err := net.Dial("tcp", mqttURL.Host)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	client := pahopkg.NewClient(pahopkg.ClientConfig{
		Conn:     conn,
		ClientID: "test-commander",
		OnPublishReceived: []func(pahopkg.PublishReceived) (bool, error){
			func(_ pahopkg.PublishReceived) (bool, error) { return true, nil },
		},
		OnClientError: func(err error) { wg.Done() },
	})
	_, err = client.Connect(context.Background(), &pahopkg.Connect{ClientID: "test-commander", KeepAlive: 5})
	require.NoError(t, err)

	_, err = client.Publish(context.Background(), &pahopkg.Publish{
		Topic:   "evcc/ns-cmd/cmnd/plug1/POWER",
		QoS:     0,
		Payload: []byte("ON"),
	})
	require.NoError(t, err)

	// Verify stat/POWER response.
	select {
	case stat := <-statCh:
		assert.Equal(t, "ON", string(stat))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stat/POWER")
	}

	h.mu.RLock()
	assert.True(t, h.powerState)
	h.mu.RUnlock()
}
