package services

import (
	"context"
	"fmt"
	"log/slog"

	"ev-charge-controller/api/models"
)


// plugInitializerRepo is the narrow plug repo slice needed by PlugInitializerService.
type plugInitializerRepo interface {
	FindByID(ctx context.Context, id string) (*models.Plug, error)
	SetInitialized(ctx context.Context, plugID string) error
}

// plugCommandPublisher publishes arbitrary Tasmota cmnd messages.
type plugCommandPublisher interface {
	PublishCommand(ctx context.Context, namespace, slug, cmd, payload string) error
}

// ensureCommands are Tasmota commands re-asserted on EVERY online transition,
// including for plugs provisioned before a command joined the init set (they
// are already marked initialized, so initCommands never reach them again).
// Each entry is [command, value].
//
// This is the full MQTT retain posture the API depends on - asserted
// explicitly rather than assumed, so a device someone pre-configured (or a
// Tasmota default change) can never desync it:
//
//   - SensorRetain 1: tele/SENSOR retained, so the broker replays the last
//     energy reading the moment the API (re)subscribes - without it, a session
//     started before the first TelePeriod tick has no wall-side baseline.
//   - PowerRetain 0: retained power messages override PowerOnState and cause
//     ghost switching; the API also relies on stat/POWER being LIVE-only for
//     manual button-press detection.
//   - StateRetain 0 / StatusRetain 0: a stale retained STATE/STATUS10 replay
//     would prime relay state or an ancient meter total after a reconnect.
//   - ButtonRetain 0 / SwitchRetain 0: these retain cmnd messages, the classic
//     ghost-switching trap.
//
// Status 10 requests an immediate sensor snapshot (stat/%topic%/STATUS10),
// priming the energy cache the moment a plug comes online - covering the gap
// before the first retained/periodic SENSOR message exists.
var ensureCommands = [][2]string{
	{"SensorRetain", "1"},
	{"PowerRetain", "0"},
	{"StateRetain", "0"},
	{"StatusRetain", "0"},
	{"ButtonRetain", "0"},
	{"SwitchRetain", "0"},
	{"Status", "10"},
}

// initCommands are the Tasmota commands pushed to a plug on first connect.
// Each entry is [command, value].
var initCommands = [][2]string{
	{"EnergyRes", fmt.Sprintf("%d", models.EnergyResolutionDecimalPlaces)},
	{"AmpRes", "2"},
	{"TelePeriod", "10"},
	{"SaveState", "1"},
	{"SaveData", "1"},
	{"SetOption3", "1"},
	{"Restart", "1"},
}

// PlugInitializerService pushes Tasmota configuration commands the first time a
// plug connects via MQTT, then marks it initialized so subsequent Online events
// are no-ops.
type PlugInitializerService struct {
	repo      plugInitializerRepo
	publisher plugCommandPublisher
}

// NewPlugInitializerService creates a PlugInitializerService.
func NewPlugInitializerService(repo plugInitializerRepo, publisher plugCommandPublisher) *PlugInitializerService {
	return &PlugInitializerService{repo: repo, publisher: publisher}
}

// OnPlugOnline is called whenever a plug transitions to Online. If the plug has
// not been initialized, it publishes device-configuration commands and marks
// initialized=true. For maintenance plugs it also sends a one-time Power ON so
// the relay defaults to on after first setup (SaveState 1 persists it on the device).
// Idempotent: subsequent calls are no-ops.
func (s *PlugInitializerService) OnPlugOnline(ctx context.Context, plugID string) error {
	plug, err := s.repo.FindByID(ctx, plugID)
	if err != nil {
		return fmt.Errorf("plug initializer: find plug %s: %w", plugID, err)
	}
	if plug == nil {
		return fmt.Errorf("plug initializer: plug %s not found", plugID)
	}

	for _, cmdPair := range ensureCommands {
		cmd, val := cmdPair[0], cmdPair[1]
		if err := s.publisher.PublishCommand(ctx, plug.Namespace, plug.MqttTopic, cmd, val); err != nil {
			return fmt.Errorf("plug initializer: publish %s to %s: %w", cmd, plugID, err)
		}
	}

	if plug.Initialized {
		return nil
	}

	slog.Info("plug initializer: configuring plug", "plugID", plugID, "name", plug.Name, "type", plug.Type)

	for _, cmdPair := range initCommands {
		cmd, val := cmdPair[0], cmdPair[1]
		if err := s.publisher.PublishCommand(ctx, plug.Namespace, plug.MqttTopic, cmd, val); err != nil {
			return fmt.Errorf("plug initializer: publish %s to %s: %w", cmd, plugID, err)
		}
	}

	if plug.Type == models.PlugTypeMaintenance {
		// Send Power ON once at setup. SaveState 1 (in initCommands) makes Tasmota
		// persist the relay state across reboots - no runtime re-assertion needed.
		if err := s.publisher.PublishCommand(ctx, plug.Namespace, plug.MqttTopic, "Power", "ON"); err != nil {
			return fmt.Errorf("plug initializer: publish Power ON to maintenance plug %s: %w", plugID, err)
		}
		slog.Info("plug initializer: sent default Power ON for maintenance plug", "plugID", plugID)
	}

	if err := s.repo.SetInitialized(ctx, plugID); err != nil {
		return fmt.Errorf("plug initializer: set initialized %s: %w", plugID, err)
	}

	slog.Info("plug initializer: plug configured and marked initialized", "plugID", plugID)
	return nil
}
