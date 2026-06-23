package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	pahopkg "github.com/eclipse/paho.golang/paho"
)

const (
	subscribeAllNamespaces = "evcc/+/#"
	connectRetryDelay      = 5 * time.Second
)

// ClientConfig holds connection parameters for the MQTT backend client.
type ClientConfig struct {
	BrokerURL        string
	Username         string
	Password         string
	ClientID         string
	KeepAlive        uint16
}

// Client wraps autopaho to provide a managed, auto-reconnecting MQTT connection.
type Client struct {
	cm         *autopaho.ConnectionManager
	dispatcher *Dispatcher
}

// NewClient creates and starts an MQTT client that routes messages through dispatcher.
// Call Close to disconnect cleanly.
func NewClient(ctx context.Context, cfg ClientConfig, dispatcher *Dispatcher) (*Client, error) {
	brokerURL, err := url.Parse(cfg.BrokerURL)
	if err != nil {
		return nil, fmt.Errorf("mqtt: parse broker URL: %w", err)
	}
	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = 30
	}
	if cfg.ClientID == "" {
		cfg.ClientID = "ev-charge-api"
	}

	c := &Client{dispatcher: dispatcher}

	ccfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{brokerURL},
		KeepAlive:         cfg.KeepAlive,
		ConnectRetryDelay: connectRetryDelay,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *pahopkg.Connack) {
			slog.Info("mqtt: connected", "broker", cfg.BrokerURL)
			sub := &pahopkg.Subscribe{
				Subscriptions: []pahopkg.SubscribeOptions{
					{Topic: subscribeAllNamespaces, QoS: 1},
				},
			}
			if _, err := cm.Subscribe(ctx, sub); err != nil {
				slog.Error("mqtt: subscribe failed", "err", err)
			}
		},
		OnConnectError: func(err error) {
			slog.Warn("mqtt: connection error", "err", err)
		},
		ClientConfig: pahopkg.ClientConfig{
			ClientID: cfg.ClientID,
			OnPublishReceived: []func(pahopkg.PublishReceived) (bool, error){
				func(pr pahopkg.PublishReceived) (bool, error) {
					slog.Debug("mqtt: received", "topic", pr.Packet.Topic, "retain", pr.Packet.Retain, "payload_len", len(pr.Packet.Payload), "payload", string(pr.Packet.Payload))
					c.dispatcher.Dispatch(ctx, pr.Packet.Topic, pr.Packet.Payload, pr.Packet.Retain)
					return true, nil
				},
			},
			OnClientError: func(err error) {
				slog.Error("mqtt: client error", "err", err)
			},
			OnServerDisconnect: func(d *pahopkg.Disconnect) {
				slog.Warn("mqtt: server disconnected", "reason", d.ReasonCode)
			},
		},
	}
	if cfg.Username != "" {
		ccfg.ConnectUsername = cfg.Username
		ccfg.ConnectPassword = []byte(cfg.Password)
	}

	cm, err := autopaho.NewConnection(ctx, ccfg)
	if err != nil {
		return nil, fmt.Errorf("mqtt: create connection manager: %w", err)
	}
	c.cm = cm
	return c, nil
}

// AwaitConnection blocks until the broker connection is established or ctx is cancelled.
func (c *Client) AwaitConnection(ctx context.Context) error {
	return c.cm.AwaitConnection(ctx)
}

// Disconnect cleanly disconnects from the broker.
func (c *Client) Disconnect(ctx context.Context) {
	if err := c.cm.Disconnect(ctx); err != nil {
		slog.Warn("mqtt: disconnect error", "err", err)
	}
}

// ConnectionManager returns the underlying autopaho connection manager, which
// implements pahoPublisher and can be passed to NewDynsecManager.
func (c *Client) ConnectionManager() *autopaho.ConnectionManager {
	return c.cm
}
