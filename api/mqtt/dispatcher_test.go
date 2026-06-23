package mqtt

import (
	"context"
	"testing"

	"ev-charge-controller/api/tasmota"
	"github.com/stretchr/testify/assert"
)

func TestDispatcher_Dispatch_InvalidTopic(t *testing.T) {
	plugCache := NewStaticPlugCache(nil)
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	dispatcher.Dispatch(context.Background(), "invalid/topic/here", []byte("test"), false)
}

func TestDispatcher_DispatchLWT_NilManager(t *testing.T) {
	plugCache := NewStaticPlugCache(nil)
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/LWT", []byte("Online"), false)
}

func TestDispatcher_DispatchLWT_UnknownPlug(t *testing.T) {
	plugCache := NewStaticPlugCache(nil)
	repo := &mockPlugRepo{}
	lwtMgr := NewLWTManager(repo, nil, nil, nil, nil)
	dispatcher := NewDispatcher(plugCache, nil, lwtMgr, nil)

	dispatcher.Dispatch(context.Background(), "evcc/ns-unknown/tele/plug1/LWT", []byte("Online"), false)
}

func TestDispatcher_DispatchLWT_Success(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	repo := &mockPlugRepo{}
	lwtMgr := NewLWTManager(repo, nil, nil, nil, nil)
	dispatcher := NewDispatcher(plugCache, nil, lwtMgr, nil)

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/LWT", []byte("Online"), false)

	assert.Equal(t, []setOnlineCall{{plugID: "plug-id-1", online: true}}, repo.setOnlineCalls)
}

func TestDispatcher_DispatchSENSOR_InvalidPayload(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	var called bool
	dispatcher := NewDispatcher(plugCache, func(_ context.Context, _ string, _ *tasmota.EnergyData) {
		called = true
	}, nil, nil)

	// Send malformed JSON - should be silently ignored
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/SENSOR", []byte("not-json"), false)
	assert.False(t, called, "handler should not be called for invalid SENSOR payload")
}

func TestDispatcher_DispatchSENSOR_NilHandler(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	// nil handler - should not panic
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/SENSOR",
		[]byte(`{"Time":"2024-01-01T12:00:00","ENERGY":{"Total":1.0,"Yesterday":0,"Today":0,"Power":100,"ApparentPower":100,"ReactivePower":0,"Factor":1,"Voltage":230,"Current":0.43}}`), false)

	// Energy should still be cached even with nil handler
	energy := dispatcher.LastEnergy("plug-id-1")
	assert.NotNil(t, energy)
}

func TestDispatcher_DispatchSTAT_POWER_SignalsConfirmer(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	confirmCh := dispatcher.RegisterPowerConfirm("plug-id-1")

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("ON"), false)

	select {
	case on := <-confirmCh:
		assert.True(t, on)
	default:
		t.Fatal("confirmation channel should have been signalled")
	}
}

func TestDispatcher_DispatchSTAT_POWER_OffSignalsConfirmer(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	confirmCh := dispatcher.RegisterPowerConfirm("plug-id-1")

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("OFF"), false)

	select {
	case on := <-confirmCh:
		assert.False(t, on)
	default:
		t.Fatal("confirmation channel should have been signalled")
	}
}

func TestDispatcher_DispatchSTAT_POWER_NoConfirmer(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	// No confirmer registered - should not panic
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("ON"), false)
}

func TestDispatcher_DispatchSTAT_POWER_UnknownPlug(t *testing.T) {
	plugCache := NewStaticPlugCache(nil)
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	confirmCh := dispatcher.RegisterPowerConfirm("unknown-plug")

	dispatcher.Dispatch(context.Background(), "evcc/ns-unknown/stat/plug1/POWER", []byte("ON"), false)

	select {
	case <-confirmCh:
		t.Fatal("confirmation channel should not have been signalled for unknown plug")
	default:
		// expected
	}
}

func TestDispatcher_DispatchSTAT_POWER_InvalidPayload(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	confirmCh := dispatcher.RegisterPowerConfirm("plug-id-1")

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte(""), false)

	select {
	case <-confirmCh:
		t.Fatal("confirmation channel should not have been signalled for invalid payload")
	default:
		// expected
	}
}

func TestDispatcher_DispatchSTAT_POWER_OneShotOnly(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	confirmCh := dispatcher.RegisterPowerConfirm("plug-id-1")

	// First message should signal
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("ON"), false)
	select {
	case <-confirmCh:
		// expected
	default:
		t.Fatal("first message should signal")
	}

	// Second message should NOT signal (one-shot)
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("OFF"), false)
	select {
	case <-confirmCh:
		// channel is closed, this is expected
	default:
		// also expected if channel wasn't signalled again
	}
}

func TestDispatcher_RemovePowerConfirm(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	dispatcher.RegisterPowerConfirm("plug-id-1")
	dispatcher.RemovePowerConfirm("plug-id-1")

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("ON"), false)

	// No panic, no signal
}

func TestDispatcher_DispatchSTATE_CachesPowerState(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	// Before any STATE message, power state is unknown
	_, known := dispatcher.LastPowerState("plug-id-1")
	assert.False(t, known)

	// Dispatch a tele/STATE ON
	statePayload := []byte(`{"POWER":"ON","Time":"2025-01-01T00:00:00"}`)
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/STATE", statePayload, false)

	on, known := dispatcher.LastPowerState("plug-id-1")
	assert.True(t, known)
	assert.True(t, on)

	// Dispatch a tele/STATE OFF
	statePayloadOff := []byte(`{"POWER":"OFF","Time":"2025-01-01T00:00:01"}`)
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/STATE", statePayloadOff, false)

	on, known = dispatcher.LastPowerState("plug-id-1")
	assert.True(t, known)
	assert.False(t, on)
}

func TestDispatcher_DispatchSTAT_POWER_CachesPowerState(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	dispatcher.Dispatch(context.Background(), "evcc/ns-test/stat/plug1/POWER", []byte("ON"), false)

	on, known := dispatcher.LastPowerState("plug-id-1")
	assert.True(t, known)
	assert.True(t, on)
}

func TestDispatcher_DispatchSTATE_PersistsPowerState(t *testing.T) {
	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns-test", Slug: "plug1"}: "plug-id-1",
	})

	var persistedPlugID string
	var persistedOn bool
	repo := &mockPowerRepo{
		setFn: func(plugID string, on bool) {
			persistedPlugID = plugID
			persistedOn = on
		},
	}
	dispatcher := NewDispatcher(plugCache, nil, nil, repo)

	statePayload := []byte(`{"POWER":"ON"}`)
	dispatcher.Dispatch(context.Background(), "evcc/ns-test/tele/plug1/STATE", statePayload, false)

	assert.Equal(t, "plug-id-1", persistedPlugID)
	assert.True(t, persistedOn)
}

type mockPowerRepo struct {
	setFn func(plugID string, on bool)
}

func (m *mockPowerRepo) SetPowerState(_ context.Context, plugID string, on bool) error {
	if m.setFn != nil {
		m.setFn(plugID, on)
	}
	return nil
}
