package mqtt

import (
	"context"
	"testing"
	"time"

	"ev-charge-controller/api/models"

	pahopkg "github.com/eclipse/paho.golang/paho"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPahoPublisher struct {
	published []pahopkg.Publish
	pubErr    error
}

func (m *mockPahoPublisher) Publish(_ context.Context, p *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
	m.published = append(m.published, *p)
	return nil, m.pubErr
}

type mockPlugLookup struct {
	namespace string
	slug      string
	err       error
}

func (m *mockPlugLookup) NamespaceAndSlug(_ context.Context, _ string) (string, string, error) {
	return m.namespace, m.slug, m.err
}

func TestNewPublisher(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{}
	publisher := NewPublisher(client, plugs)
	require.NotNil(t, publisher)
	assert.Equal(t, client, publisher.client)
	assert.Equal(t, plugs, publisher.plugs)
}

func TestPublisher_SetPower_On(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	err := publisher.SetPower(context.Background(), "plug-id-1", true)
	require.NoError(t, err)
	assert.Len(t, client.published, 1)
	assert.Equal(t, "evcc/ns1/cmnd/plug1/POWER", client.published[0].Topic)
	assert.Equal(t, byte(1), client.published[0].QoS)
	assert.Equal(t, "ON", string(client.published[0].Payload))
}

func TestPublisher_SetPower_Off(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	err := publisher.SetPower(context.Background(), "plug-id-1", false)
	require.NoError(t, err)
	assert.Len(t, client.published, 1)
	assert.Equal(t, "evcc/ns1/cmnd/plug1/POWER", client.published[0].Topic)
	assert.Equal(t, "OFF", string(client.published[0].Payload))
}

func TestPublisher_SetPower_LookupError(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{err: assert.AnError}
	publisher := NewPublisher(client, plugs)

	err := publisher.SetPower(context.Background(), "plug-id-1", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher: lookup plug")
	assert.Len(t, client.published, 0)
}

func TestPublisher_SetPower_PublishError(t *testing.T) {
	client := &mockPahoPublisher{pubErr: assert.AnError}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	err := publisher.SetPower(context.Background(), "plug-id-1", true)
	assert.Error(t, err)
}

func TestPublisher_PublishCommand(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{}
	publisher := NewPublisher(client, plugs)

	err := publisher.PublishCommand(context.Background(), "ns1", "plug1", "BACKLOG", "Delay 1000")
	require.NoError(t, err)
	assert.Len(t, client.published, 1)
	assert.Equal(t, "evcc/ns1/cmnd/plug1/BACKLOG", client.published[0].Topic)
	assert.Equal(t, byte(1), client.published[0].QoS)
	assert.Equal(t, "Delay 1000", string(client.published[0].Payload))
}

func TestPublisher_PublishCommand_Error(t *testing.T) {
	client := &mockPahoPublisher{pubErr: assert.AnError}
	plugs := &mockPlugLookup{}
	publisher := NewPublisher(client, plugs)

	err := publisher.PublishCommand(context.Background(), "ns1", "plug1", "POWER", "ON")
	assert.Error(t, err)
}

func TestNewRepoPlugLookup(t *testing.T) {
	repo := &mockPlugRepoCache{plugs: make(map[string]*models.Plug)}
	lookup := NewRepoPlugLookup(repo)
	require.NotNil(t, lookup)
}

func TestRepoPlugLookup_NamespaceAndSlug_Success(t *testing.T) {
	repo := &mockPlugRepoCache{
		plugs: map[string]*models.Plug{
			"plug-id-1": {ID: "plug-id-1", Namespace: "ns1", MqttTopic: "plug1"},
		},
	}
	lookup := NewRepoPlugLookup(repo)

	ns, slug, err := lookup.NamespaceAndSlug(context.Background(), "plug-id-1")
	require.NoError(t, err)
	assert.Equal(t, "ns1", ns)
	assert.Equal(t, "plug1", slug)
}

func TestRepoPlugLookup_NamespaceAndSlug_NotFound(t *testing.T) {
	repo := &mockPlugRepoCache{plugs: make(map[string]*models.Plug)}
	lookup := NewRepoPlugLookup(repo)

	ns, slug, err := lookup.NamespaceAndSlug(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Empty(t, ns)
	assert.Empty(t, slug)
}

func TestRepoPlugLookup_NamespaceAndSlug_DBError(t *testing.T) {
	repo := &mockPlugRepoCache{
		plugs: make(map[string]*models.Plug),
		err:   assert.AnError,
	}
	lookup := NewRepoPlugLookup(repo)

	ns, slug, err := lookup.NamespaceAndSlug(context.Background(), "plug-id-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugLookup: find plug")
	assert.Empty(t, ns)
	assert.Empty(t, slug)
}

func TestPublisher_SetPowerAndWait_Confirms(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns1", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	// Simulate stat/POWER response arriving shortly after command
	go func() {
		time.Sleep(10 * time.Millisecond)
		dispatcher.Dispatch(context.Background(), "evcc/ns1/stat/plug1/POWER", []byte("ON"), false)
	}()

	confirmed, err := publisher.SetPowerAndWait(context.Background(), "plug-id-1", true, dispatcher, 2*time.Second)
	require.NoError(t, err)
	assert.True(t, confirmed)
	assert.Len(t, client.published, 1)
}

func TestPublisher_SetPowerAndWait_Timeout(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns1", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	// No stat/POWER response - should timeout
	confirmed, err := publisher.SetPowerAndWait(context.Background(), "plug-id-1", true, dispatcher, 50*time.Millisecond)
	assert.ErrorIs(t, err, ErrPowerConfirmationTimeout)
	assert.False(t, confirmed)
	assert.Len(t, client.published, 1)
}

func TestPublisher_SetPowerAndWait_ContextCancelled(t *testing.T) {
	client := &mockPahoPublisher{}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns1", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	confirmed, err := publisher.SetPowerAndWait(ctx, "plug-id-1", true, dispatcher, 2*time.Second)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, confirmed)
}

func TestPublisher_SetPowerAndWait_PublishError(t *testing.T) {
	client := &mockPahoPublisher{pubErr: assert.AnError}
	plugs := &mockPlugLookup{namespace: "ns1", slug: "plug1"}
	publisher := NewPublisher(client, plugs)

	plugCache := NewStaticPlugCache(map[NamespaceSlug]string{
		{Namespace: "ns1", Slug: "plug1"}: "plug-id-1",
	})
	dispatcher := NewDispatcher(plugCache, nil, nil, nil)

	confirmed, err := publisher.SetPowerAndWait(context.Background(), "plug-id-1", true, dispatcher, 2*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher: set power")
	assert.False(t, confirmed)
}
