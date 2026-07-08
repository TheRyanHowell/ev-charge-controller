package mqtt_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ev-charge-controller/api/mqtt"
	"ev-charge-controller/api/mqtt/mqtttest"
	"ev-charge-controller/api/tasmota"

	pahopkg "github.com/eclipse/paho.golang/paho"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSENSOR returns a minimal Tasmota SENSOR JSON payload.
func buildSENSOR(powerW, totalKwh float64) []byte {
	p := map[string]any{
		"Time": "2024-01-01T12:00:00",
		"ENERGY": map[string]any{
			"Total":         totalKwh,
			"Yesterday":     0.0,
			"Today":         0.0,
			"Power":         powerW,
			"ApparentPower": powerW,
			"ReactivePower": 0.0,
			"Factor":        1.0,
			"Voltage":       230.0,
			"Current":       powerW / 230.0,
		},
	}
	b, _ := json.Marshal(p)
	return b
}

// newTestPlugCache builds a PlugCache with a pre-seeded namespace→plugID mapping.
func newTestPlugCache(namespace, slug, plugID string) *mqtt.PlugCache {
	return mqtt.NewStaticPlugCache(map[mqtt.NamespaceSlug]string{
		{Namespace: namespace, Slug: slug}: plugID,
	})
}

// --- Dispatcher tests ---

func TestDispatcher_SENSOR_UpdatesEnergyCache(t *testing.T) {
	const (
		namespace = "ns-test1"
		slug      = "myplug"
		plugID    = "plug-id-1"
	)

	var called atomic.Bool
	var gotEnergy *tasmota.EnergyData
	var mu sync.Mutex

	handler := func(_ context.Context, pid string, energy *tasmota.EnergyData) {
		assert.Equal(t, plugID, pid)
		mu.Lock()
		gotEnergy = energy
		mu.Unlock()
		called.Store(true)
	}

	cache := newTestPlugCache(namespace, slug, plugID)
	d := mqtt.NewDispatcher(cache, handler, nil, nil)

	topic := fmt.Sprintf("evcc/%s/tele/%s/SENSOR", namespace, slug)
	d.Dispatch(context.Background(), topic, buildSENSOR(1500, 1.234), false)

	assert.True(t, called.Load())
	mu.Lock()
	assert.NotNil(t, gotEnergy)
	assert.InDelta(t, 1.234, gotEnergy.Total, 0.0001)
	assert.InDelta(t, 1500.0, gotEnergy.Power, 0.0001)
	mu.Unlock()

	// Cache should be updated
	cached := d.LastEnergy(plugID)
	require.NotNil(t, cached)
	assert.InDelta(t, 1.234, cached.Total, 0.0001)
}

func TestDispatcher_UnknownPlug_Ignored(t *testing.T) {
	var called atomic.Bool
	cache := newTestPlugCache("ns-known", "knownslug", "plug-1")
	d := mqtt.NewDispatcher(cache, func(_ context.Context, _ string, _ *tasmota.EnergyData) {
		called.Store(true)
	}, nil, nil)

	d.Dispatch(context.Background(), "evcc/ns-unknown/tele/unknownslug/SENSOR", buildSENSOR(100, 0.5), false)
	assert.False(t, called.Load())
}

func TestDispatcher_NonSENSOR_Ignored(t *testing.T) {
	var called atomic.Bool
	cache := newTestPlugCache("ns-x", "plug", "pid")
	d := mqtt.NewDispatcher(cache, func(_ context.Context, _ string, _ *tasmota.EnergyData) {
		called.Store(true)
	}, nil, nil)
	d.Dispatch(context.Background(), "evcc/ns-x/tele/plug/LWT", []byte("Online"), false)
	d.Dispatch(context.Background(), "evcc/ns-x/stat/plug/POWER", []byte("ON"), false)
	assert.False(t, called.Load())
}

func TestDispatcher_Race_TwoPlugs(t *testing.T) {
	// Interleave SENSOR messages for two plugs and verify no data races.
	cache := mqtt.NewStaticPlugCache(map[mqtt.NamespaceSlug]string{
		{Namespace: "ns-a", Slug: "plug-a"}: "plug-id-a",
		{Namespace: "ns-b", Slug: "plug-b"}: "plug-id-b",
	})

	var wg sync.WaitGroup
	var counts [2]atomic.Int64
	d := mqtt.NewDispatcher(cache, func(_ context.Context, plugID string, _ *tasmota.EnergyData) {
		if plugID == "plug-id-a" {
			counts[0].Add(1)
		} else {
			counts[1].Add(1)
		}
		wg.Done()
	}, nil, nil)

	const msgs = 50
	wg.Add(msgs * 2)
	for i := 0; i < msgs; i++ {
		go d.Dispatch(context.Background(), "evcc/ns-a/tele/plug-a/SENSOR", buildSENSOR(float64(i), float64(i)/100), false)
		go d.Dispatch(context.Background(), "evcc/ns-b/tele/plug-b/SENSOR", buildSENSOR(float64(i), float64(i)/100), false)
	}
	wg.Wait()

	assert.Equal(t, int64(msgs), counts[0].Load())
	assert.Equal(t, int64(msgs), counts[1].Load())
}

