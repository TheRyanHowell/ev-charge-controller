package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ev-charge-controller/api/internal"
	mqttpkg "ev-charge-controller/api/mqtt"
	"ev-charge-controller/api/models"
)

const (
	tasmotaConfigTimeout = 10 * time.Second
	tasmotaUsername      = "admin"
)

// ErrPlugNotFound is returned when a plug cannot be found or does not belong to the user.
var ErrPlugNotFound = errors.New("plug not found")

// ErrDuplicatePlugName is returned when a plug name already exists for the user.
var ErrDuplicatePlugName = errors.New("a plug with this name already exists")

// plugProvisioner manages per-plug MQTT clients in the broker's auth backend.
type plugProvisioner interface {
	ProvisionPlug(ctx context.Context, namespace, rawPassword string) error
	RemovePlug(ctx context.Context, namespace string) error
}

// httpDoer is the subset of *http.Client needed for Tasmota commands.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// MqttProvisioningService creates plugs and manages their MQTT credentials
// via the broker's dynamic security plugin.
type MqttProvisioningService struct {
	plugs        internal.PlugRepo
	dynsec       plugProvisioner
	mqttExtHost  string
	mqttExtPort  string
	httpClient   httpDoer
}

func NewMqttProvisioningService(plugs internal.PlugRepo, dynsec plugProvisioner, cfg *internal.Config) *MqttProvisioningService {
	return &MqttProvisioningService{
		plugs:       plugs,
		dynsec:      dynsec,
		mqttExtHost: cfg.MQTTExternalIP,
		mqttExtPort: cfg.MQTTExternalPort,
		httpClient:  &http.Client{Timeout: tasmotaConfigTimeout},
	}
}

// SetDynsec wires in the dynsec manager after the MQTT client connects at startup.
func (s *MqttProvisioningService) SetDynsec(d *mqttpkg.DynsecManager) {
	s.dynsec = d
}

// SetHTTPClient overrides the HTTP client for Tasmota commands (used in tests).
func (s *MqttProvisioningService) SetHTTPClient(c httpDoer) {
	s.httpClient = c
}

// GenerateTopic creates a random MQTT topic segment.
func GenerateTopic() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreatePlug creates a plug record in the DB with a generated namespace.
// No MQTT password is generated yet - that happens when the user configures the device.
// mqttTopic is ignored; a random topic is always generated.
// vehicleID is optional; when set it associates the plug with an existing vehicle.
// plugType defaults to "charging" when empty.
func (s *MqttProvisioningService) CreatePlug(ctx context.Context, userID, name, mqttTopic, vehicleID, plugType string) (*models.Plug, error) {
	topic, err := GenerateTopic()
	if err != nil {
		return nil, fmt.Errorf("generate topic: %w", err)
	}
	ns, err := GenerateNamespace()
	if err != nil {
		return nil, fmt.Errorf("generate namespace: %w", err)
	}

	if plugType == "" {
		plugType = models.PlugTypeCharging
	}

	plug := &models.Plug{
		UserID:    userID,
		Name:      name,
		Namespace: ns,
		MqttTopic: topic,
		Type:      plugType,
	}
	if vehicleID != "" {
		plug.VehicleID = &vehicleID
	}
	if err := s.plugs.Create(ctx, plug); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicatePlugName
		}
		return nil, fmt.Errorf("create plug: %w", err)
	}
	return plug, nil
}

// BuildConsoleCommands generates the Tasmota Backlog command string.
func BuildConsoleCommands(host, port, ns, topic, password string) string {
	mqttUser := mqttpkg.PlugMQTTUsername(ns)
	fullTopic := fmt.Sprintf("evcc/%s/%%prefix%%/%%topic%%/", ns)
	return fmt.Sprintf(
		"Backlog MQTTHost %s; MQTTPort %s; MQTTUser %s; MQTTPassword %s; FullTopic %s; Topic %s; TelePeriod 10; SensorRetain 1; PowerRetain 1; SetOption3 1; Restart 1",
		host, port, mqttUser, password, fullTopic, topic,
	)
}

// GetPlug returns a single plug by ID, enforcing user ownership.
func (s *MqttProvisioningService) GetPlug(ctx context.Context, userID, plugID string) (*models.Plug, error) {
	plug, err := s.plugs.FindByID(ctx, plugID)
	if err != nil {
		return nil, fmt.Errorf("find plug: %w", err)
	}
	if plug == nil || plug.UserID != userID {
		return nil, ErrPlugNotFound
	}
	return plug, nil
}

