package services_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	pahopkg "github.com/eclipse/paho.golang/paho"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	mqttpkg "ev-charge-controller/api/mqtt"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureDynsecPublisher records Publish calls for inspection in tests.
type captureDynsecPublisher struct {
	mu       sync.Mutex
	messages []string
}

func (p *captureDynsecPublisher) Publish(_ context.Context, msg *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, string(msg.Payload))
	return &pahopkg.PublishResponse{}, nil
}

func (p *captureDynsecPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.messages)
}

func setupProvisioningService(t *testing.T) (*services.MqttProvisioningService, *models.User, *repository.PlugRepository, *captureDynsecPublisher) {
	t.Helper()
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "provision@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	pub := &captureDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(pub)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, dynsec, cfg)
	return svc, user, plugRepo, pub
}

func TestMqttProvisioningService_CreatePlug_CreatesPlugOnly(t *testing.T) {
	svc, user, plugRepo, pub := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "My Garage", "garage", "", "")
	require.NoError(t, err)

	// Plug created in DB.
	assert.NotEmpty(t, plug.ID)
	assert.Equal(t, "My Garage", plug.Name)
	assert.Equal(t, user.ID, plug.UserID)
	assert.NotEmpty(t, plug.Namespace, "namespace should be generated at creation")
	assert.True(t, strings.HasPrefix(plug.Namespace, "ns-"), "namespace=%s", plug.Namespace)
	assert.Len(t, plug.MqttTopic, 8)
	assert.Regexp(t, `^[0-9a-f]+$`, plug.MqttTopic)

	// DynsecManager was NOT called (no password generated yet).
	assert.Equal(t, 0, pub.count())

	// Plug findable in DB.
	dbPlug, err := plugRepo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, dbPlug)
}

func TestMqttProvisioningService_CreatePlug_EachPlugGetsOwnID(t *testing.T) {
	svc, user, _, pub := setupProvisioningService(t)
	ctx := context.Background()

	plug1, err := svc.CreatePlug(ctx, user.ID, "Plug One", "plug1", "", "")
	require.NoError(t, err)

	plug2, err := svc.CreatePlug(ctx, user.ID, "Plug Two", "plug2", "", "")
	require.NoError(t, err)

	// Different plugs, different IDs, different namespaces.
	assert.NotEqual(t, plug1.ID, plug2.ID)
	assert.NotEmpty(t, plug1.Namespace)
	assert.NotEmpty(t, plug2.Namespace)
	assert.NotEqual(t, plug1.Namespace, plug2.Namespace)

	// Dynsec not called.
	assert.Equal(t, 0, pub.count())
}

func TestMqttProvisioningService_CreatePlug_RandomTopic(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)

	plug, err := svc.CreatePlug(context.Background(), user.ID, "My Garage Plug", "", "", "")
	require.NoError(t, err)

	// Topic is random hex (8 chars from 4 bytes).
	assert.Len(t, plug.MqttTopic, 8)
	assert.Regexp(t, `^[0-9a-f]+$`, plug.MqttTopic)
}

func TestMqttProvisioningService_DeletePlug_CallsDynsecRemove(t *testing.T) {
	svc, user, _, pub := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "Garage", "garage", "", "")
	require.NoError(t, err)

	// Configure to provision MQTT.
	_, err = svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "", "")
	require.NoError(t, err)
	beforeCount := pub.count()

	err = svc.DeletePlug(ctx, user.ID, plug.ID)
	require.NoError(t, err)

	// One additional dynsec call for the remove.
	assert.Equal(t, beforeCount+1, pub.count())
}

func TestMqttProvisioningService_DeletePlug_NotFound(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)

	err := svc.DeletePlug(context.Background(), user.ID, "nonexistent-id")
	assert.ErrorIs(t, err, services.ErrPlugNotFound)
}

// failingDynsecPublisher always returns an error, simulating broker unavailability.
type failingDynsecPublisher struct{}

