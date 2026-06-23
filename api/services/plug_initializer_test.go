package services

import (
	"context"
	"fmt"
	"testing"

	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type mockInitializerRepo struct {
	plug           *models.Plug
	findErr        error
	setInitialized bool
	setInitErr     error
}

func (m *mockInitializerRepo) FindByID(_ context.Context, _ string) (*models.Plug, error) {
	return m.plug, m.findErr
}

func (m *mockInitializerRepo) SetInitialized(_ context.Context, _ string) error {
	m.setInitialized = true
	return m.setInitErr
}

type mockCommandPublisher struct {
	published []publishedCmd
	err       error
}

type publishedCmd struct {
	namespace, slug, cmd, payload string
}

func (m *mockCommandPublisher) PublishCommand(_ context.Context, namespace, slug, cmd, payload string) error {
	m.published = append(m.published, publishedCmd{namespace, slug, cmd, payload})
	return m.err
}

// --- Tests ---

func TestPlugInitializerService_OnPlugOnline_FirstTime(t *testing.T) {
	plug := &models.Plug{
		ID:          "plug-1",
		Namespace:   "evcc-ns",
		MqttTopic:   "my-plug",
		Initialized: false,
	}
	repo := &mockInitializerRepo{plug: plug}
	pub := &mockCommandPublisher{}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "plug-1")
	require.NoError(t, err)

	// Should publish all init commands
	assert.Len(t, pub.published, len(initCommands))
	for i, cmdPair := range initCommands {
		assert.Equal(t, "evcc-ns", pub.published[i].namespace)
		assert.Equal(t, "my-plug", pub.published[i].slug)
		assert.Equal(t, cmdPair[0], pub.published[i].cmd)
		assert.Equal(t, cmdPair[1], pub.published[i].payload)
	}

	// Should mark initialized
	assert.True(t, repo.setInitialized)
}

func TestPlugInitializerService_OnPlugOnline_AlreadyInitialized(t *testing.T) {
	plug := &models.Plug{
		ID:          "plug-2",
		Initialized: true,
	}
	repo := &mockInitializerRepo{plug: plug}
	pub := &mockCommandPublisher{}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "plug-2")
	require.NoError(t, err)

	// No commands published; no SetInitialized call
	assert.Empty(t, pub.published)
	assert.False(t, repo.setInitialized)
}

func TestPlugInitializerService_OnPlugOnline_PlugNotFound(t *testing.T) {
	repo := &mockInitializerRepo{plug: nil}
	pub := &mockCommandPublisher{}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Empty(t, pub.published)
}

func TestPlugInitializerService_OnPlugOnline_FindError(t *testing.T) {
	repo := &mockInitializerRepo{findErr: fmt.Errorf("db error")}
	pub := &mockCommandPublisher{}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "plug-3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestPlugInitializerService_OnPlugOnline_PublishError(t *testing.T) {
	plug := &models.Plug{ID: "plug-4", Namespace: "ns", MqttTopic: "topic", Initialized: false}
	repo := &mockInitializerRepo{plug: plug}
	pub := &mockCommandPublisher{err: fmt.Errorf("mqtt unavailable")}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "plug-4")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mqtt unavailable")
	assert.False(t, repo.setInitialized)
}

func TestPlugInitializerService_OnPlugOnline_MaintenancePlug_SendsPowerON(t *testing.T) {
	plug := &models.Plug{
		ID:          "plug-maint",
		Namespace:   "evcc-ns",
		MqttTopic:   "maint-plug",
		Initialized: false,
		Type:        models.PlugTypeMaintenance,
	}
	repo := &mockInitializerRepo{plug: plug}
	pub := &mockCommandPublisher{}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "plug-maint")
	require.NoError(t, err)

	// Should publish initCommands + one extra Power ON
	assert.Len(t, pub.published, len(initCommands)+1)

	last := pub.published[len(pub.published)-1]
	assert.Equal(t, "Power", last.cmd)
	assert.Equal(t, "ON", last.payload)
	assert.True(t, repo.setInitialized)
}

func TestPlugInitializerService_OnPlugOnline_ChargingPlug_NoExtraPowerON(t *testing.T) {
	plug := &models.Plug{
		ID:          "plug-charging",
		Namespace:   "evcc-ns",
		MqttTopic:   "charge-plug",
		Initialized: false,
		Type:        models.PlugTypeCharging,
	}
	repo := &mockInitializerRepo{plug: plug}
	pub := &mockCommandPublisher{}
	svc := NewPlugInitializerService(repo, pub)

	err := svc.OnPlugOnline(context.Background(), "plug-charging")
	require.NoError(t, err)

	// Charging plug should NOT get an extra Power ON
	assert.Len(t, pub.published, len(initCommands))
	for _, cmd := range pub.published {
		assert.NotEqual(t, "Power", cmd.cmd, "charging plug must not receive Power command on init")
	}
}