// ListPlugs returns all plugs belonging to userID.
func (s *MqttProvisioningService) ListPlugs(ctx context.Context, userID string) ([]models.Plug, error) {
	return s.plugs.List(ctx, userID)
}

// UpdatePlug updates the name and/or vehicle assignment for a plug owned by userID.
func (s *MqttProvisioningService) UpdatePlug(ctx context.Context, userID, plugID string, name, vehicleID *string) (*models.Plug, error) {
	plug, err := s.plugs.FindByID(ctx, plugID)
	if err != nil {
		return nil, fmt.Errorf("find plug: %w", err)
	}
	if plug == nil || plug.UserID != userID {
		return nil, ErrPlugNotFound
	}

	if name != nil {
		plug.Name = *name
	}
	if vehicleID != nil {
		plug.VehicleID = vehicleID
	}

	if err := s.plugs.Update(ctx, plug); err != nil {
		return nil, fmt.Errorf("update plug: %w", err)
	}
	return plug, nil
}

// DeletePlug removes a plug owned by userID and its dynsec client/role.
func (s *MqttProvisioningService) DeletePlug(ctx context.Context, userID, plugID string) error {
	plug, err := s.plugs.FindByID(ctx, plugID)
	if err != nil {
		return fmt.Errorf("find plug: %w", err)
	}
	if plug == nil || plug.UserID != userID {
		return ErrPlugNotFound
	}

	if s.dynsec != nil {
		if err := s.dynsec.RemovePlug(ctx, plug.Namespace); err != nil {
			return fmt.Errorf("dynsec remove: %w", err)
		}
	}

	return s.plugs.Delete(ctx, plugID, userID)
}

// ConfigureTasmotaDevice provisions MQTT and optionally pushes config to a Tasmota
// device. If tasmotaIP is empty, only provisions MQTT and returns console commands.
// If tasmotaIP is set, also pushes commands to the device via HTTP.
func (s *MqttProvisioningService) ConfigureTasmotaDevice(ctx context.Context, userID, plugID, tasmotaIP, tasmotaPass string) (string, error) {
	plug, err := s.plugs.FindByID(ctx, plugID)
	if err != nil {
		return "", fmt.Errorf("find plug: %w", err)
	}
	if plug == nil || plug.UserID != userID {
		return "", ErrPlugNotFound
	}

	rawPw, err := GeneratePassword()
	if err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}

	if s.dynsec != nil {
		if err := s.dynsec.ProvisionPlug(ctx, plug.Namespace, rawPw); err != nil {
			slog.Warn("dynsec: plug provisioning failed", "plugID", plugID, "err", err)
			_ = s.plugs.Delete(ctx, plug.ID, userID)
			return "", fmt.Errorf("dynsec provision: %w", err)
		}
		slog.Info("dynsec: plug provisioned", "plugID", plugID)
	}

	consoleCmd := BuildConsoleCommands(s.mqttExtHost, s.mqttExtPort, plug.Namespace, plug.MqttTopic, rawPw)

	if tasmotaIP == "" {
		// Manual path: just provision, return commands
		return consoleCmd, nil
	}

	// Auto path: push individual commands to device, then return console fallback
	backlogCmds := ParseBacklogCommands(consoleCmd)
	for _, cmd := range backlogCmds {
		if err := s.tasmotaCmd(ctx, tasmotaIP, tasmotaUsername, tasmotaPass, cmd); err != nil {
			_ = s.DeletePlug(ctx, userID, plugID)
			return "", fmt.Errorf("tasmota command %q: %w", cmd, err)
		}
		if cmd != "Restart 1" {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return consoleCmd, nil
}

// tasmotaCmd sends a single command to a Tasmota device's HTTP API.
func (s *MqttProvisioningService) tasmotaCmd(ctx context.Context, ip, user, pass, cmd string) error {
	endpoint := fmt.Sprintf("http://%s/cm?cmnd=%s", ip, url.QueryEscape(cmd))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from Tasmota", resp.StatusCode)
	}
	return nil
}

// ParseBacklogCommands splits a Tasmota Backlog string into individual commands.
func ParseBacklogCommands(backlog string) []string {
	// Strip the "Backlog " prefix
	cmd := strings.TrimPrefix(backlog, "Backlog ")
	parts := strings.Split(cmd, "; ")
	return parts
}

// GenerateNamespace creates a random namespace identifier.
func GenerateNamespace() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ns-" + hex.EncodeToString(b), nil
}

// GeneratePassword creates a cryptographically random hex password.
func GeneratePassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