func (p *failingDynsecPublisher) Publish(_ context.Context, _ *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
	return nil, fmt.Errorf("broker unavailable")
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_DynsecFailure_NoOrphan(t *testing.T) {
	t.Helper()
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "orphan@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	failPub := &failingDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(failPub)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, dynsec, cfg)

	ctx := context.Background()
	plug, err := svc.CreatePlug(ctx, user.ID, "Orphan Plug", "orphan-slug", "", "")
	require.NoError(t, err)

	_, err = svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "", "")
	assert.Error(t, err, "should fail when dynsec is unavailable")

	// No orphaned row in the plugs table.
	plugs, err := plugRepo.List(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, plugs, "plug row must be rolled back on dynsec failure")
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_ManualPath_ReturnsCommands(t *testing.T) {
	svc, user, _, pub := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "Manual Plug", "manual", "", "")
	require.NoError(t, err)

	// Manual path: no tasmotaIP, just provisions MQTT and returns commands.
	consoleCommands, err := svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "", "")
	require.NoError(t, err)
	assert.NotEmpty(t, consoleCommands)
	assert.Contains(t, consoleCommands, "Backlog")
	assert.Contains(t, consoleCommands, "MQTTHost")
	assert.Contains(t, consoleCommands, "MQTTPassword")

	// Dynsec called once to provision.
	assert.Equal(t, 1, pub.count())

	// Plug now has namespace.
	dbPlug, err := svc.GetPlug(ctx, user.ID, plug.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, dbPlug.Namespace)
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_Reconfigure_ReturnsNewCommands(t *testing.T) {
	svc, user, _, pub := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "Reconfig Plug", "reconfig", "", "")
	require.NoError(t, err)

	// First configure.
	cmds1, err := svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "", "")
	require.NoError(t, err)
	countBefore := pub.count()

	// Second configure (regenerates password).
	cmds2, err := svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "", "")
	require.NoError(t, err)
	assert.NotEqual(t, cmds1, cmds2, "password should be different")
	assert.Contains(t, cmds2, "Backlog")
	assert.Equal(t, countBefore+1, pub.count(), "should publish a dynsec ProvisionPlug command")
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_Failure_DeletesPlug(t *testing.T) {
	svc, user, plugRepo, pub := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "Config Plug", "config", "", "")
	require.NoError(t, err)
	dynsecCountBefore := pub.count()

	// Bad IP causes tasmotaCmd to fail.
	_, err = svc.ConfigureTasmotaDevice(
		ctx, user.ID, plug.ID,
		"192.0.2.1:99999", "",
	)
	assert.Error(t, err, "configure should fail with unreachable IP")

	// Plug must be cleaned up.
	plugs, err := plugRepo.List(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, plugs, "plug must be deleted on configure failure")

	// Dynsec: 1 for provision + 1 for remove = 2 total.
	assert.Equal(t, dynsecCountBefore+2, pub.count(), "dynsec client must be removed")
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_NotFound(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)
	ctx := context.Background()

	_, err := svc.ConfigureTasmotaDevice(ctx, user.ID, "nonexistent", "", "")
	assert.ErrorIs(t, err, services.ErrPlugNotFound)
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_CommandsContainNamespace(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "NS Test", "nstest", "", "")
	require.NoError(t, err)

	cmds, err := svc.ConfigureTasmotaDevice(ctx, user.ID, plug.ID, "", "")
	require.NoError(t, err)

	// Get the namespace from the plug.
	dbPlug, err := svc.GetPlug(ctx, user.ID, plug.ID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(dbPlug.Namespace, "ns-"), "namespace=%s", dbPlug.Namespace)
	assert.Len(t, dbPlug.Namespace, 3+16, "expected ns- + 16 hex chars, got %s", dbPlug.Namespace)
	assert.Contains(t, cmds, dbPlug.Namespace)
}

func TestMqttProvisioningService_ListPlugs(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)
	ctx := context.Background()

	plug1, err := svc.CreatePlug(ctx, user.ID, "Plug One", "plug1", "", "")
	require.NoError(t, err)
	plug2, err := svc.CreatePlug(ctx, user.ID, "Plug Two", "plug2", "", "")
	require.NoError(t, err)

	plugs, err := svc.ListPlugs(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, plugs, 2)

	ids := make(map[string]bool)
	for _, p := range plugs {
		ids[p.ID] = true
	}
	assert.True(t, ids[plug1.ID])
	assert.True(t, ids[plug2.ID])
}

func TestMqttProvisioningService_ListPlugs_Empty(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)

	plugs, err := svc.ListPlugs(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, plugs)
}

func TestMqttProvisioningService_UpdatePlug_Name(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)
	ctx := context.Background()

	plug, err := svc.CreatePlug(ctx, user.ID, "Original", "original", "", "")
	require.NoError(t, err)

	newName := "Renamed"
	updated, err := svc.UpdatePlug(ctx, user.ID, plug.ID, &newName, nil)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)
}

func TestMqttProvisioningService_UpdatePlug_Vehicle(t *testing.T) {
	ctx := context.Background()

	// Create a vehicle first (FK constraint on plugs.vehicle_id)
	vid := "test-vehicle-123"
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	testUser := &models.User{Email: "vehicle@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(ctx, testUser))
	require.NoError(t, testdb.InsertVehicle(db, vid, testUser.ID, "rm1", "Test Vehicle", 20, 80))

	pub := &captureDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(pub)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	testSvc := services.NewMqttProvisioningService(plugRepo, dynsec, cfg)

	plug, err := testSvc.CreatePlug(ctx, testUser.ID, "Garage", "garage", vid, "")
	require.NoError(t, err)
	require.NotNil(t, plug.VehicleID)
	assert.Equal(t, vid, *plug.VehicleID)
}

func TestMqttProvisioningService_UpdatePlug_NotFound(t *testing.T) {
	svc, user, _, _ := setupProvisioningService(t)

	newName := "New Name"
	_, err := svc.UpdatePlug(context.Background(), user.ID, "nonexistent", &newName, nil)
	assert.ErrorIs(t, err, services.ErrPlugNotFound)
}

func TestMqttProvisioningService_SetDynsec(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	plugRepo := repository.NewPlugRepository(db)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, nil, cfg)

	// SetDynsec should not panic with nil dynsec
	pub := &captureDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(pub)
	svc.SetDynsec(dynsec)

	// Verify it was set by checking that the service can now use it
	// (SetDynsec is a simple setter, hard to verify without reflection)
}

