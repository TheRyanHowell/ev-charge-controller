package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBacklogCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "full backlog",
			input: "Backlog MQTTHost test.local; MQTTPort 1883; MQTTUser user; MQTTPassword pass; FullTopic evcc/ns/%%prefix%%/%%topic%%/; Topic abc123; TelePeriod 10; SensorRetain 1; PowerRetain 1; SetOption3 1; Restart 1",
			expected: []string{
				"MQTTHost test.local",
				"MQTTPort 1883",
				"MQTTUser user",
				"MQTTPassword pass",
				"FullTopic evcc/ns/%%prefix%%/%%topic%%/",
				"Topic abc123",
				"TelePeriod 10",
				"SensorRetain 1",
				"PowerRetain 1",
				"SetOption3 1",
				"Restart 1",
			},
		},
		{
			name:     "single command",
			input:    "Backlog Restart 1",
			expected: []string{"Restart 1"},
		},
		{
			name:     "no backlog prefix",
			input:    "MQTTHost test.local; MQTTPort 1883",
			expected: []string{"MQTTHost test.local", "MQTTPort 1883"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseBacklogCommands(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// tasmotaMockClient records requests and returns configurable responses.
type tasmotaMockClient struct {
	requests []*http.Request
	status   int
}

func (m *tasmotaMockClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(http.NoBody),
	}, nil
}

func TestConfigureTasmotaDevice_AutoPath_SendsAllCommands(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "auto@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	mockClient := &tasmotaMockClient{status: http.StatusOK}

	cfg := &internal.Config{MQTTExternalIP: "mqtt.local", MQTTExternalPort: "1883"}
	svc := NewMqttProvisioningService(plugRepo, nil, cfg)
	svc.SetHTTPClient(mockClient)

	ctx := context.Background()
	plug, err := svc.CreatePlug(ctx, user.ID, "Auto Plug", "auto", "", "")
	require.NoError(t, err)

	consoleCmd, err := svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "192.168.1.100", "")
	require.NoError(t, err)

	// Should send 11 commands (MQTTHost, MQTTPort, MQTTUser, MQTTPassword, FullTopic, Topic, TelePeriod, SensorRetain, PowerRetain, SetOption3, Restart)
	assert.Equal(t, 11, len(mockClient.requests))

	// Verify specific commands
	assert.Contains(t, mockClient.requests[0].URL.RawQuery, "cmnd=MQTTHost+mqtt.local")
	assert.Contains(t, mockClient.requests[1].URL.RawQuery, "cmnd=MQTTPort+1883")
	assert.Contains(t, mockClient.requests[6].URL.RawQuery, "cmnd=TelePeriod+10")
	assert.Contains(t, mockClient.requests[7].URL.RawQuery, "cmnd=SensorRetain+1")
	assert.Contains(t, mockClient.requests[8].URL.RawQuery, "cmnd=PowerRetain+1")
	assert.Contains(t, mockClient.requests[9].URL.RawQuery, "cmnd=SetOption3+1")
	assert.Contains(t, mockClient.requests[10].URL.RawQuery, "cmnd=Restart+1")

	// Return value should contain all commands
	assert.Contains(t, consoleCmd, "TelePeriod 10")
	assert.Contains(t, consoleCmd, "SensorRetain 1")
	assert.Contains(t, consoleCmd, "PowerRetain 1")
}

func TestConfigureTasmotaDevice_AutoPath_MidwayFailure_CleansUp(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "fail@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	// Fail on 5th request
	failingClient := &tasmotaSequentialClient{
		responses: []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusOK, http.StatusInternalServerError},
	}

	cfg := &internal.Config{MQTTExternalIP: "mqtt.local", MQTTExternalPort: "1883"}
	svc := NewMqttProvisioningService(plugRepo, nil, cfg)
	svc.SetHTTPClient(failingClient)

	ctx := context.Background()
	plug, err := svc.CreatePlug(ctx, user.ID, "Fail Plug", "fail", "", "")
	require.NoError(t, err)

	_, err = svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "192.168.1.100", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tasmota command")

	// Plug should be cleaned up
	plugs, err := plugRepo.List(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, plugs)
}

// tasmotaSequentialClient returns different status codes per request.
type tasmotaSequentialClient struct {
	responses []int
	index     int
	requests  []*http.Request
}