func TestPlugCache_Invalidate_ClearsEntry(t *testing.T) {
	cache := mqtt.NewStaticPlugCache(map[mqtt.NamespaceSlug]string{
		{Namespace: "ns-inv", Slug: "plug"}: "plug-id-inv",
	})

	id, ok := cache.Lookup("ns-inv", "plug")
	require.True(t, ok)
	assert.Equal(t, "plug-id-inv", id)

	cache.Invalidate("ns-inv", "plug")
	_, ok = cache.Lookup("ns-inv", "plug")
	assert.False(t, ok, "entry should be gone after Invalidate (no DB to reload from)")
}

// --- Publisher tests via embedded broker ---

// subscribeAndAwait opens an autopaho connection subscribed to topic and
// blocks until the subscription is acknowledged by the broker.
//
// autopaho's AwaitConnection only waits for the transport connection to come
// up; it returns before OnConnectionUp (where the subscribe happens) has even
// run, let alone completed. A caller that publishes right after AwaitConnection
// races the broker's SUBACK: most of the time the subscribe wins, but under
// load (e.g. the extra scheduling overhead of `go test -race`) the publish can
// occasionally land first and the message is silently dropped, since MQTT
// does not redeliver to a subscriber that wasn't yet registered. Waiting for
// an explicit "subscribed" signal (closed only once cm.Subscribe returns
// successfully) closes that window.
func subscribeAndAwait(t *testing.T, ctx context.Context, brokerURL *url.URL, clientID, topic string, received chan<- string) *autopaho.ConnectionManager {
	t.Helper()

	subscribed := make(chan struct{})
	conn, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		BrokerUrls: []*url.URL{brokerURL},
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *pahopkg.Connack) {
			if _, err := cm.Subscribe(ctx, &pahopkg.Subscribe{
				Subscriptions: []pahopkg.SubscribeOptions{{Topic: topic, QoS: 1}},
			}); err == nil {
				close(subscribed)
			}
		},
		ClientConfig: pahopkg.ClientConfig{
			ClientID: clientID,
			OnPublishReceived: []func(pahopkg.PublishReceived) (bool, error){
				func(pr pahopkg.PublishReceived) (bool, error) {
					received <- pr.Packet.Topic
					return true, nil
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, conn.AwaitConnection(ctx))
	t.Cleanup(func() { _ = conn.Disconnect(context.Background()) })

	select {
	case <-subscribed:
	case <-ctx.Done():
		t.Fatal("timed out waiting for subscription to be established")
	}

	return conn
}

func TestPublisher_SetPower_PublishesToCorrectTopic(t *testing.T) {
	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, err := url.Parse(brokerURL)
	require.NoError(t, err)

	received := make(chan string, 1)
	subscribeAndAwait(t, ctx, u, "test-sub", "evcc/#", received)

	pubConn, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		BrokerUrls:   []*url.URL{u},
		ClientConfig: pahopkg.ClientConfig{ClientID: "test-pub"},
	})
	require.NoError(t, err)
	require.NoError(t, pubConn.AwaitConnection(ctx))
	t.Cleanup(func() { _ = pubConn.Disconnect(context.Background()) })

	plugLookup := mqtt.NewStaticPlugLookup("ns-pub1", "driveway")
	pub := mqtt.NewPublisher(pubConn, plugLookup)

	require.NoError(t, pub.SetPower(ctx, "plug-id-pub1", true))

	select {
	case topic := <-received:
		assert.Equal(t, "evcc/ns-pub1/cmnd/driveway/POWER", topic)
	case <-ctx.Done():
		t.Fatal("timed out waiting for MQTT publish")
	}
}

func TestController_SetPower_AndLastEnergy(t *testing.T) {
	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u, err := url.Parse(brokerURL)
	require.NoError(t, err)

	received := make(chan string, 1)
	subscribeAndAwait(t, ctx, u, "ctrl-test-sub", "evcc/#", received)

	pubConn, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		BrokerUrls:   []*url.URL{u},
		ClientConfig: pahopkg.ClientConfig{ClientID: "ctrl-test-pub"},
	})
	require.NoError(t, err)
	require.NoError(t, pubConn.AwaitConnection(ctx))
	t.Cleanup(func() { _ = pubConn.Disconnect(context.Background()) })

	const plugID = "ctrl-plug-1"
	cache := mqtt.NewStaticPlugCache(map[mqtt.NamespaceSlug]string{
		{Namespace: "ns-ctrl", Slug: "garage"}: plugID,
	})
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	publisher := mqtt.NewPublisher(pubConn, mqtt.NewStaticPlugLookup("ns-ctrl", "garage"))
	ctrl := mqtt.NewController(dispatcher, publisher)

	// Seed energy via Dispatch so LastEnergy returns data.
	dispatcher.Dispatch(ctx, "evcc/ns-ctrl/tele/garage/SENSOR", buildSENSOR(2000, 5.0), false)
	energy := ctrl.LastEnergy(plugID)
	require.NotNil(t, energy)
	assert.InDelta(t, 2000.0, energy.Power, 0.001)

	// SetPower should publish to the correct topic.
	require.NoError(t, ctrl.SetPower(ctx, plugID, false))
	select {
	case topic := <-received:
		assert.Equal(t, "evcc/ns-ctrl/cmnd/garage/POWER", topic)
	case <-ctx.Done():
		t.Fatal("timed out waiting for MQTT publish from controller")
	}
}

