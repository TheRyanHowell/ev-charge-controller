package mqtt_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	mqttpkg "ev-charge-controller/api/mqtt"

	pahopkg "github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturePublisher records every Publish call so tests can inspect the messages.
type capturePublisher struct {
	mu       sync.Mutex
	messages []capturedMsg
}

type capturedMsg struct {
	topic   string
	payload []byte
}

func (p *capturePublisher) Publish(_ context.Context, msg *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	body := make([]byte, len(msg.Payload))
	copy(body, msg.Payload)
	p.messages = append(p.messages, capturedMsg{topic: msg.Topic, payload: body})
	return &pahopkg.PublishResponse{}, nil
}

func (p *capturePublisher) last() capturedMsg {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.messages[len(p.messages)-1]
}

// dynsecBatch mirrors the JSON structure sent to $CONTROL/dynamic-security/v1.
type dynsecBatch struct {
	Commands []dynsecCommand `json:"commands"`
}

type dynsecCommand struct {
	Command  string       `json:"command"`
	RoleName string       `json:"rolename,omitempty"`
	Username string       `json:"username,omitempty"`
	Password string       `json:"password,omitempty"`
	ACLs     []dynsecACL  `json:"acls,omitempty"`
	Roles    []dynsecRole `json:"roles,omitempty"`
}

type dynsecACL struct {
	ACLType string `json:"acltype"`
	Topic   string `json:"topic"`
	Allow   bool   `json:"allow"`
}

type dynsecRole struct {
	RoleName string `json:"rolename"`
}

func parseBatch(t *testing.T, payload []byte) dynsecBatch {
	t.Helper()
	var b dynsecBatch
	require.NoError(t, json.Unmarshal(payload, &b))
	return b
}

func TestDynsecManager_ProvisionPlug(t *testing.T) {
	pub := &capturePublisher{}
	mgr := mqttpkg.NewDynsecManager(pub)

	err := mgr.ProvisionPlug(context.Background(), "ns-abc123", "s3cret")
	require.NoError(t, err)

	require.Len(t, pub.messages, 1)
	msg := pub.last()
	assert.Equal(t, "$CONTROL/dynamic-security/v1", msg.topic)

	batch := parseBatch(t, msg.payload)
	require.Len(t, batch.Commands, 2)

	// First command: createRole with three ACL entries for the namespace.
	role := batch.Commands[0]
	assert.Equal(t, "createRole", role.Command)
	assert.Equal(t, "role-ns-abc123", role.RoleName)
	require.Len(t, role.ACLs, 3)
	aclTypes := make(map[string]bool)
	for _, acl := range role.ACLs {
		assert.Equal(t, "evcc/ns-abc123/#", acl.Topic)
		assert.True(t, acl.Allow)
		aclTypes[acl.ACLType] = true
	}
	assert.True(t, aclTypes["publishClientSend"])
	assert.True(t, aclTypes["publishClientReceive"])
	assert.True(t, aclTypes["subscribePattern"])

	// Second command: createClient bound to the role.
	client := batch.Commands[1]
	assert.Equal(t, "createClient", client.Command)
	assert.Equal(t, "plug-ns-abc123", client.Username)
	assert.Equal(t, "s3cret", client.Password)
	require.Len(t, client.Roles, 1)
	assert.Equal(t, "role-ns-abc123", client.Roles[0].RoleName)
}

func TestDynsecManager_RemovePlug(t *testing.T) {
	pub := &capturePublisher{}
	mgr := mqttpkg.NewDynsecManager(pub)

	err := mgr.RemovePlug(context.Background(), "ns-abc123")
	require.NoError(t, err)

	require.Len(t, pub.messages, 1)
	msg := pub.last()
	assert.Equal(t, "$CONTROL/dynamic-security/v1", msg.topic)

	batch := parseBatch(t, msg.payload)
	require.Len(t, batch.Commands, 2)

	assert.Equal(t, "deleteClient", batch.Commands[0].Command)
	assert.Equal(t, "plug-ns-abc123", batch.Commands[0].Username)

	assert.Equal(t, "deleteRole", batch.Commands[1].Command)
	assert.Equal(t, "role-ns-abc123", batch.Commands[1].RoleName)
}

func TestDynsecManager_EnsureAPIAccess(t *testing.T) {
	pub := &capturePublisher{}
	mgr := mqttpkg.NewDynsecManager(pub)

	err := mgr.EnsureAPIAccess(context.Background(), "api-backend")
	require.NoError(t, err)

	require.Len(t, pub.messages, 1)
	msg := pub.last()
	assert.Equal(t, "$CONTROL/dynamic-security/v1", msg.topic)

	batch := parseBatch(t, msg.payload)
	require.Len(t, batch.Commands, 2)

	// createRole with wildcard
	role := batch.Commands[0]
	assert.Equal(t, "createRole", role.Command)
	assert.Equal(t, "role-api-full-access", role.RoleName)
	require.Len(t, role.ACLs, 3)
	for _, acl := range role.ACLs {
		assert.Equal(t, "#", acl.Topic)
		assert.True(t, acl.Allow)
	}

	// addClientRole
	assign := batch.Commands[1]
	assert.Equal(t, "addClientRole", assign.Command)
	assert.Equal(t, "api-backend", assign.Username)
	assert.Equal(t, "role-api-full-access", assign.RoleName)
}

func TestPlugMQTTUsername(t *testing.T) {
	assert.Equal(t, "plug-ns-abc123", mqttpkg.PlugMQTTUsername("ns-abc123"))
}

// errorPublisher always returns an error to test error propagation.
type errorPublisher struct{}

func (p *errorPublisher) Publish(_ context.Context, _ *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
	return nil, assert.AnError
}

func TestDynsecManager_ProvisionPlug_PublishError(t *testing.T) {
	mgr := mqttpkg.NewDynsecManager(&errorPublisher{})

	err := mgr.ProvisionPlug(context.Background(), "ns-abc123", "s3cret")
	assert.Error(t, err)
}

func TestDynsecManager_RemovePlug_PublishError(t *testing.T) {
	mgr := mqttpkg.NewDynsecManager(&errorPublisher{})

	err := mgr.RemovePlug(context.Background(), "ns-abc123")
	assert.Error(t, err)
}

func TestDynsecManager_EnsureAPIAccess_PublishError(t *testing.T) {
	mgr := mqttpkg.NewDynsecManager(&errorPublisher{})

	err := mgr.EnsureAPIAccess(context.Background(), "api-backend")
	assert.Error(t, err)
}
