package mqtt

import (
	"context"
	"encoding/json"
	"fmt"

	pahopkg "github.com/eclipse/paho.golang/paho"
)

const (
	dynsecCommandsTopic = "$CONTROL/dynamic-security/v1"
	apiRoleName         = "role-api-full-access"
)

// PlugMQTTUsername returns the deterministic MQTT username for a plug namespace.
func PlugMQTTUsername(namespace string) string {
	return "plug-" + namespace
}

func plugRoleName(namespace string) string {
	return "role-" + namespace
}

// dynsecBatch is the JSON envelope sent to $DYNSEC/commands.
type dynsecBatch struct {
	Commands []dynsecCmd `json:"commands"`
}

type dynsecCmd struct {
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

// pahoPublisher is the narrow interface needed by DynsecManager.
// Satisfied by *autopaho.ConnectionManager and *pahopkg.Client.
type pahoPublisher interface {
	Publish(ctx context.Context, p *pahopkg.Publish) (*pahopkg.PublishResponse, error)
}

// DynsecManager provisions per-plug MQTT clients and ACL roles via the
// Mosquitto Dynamic Security plugin's $CONTROL/dynamic-security/v1 interface.
// All operations are fire-and-forget at QoS 1: the broker acknowledges receipt
// before PUBACK is sent, so the plugin has processed the command by the time
// Publish returns without error.
type DynsecManager struct {
	pub pahoPublisher
}

// NewDynsecManager creates a DynsecManager that publishes via pub.
func NewDynsecManager(pub pahoPublisher) *DynsecManager {
	return &DynsecManager{pub: pub}
}

// ProvisionPlug creates a dynsec role scoped to the plug's namespace and a
// client bound to that role. Safe to call even if the client/role already exists
// (the broker logs an error but does not return one to us).
func (m *DynsecManager) ProvisionPlug(ctx context.Context, namespace, rawPassword string) error {
	nsTopicWildcard := fmt.Sprintf("evcc/%s/#", namespace)
	batch := dynsecBatch{
		Commands: []dynsecCmd{
			{
				Command:  "createRole",
				RoleName: plugRoleName(namespace),
				ACLs: []dynsecACL{
					{ACLType: "publishClientSend", Topic: nsTopicWildcard, Allow: true},
					{ACLType: "publishClientReceive", Topic: nsTopicWildcard, Allow: true},
					{ACLType: "subscribePattern", Topic: nsTopicWildcard, Allow: true},
				},
			},
			{
				Command:  "createClient",
				Username: PlugMQTTUsername(namespace),
				Password: rawPassword,
				Roles:    []dynsecRole{{RoleName: plugRoleName(namespace)}},
			},
		},
	}
	if err := m.publish(ctx, batch); err != nil {
		return err
	}
	return nil
}


// RemovePlug deletes the dynsec client and role for a plug.
func (m *DynsecManager) RemovePlug(ctx context.Context, namespace string) error {
	batch := dynsecBatch{
		Commands: []dynsecCmd{
			{Command: "deleteClient", Username: PlugMQTTUsername(namespace)},
			{Command: "deleteRole", RoleName: plugRoleName(namespace)},
		},
	}
	return m.publish(ctx, batch)
}

// EnsureAPIAccess creates a wildcard role and assigns it to the API backend
// client so the API can subscribe and publish to all evcc topics. Idempotent:
// the broker ignores duplicate role/assignment creation.
func (m *DynsecManager) EnsureAPIAccess(ctx context.Context, apiUsername string) error {
	batch := dynsecBatch{
		Commands: []dynsecCmd{
			{
				Command:  "createRole",
				RoleName: apiRoleName,
				ACLs: []dynsecACL{
					{ACLType: "publishClientSend", Topic: "#", Allow: true},
					{ACLType: "publishClientReceive", Topic: "#", Allow: true},
					{ACLType: "subscribePattern", Topic: "#", Allow: true},
				},
			},
			{
				Command:  "addClientRole",
				Username: apiUsername,
				RoleName: apiRoleName,
			},
		},
	}
	return m.publish(ctx, batch)
}

func (m *DynsecManager) publish(ctx context.Context, batch dynsecBatch) error {
	payload, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("dynsec: marshal command: %w", err)
	}
	_, err = m.pub.Publish(ctx, &pahopkg.Publish{
		Topic:   dynsecCommandsTopic,
		QoS:     1,
		Payload: payload,
	})
	return err
}