func TestClient_AwaitConnection(t *testing.T) {
	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cache := mqtt.NewStaticPlugCache(nil)
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	client, err := mqtt.NewClient(ctx, mqtt.ClientConfig{BrokerURL: brokerURL}, dispatcher)
	require.NoError(t, err)
	require.NoError(t, client.AwaitConnection(ctx))
	client.Disconnect(context.Background())
}

// TestClient_AwaitConnection_SubscriptionActiveOnReturn guards against a race
// between the transport connecting and the client's wildcard subscription
// being acknowledged. autopaho signals "connection up" (what AwaitConnection
// waits on) before running OnConnectionUp, where Client subscribes - so a
// caller that publishes right after AwaitConnection returns can race the
// broker's SUBACK and have its message silently dropped. AwaitConnection must
// not return until the initial subscription is confirmed.
func TestClient_AwaitConnection_SubscriptionActiveOnReturn(t *testing.T) {
	const (
		namespace = "ns-ready"
		slug      = "plug"
		plugID    = "ready-plug-1"
	)

	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cache := newTestPlugCache(namespace, slug, plugID)
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	client, err := mqtt.NewClient(ctx, mqtt.ClientConfig{BrokerURL: brokerURL}, dispatcher)
	require.NoError(t, err)
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	require.NoError(t, client.AwaitConnection(ctx))

	// Publish a stat/POWER message from an independent connection immediately
	// after AwaitConnection returns. If the client's own subscription isn't
	// active yet, the broker has nobody to deliver it to and it's dropped
	// for good - waiting longer would not help, unlike a merely slow delivery.
	confirmCh := dispatcher.RegisterPowerConfirm(plugID)
	defer dispatcher.RemovePowerConfirm(plugID)

	u, err := url.Parse(brokerURL)
	require.NoError(t, err)
	pubConn, err := autopaho.NewConnection(ctx, autopaho.ClientConfig{
		BrokerUrls:   []*url.URL{u},
		ClientConfig: pahopkg.ClientConfig{ClientID: "ready-check-pub"},
	})
	require.NoError(t, err)
	require.NoError(t, pubConn.AwaitConnection(ctx))
	t.Cleanup(func() { _ = pubConn.Disconnect(context.Background()) })

	topic := fmt.Sprintf("evcc/%s/stat/%s/POWER", namespace, slug)
	_, err = pubConn.Publish(ctx, &pahopkg.Publish{Topic: topic, QoS: 1, Payload: []byte("ON")})
	require.NoError(t, err)

	select {
	case <-confirmCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for dispatcher to observe stat/POWER published right after AwaitConnection returned")
	}
}

func TestClient_NewClient_InvalidURL(t *testing.T) {
	cache := mqtt.NewStaticPlugCache(nil)
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	_, err := mqtt.NewClient(context.Background(), mqtt.ClientConfig{
		BrokerURL: "://not-a-valid-url",
	}, dispatcher)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse broker URL")
}

func TestClient_ConnectionManager_ReturnsManager(t *testing.T) {
	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cache := mqtt.NewStaticPlugCache(nil)
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	client, err := mqtt.NewClient(ctx, mqtt.ClientConfig{BrokerURL: brokerURL}, dispatcher)
	require.NoError(t, err)
	t.Cleanup(func() { client.Disconnect(context.Background()) })

	cm := client.ConnectionManager()
	assert.NotNil(t, cm)
}

func TestClient_NewClient_DefaultValues(t *testing.T) {
	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cache := mqtt.NewStaticPlugCache(nil)
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	// Omit ClientID and KeepAlive to verify defaults are applied
	client, err := mqtt.NewClient(ctx, mqtt.ClientConfig{BrokerURL: brokerURL}, dispatcher)
	require.NoError(t, err)
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	assert.NotNil(t, client)
}

func TestClient_NewClient_WithAuth(t *testing.T) {
	brokerURL := mqtttest.BrokerURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cache := mqtt.NewStaticPlugCache(nil)
	dispatcher := mqtt.NewDispatcher(cache, nil, nil, nil)
	client, err := mqtt.NewClient(ctx, mqtt.ClientConfig{
		BrokerURL: brokerURL,
		Username:  "test-user",
		Password:  "test-pass",
		ClientID:  "auth-test-client",
	}, dispatcher)
	require.NoError(t, err)
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	assert.NotNil(t, client)
}