func (m *tasmotaSequentialClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	status := m.responses[m.index]
	m.index++
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(http.NoBody),
	}, nil
}

// errorPlugRepo wraps a real repo but injects errors on specific calls.
type errorPlugRepo struct {
	*repository.PlugRepository
	failCreate bool
	failUpdate bool
	failDelete bool
	failFindByID bool
}

func (r *errorPlugRepo) Create(ctx context.Context, plug *models.Plug) error {
	if r.failCreate {
		return fmt.Errorf("simulated DB error on create")
	}
	return r.PlugRepository.Create(ctx, plug)
}

func (r *errorPlugRepo) Update(ctx context.Context, plug *models.Plug) error {
	if r.failUpdate {
		return fmt.Errorf("simulated DB error on update")
	}
	return r.PlugRepository.Update(ctx, plug)
}

func (r *errorPlugRepo) Delete(ctx context.Context, id string, userID string) error {
	if r.failDelete {
		return fmt.Errorf("simulated DB error on delete")
	}
	return r.PlugRepository.Delete(ctx, id, userID)
}

func (r *errorPlugRepo) FindByID(ctx context.Context, id string) (*models.Plug, error) {
	if r.failFindByID {
		return nil, fmt.Errorf("simulated DB error on find")
	}
	return r.PlugRepository.FindByID(ctx, id)
}

// errorPlugProvisioner simulates dynsec failures.
type errorPlugProvisioner struct {
	failProvision bool
	failRemove    bool
}

func (p *errorPlugProvisioner) ProvisionPlug(ctx context.Context, namespace, rawPassword string) error {
	if p.failProvision {
		return fmt.Errorf("simulated dynsec provision error")
	}
	return nil
}

func (p *errorPlugProvisioner) RemovePlug(ctx context.Context, namespace string) error {
	if p.failRemove {
		return fmt.Errorf("simulated dynsec remove error")
	}
	return nil
}

func TestCreatePlug_DBError(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "dberr@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	// Close DB to force errors
	require.NoError(t, db.Close())

	errRepo := &errorPlugRepo{PlugRepository: plugRepo, failCreate: true}
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := NewMqttProvisioningService(errRepo, nil, cfg)

	_, err = svc.CreatePlug(context.Background(), user.ID, "Error Plug", "error", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create plug")
}

func TestUpdatePlug_DBError(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "upderr@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	// Create a plug first
	svc := NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	plug, err := svc.CreatePlug(context.Background(), user.ID, "Update Plug", "update", "", "")
	require.NoError(t, err)

	// Now use error repo for update
	errRepo := &errorPlugRepo{PlugRepository: plugRepo, failUpdate: true}
	svc2 := NewMqttProvisioningService(errRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})

	newName := "Updated"
	_, err = svc2.UpdatePlug(context.Background(), user.ID, plug.ID, &newName, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update plug")
}

func TestDeletePlug_DynsecRemoveError(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "delerr@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	// Create a plug first
	svc := NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	plug, err := svc.CreatePlug(context.Background(), user.ID, "Delete Plug", "delete", "", "")
	require.NoError(t, err)

	// Now use error provisioner that fails on RemovePlug
	errProv := &errorPlugProvisioner{failRemove: true}
	svc2 := NewMqttProvisioningService(plugRepo, errProv, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})

	err = svc2.DeletePlug(context.Background(), user.ID, plug.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dynsec remove")
}

func TestGetPlug_DBError(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "geterr@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	// Close DB to force errors
	require.NoError(t, db.Close())

	errRepo := &errorPlugRepo{PlugRepository: plugRepo, failFindByID: true}
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := NewMqttProvisioningService(errRepo, nil, cfg)

	_, err = svc.GetPlug(context.Background(), user.ID, "some-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "find plug")
}

func TestDeletePlug_DBDeleteError(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "delerr2@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	// Create a plug first
	svc := NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	plug, err := svc.CreatePlug(context.Background(), user.ID, "Delete Plug 2", "delete2", "", "")
	require.NoError(t, err)

	// Use error repo that fails on Delete
	errRepo := &errorPlugRepo{PlugRepository: plugRepo, failDelete: true}
	svc2 := NewMqttProvisioningService(errRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})

	err = svc2.DeletePlug(context.Background(), user.ID, plug.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated DB error on delete")
}
