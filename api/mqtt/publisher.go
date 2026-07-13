package mqtt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pahopkg "github.com/eclipse/paho.golang/paho"

	"ev-charge-controller/api/internal"
)

var ErrPowerConfirmationTimeout = errors.New("power command confirmation timed out")

// pahoClient is satisfied by both *pahopkg.Client and *autopaho.ConnectionManager.
type pahoClient interface {
	Publish(ctx context.Context, p *pahopkg.Publish) (*pahopkg.PublishResponse, error)
}

// Publisher publishes MQTT commands to Tasmota plugs.
type Publisher struct {
	client pahoClient
	plugs  plugLookup
}

// plugLookup provides namespace+slug for a given plugID.
type plugLookup interface {
	NamespaceAndSlug(ctx context.Context, plugID string) (namespace, slug string, err error)
}

// repoPlugLookup adapts internal.PlugRepo to the plugLookup interface.
type repoPlugLookup struct {
	repo internal.PlugRepo
}

// NewRepoPlugLookup returns a plugLookup backed by the plug repository.
func NewRepoPlugLookup(repo internal.PlugRepo) plugLookup {
	return &repoPlugLookup{repo: repo}
}

func (r *repoPlugLookup) NamespaceAndSlug(ctx context.Context, plugID string) (string, string, error) {
	plug, err := r.repo.FindByID(ctx, plugID)
	if err != nil {
		return "", "", fmt.Errorf("plugLookup: find plug %s: %w", plugID, err)
	}
	if plug == nil {
		return "", "", fmt.Errorf("plugLookup: plug %s not found", plugID)
	}
	return plug.Namespace, plug.MqttTopic, nil
}

// NewPublisher creates a Publisher backed by the given paho client and plug lookup.
func NewPublisher(client pahoClient, plugs plugLookup) *Publisher {
	return &Publisher{client: client, plugs: plugs}
}

// SetPower publishes ON or OFF to the plug's cmnd/<slug>/POWER topic.
func (p *Publisher) SetPower(ctx context.Context, plugID string, on bool) error {
	ns, slug, err := p.plugs.NamespaceAndSlug(ctx, plugID)
	if err != nil {
		return fmt.Errorf("publisher: lookup plug %s: %w", plugID, err)
	}
	topic := fmt.Sprintf("evcc/%s/cmnd/%s/POWER", ns, slug)
	payload := "OFF"
	if on {
		payload = "ON"
	}
	_, err = p.client.Publish(ctx, &pahopkg.Publish{
		Topic:   topic,
		QoS:     1,
		Payload: []byte(payload),
	})
	return err
}

// PublishCommand publishes an arbitrary command to evcc/<namespace>/cmnd/<slug>/<cmd>.
func (p *Publisher) PublishCommand(ctx context.Context, namespace, slug, cmd, payload string) error {
	topic := fmt.Sprintf("evcc/%s/cmnd/%s/%s", namespace, slug, cmd)
	_, err := p.client.Publish(ctx, &pahopkg.Publish{
		Topic:   topic,
		QoS:     1,
		Payload: []byte(payload),
	})
	return err
}

// SetPowerAndWait publishes a POWER command and waits for stat/POWER confirmation.
// It registers a one-shot confirmation channel with the dispatcher, publishes the
// command, and waits for the channel to be signalled or the context to timeout.
// The stat/POWER topic is already covered by the wildcard evcc/+/# subscription
// established at connection time - no per-call subscribe is needed.
// Only a report of the COMMANDED state confirms: a stat/POWER for the opposite
// state is another actor racing this command (relay reconciliation, a manual
// toggle, a retained replay) and the wait re-arms until the commanded state is
// reported or the timeout elapses.
// Returns (true, nil) if confirmation arrived, (false, ErrPowerConfirmationTimeout) on timeout.
func (p *Publisher) SetPowerAndWait(ctx context.Context, plugID string, on bool, dispatcher *Dispatcher, timeout time.Duration) (bool, error) {
	confirmCh := dispatcher.RegisterPowerConfirm(plugID)
	defer dispatcher.RemovePowerConfirm(plugID)

	slog.Info("mqtt: SetPowerAndWait start", "plugID", plugID, "power_on", on)

	if err := p.SetPower(ctx, plugID, on); err != nil {
		return false, fmt.Errorf("publisher: set power: %w", err)
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Warn("mqtt: context cancelled while waiting for confirmation", "plugID", plugID)
			return false, ctx.Err()
		case <-deadline.C:
			slog.Warn("mqtt: power confirmation timed out", "plugID", plugID, "timeout_s", timeout.Seconds())
			return false, ErrPowerConfirmationTimeout
		case reported := <-confirmCh:
			if reported == on {
				slog.Info("mqtt: power confirmed", "plugID", plugID, "confirmed", reported)
				return true, nil
			}
			slog.Warn("mqtt: stat/POWER reported opposite state, awaiting commanded state", "plugID", plugID, "commanded", on, "reported", reported)
			confirmCh = dispatcher.RegisterPowerConfirm(plugID)
		}
	}
}