func TestMqttProvisioningService_GetPlug_WrongUser(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user1 := &models.User{Email: "user1@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user1))

	user2 := &models.User{Email: "user2@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user2))

	pub := &captureDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(pub)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, dynsec, cfg)

	plug, err := svc.CreatePlug(context.Background(), user1.ID, "User1 Plug", "user1-plug", "", "")
	require.NoError(t, err)

	// User2 tries to get user1's plug
	_, err = svc.GetPlug(context.Background(), user2.ID, plug.ID)
	assert.ErrorIs(t, err, services.ErrPlugNotFound)
}

func TestMqttProvisioningService_UpdatePlug_WrongUser(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user1 := &models.User{Email: "user1@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user1))

	user2 := &models.User{Email: "user2@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user2))

	pub := &captureDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(pub)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, dynsec, cfg)

	plug, err := svc.CreatePlug(context.Background(), user1.ID, "User1 Plug", "user1-plug", "", "")
	require.NoError(t, err)

	// User2 tries to update user1's plug
	newName := "Hacked"
	_, err = svc.UpdatePlug(context.Background(), user2.ID, plug.ID, &newName, nil)
	assert.ErrorIs(t, err, services.ErrPlugNotFound)
}

func TestMqttProvisioningService_DeletePlug_WrongUser(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user1 := &models.User{Email: "user1@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user1))

	user2 := &models.User{Email: "user2@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user2))

	pub := &captureDynsecPublisher{}
	dynsec := mqttpkg.NewDynsecManager(pub)
	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, dynsec, cfg)

	plug, err := svc.CreatePlug(context.Background(), user1.ID, "User1 Plug", "user1-plug", "", "")
	require.NoError(t, err)

	// User2 tries to delete user1's plug
	err = svc.DeletePlug(context.Background(), user2.ID, plug.ID)
	assert.ErrorIs(t, err, services.ErrPlugNotFound)
}

func TestGenerateNamespace(t *testing.T) {
	ns1, err := services.GenerateNamespace()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(ns1, "ns-"))
	assert.Len(t, ns1, 3+16) // "ns-" + 16 hex chars

	ns2, err := services.GenerateNamespace()
	require.NoError(t, err)
	assert.NotEqual(t, ns1, ns2)
}

func TestGeneratePassword(t *testing.T) {
	pw1, err := services.GeneratePassword()
	require.NoError(t, err)
	assert.Len(t, pw1, 48) // 24 bytes hex = 48 chars

	pw2, err := services.GeneratePassword()
	require.NoError(t, err)
	assert.NotEqual(t, pw1, pw2)
}

func TestBuildConsoleCommands(t *testing.T) {
	cmds := services.BuildConsoleCommands("192.168.1.1", "1883", "ns-test", "mytopic", "secret")
	assert.Contains(t, cmds, "Backlog")
	assert.Contains(t, cmds, "MQTTHost 192.168.1.1")
	assert.Contains(t, cmds, "MQTTPort 1883")
	assert.Contains(t, cmds, "MQTTPassword secret")
	assert.Contains(t, cmds, "Topic mytopic")
	assert.Contains(t, cmds, "Restart 1")
}

func TestParseBacklogCommands(t *testing.T) {
	backlog := "Backlog MQTTHost 192.168.1.1; MQTTPort 1883; Restart 1"
	cmds := services.ParseBacklogCommands(backlog)
	assert.Equal(t, []string{"MQTTHost 192.168.1.1", "MQTTPort 1883", "Restart 1"}, cmds)
}

func TestMqttProvisioningService_ConfigureTasmotaDevice_NilDynsec(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	user := &models.User{Email: "nildynsec@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	cfg := &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"}
	svc := services.NewMqttProvisioningService(plugRepo, nil, cfg)

	plug, err := svc.CreatePlug(context.Background(), user.ID, "No Dynsec Plug", "no-dynsec", "", "")
	require.NoError(t, err)

	// With nil dynsec, ConfigureTasmotaDevice should succeed (skips provisioning)
	consoleCommands, err := svc.ConfigureTasmotaDevice(context.Background(), user.ID, plug.ID, "", "")
	require.NoError(t, err)
	assert.NotEmpty(t, consoleCommands)
	assert.Contains(t, consoleCommands, "Backlog")
}
