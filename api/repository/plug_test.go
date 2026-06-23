package repository_test

import (
	"context"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlugRepo(t *testing.T) (*repository.PlugRepository, *models.User) {
	t.Helper()
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	user := &models.User{Email: "plug-user@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(context.Background(), user))

	return repository.NewPlugRepository(db), user
}

func TestPlugRepository_Create(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Garage plug",
		Namespace: "ns-abc12345",
		MqttTopic: "garage",
	}
	require.NoError(t, repo.Create(ctx, plug))
	assert.NotEmpty(t, plug.ID)
}

func TestPlugRepository_FindByID_Found(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Driveway plug",
		Namespace: "ns-drive001",
		MqttTopic: "driveway",
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Driveway plug", got.Name)
	assert.Equal(t, "ns-drive001", got.Namespace)
	assert.Equal(t, user.ID, got.UserID)
}

func TestPlugRepository_FindByID_NotFound(t *testing.T) {
	repo, _ := setupPlugRepo(t)
	got, err := repo.FindByID(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPlugRepository_SetInitialized(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Smart Plug",
		Namespace: "ns-init001",
		MqttTopic: "init-plug",
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.Initialized)

	require.NoError(t, repo.SetInitialized(ctx, plug.ID))

	got, err = repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.Initialized)
}

func TestPlugRepository_NamespaceAndSlug(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Garage plug",
		Namespace: "ns-abc12345",
		MqttTopic: "garage",
	}
	require.NoError(t, repo.Create(ctx, plug))

	ns, slug, err := repo.NamespaceAndSlug(ctx, plug.ID)
	require.NoError(t, err)
	assert.Equal(t, "ns-abc12345", ns)
	assert.Equal(t, "garage", slug)
}

func TestPlugRepository_NamespaceAndSlug_NotFound(t *testing.T) {
	repo, _ := setupPlugRepo(t)
	ns, slug, err := repo.NamespaceAndSlug(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, ns)
	assert.Empty(t, slug)
}

func TestPlugRepository_FindByNamespaceAndSlug(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Driveway plug",
		Namespace: "ns-drive001",
		MqttTopic: "driveway",
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByNamespaceAndSlug(ctx, "ns-drive001", "driveway")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Driveway plug", got.Name)
}

func TestPlugRepository_FindByNamespaceAndSlug_NotFound(t *testing.T) {
	repo, _ := setupPlugRepo(t)
	got, err := repo.FindByNamespaceAndSlug(context.Background(), "ns-x", "x")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPlugRepository_List(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Plug{UserID: user.ID, Name: "p1", Namespace: "ns-a", MqttTopic: "t1"}))
	require.NoError(t, repo.Create(ctx, &models.Plug{UserID: user.ID, Name: "p2", Namespace: "ns-b", MqttTopic: "t2"}))

	plugs, err := repo.List(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, plugs, 2)
	assert.Equal(t, "p1", plugs[0].Name)
	assert.Equal(t, "p2", plugs[1].Name)

	empty, err := repo.List(ctx, "nonexistent-user")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestPlugRepository_Update(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Old Name",
		Namespace: "ns-old001",
		MqttTopic: "old-topic",
	}
	require.NoError(t, repo.Create(ctx, plug))

	plug.Name = "New Name"
	plug.Namespace = "ns-new001"
	require.NoError(t, repo.Update(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "New Name", got.Name)
	assert.Equal(t, "ns-new001", got.Namespace)
}

func TestPlugRepository_Delete(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "To Delete",
		Namespace: "ns-del001",
		MqttTopic: "del-topic",
	}
	require.NoError(t, repo.Create(ctx, plug))

	require.NoError(t, repo.Delete(ctx, plug.ID, user.ID))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPlugRepository_Delete_WrongUser(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Protected",
		Namespace: "ns-prot001",
		MqttTopic: "prot-topic",
	}
	require.NoError(t, repo.Create(ctx, plug))

	require.NoError(t, repo.Delete(ctx, plug.ID, "wrong-user"))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestPlugRepository_SetOnline(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Online Plug",
		Namespace: "ns-onl001",
		MqttTopic: "onl-topic",
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.False(t, got.Online)

	require.NoError(t, repo.SetOnline(ctx, plug.ID, true))

	got, err = repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.True(t, got.Online)
	require.NotNil(t, got.LastSeen)

	require.NoError(t, repo.SetOnline(ctx, plug.ID, false))

	got, err = repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.False(t, got.Online)
}

func TestPlugRepository_UpdateLastOfflineNotifiedAt(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Notify Plug",
		Namespace: "ns-not001",
		MqttTopic: "not-topic",
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.Nil(t, got.LastOfflineNotifiedAt)

	require.NoError(t, repo.UpdateLastOfflineNotifiedAt(ctx, plug.ID))

	got, err = repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastOfflineNotifiedAt)
}

func TestPlugRepository_TypeAndPowerOn_RoundTrip(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Maint Plug",
		Namespace: "ns-maint01",
		MqttTopic: "maint",
		Type:      models.PlugTypeMaintenance,
		PowerOn:   true,
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, models.PlugTypeMaintenance, got.Type)
	assert.True(t, got.PowerOn)
}

func TestPlugRepository_DefaultTypeIsCharging(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Default Plug",
		Namespace: "ns-def001",
		MqttTopic: "default",
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, models.PlugTypeCharging, got.Type)
	assert.False(t, got.PowerOn)
}

func TestPlugRepository_SetPowerState(t *testing.T) {
	repo, user := setupPlugRepo(t)
	ctx := context.Background()

	plug := &models.Plug{
		UserID:    user.ID,
		Name:      "Power Plug",
		Namespace: "ns-pow001",
		MqttTopic: "power",
		Type:      models.PlugTypeMaintenance,
	}
	require.NoError(t, repo.Create(ctx, plug))

	got, err := repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.False(t, got.PowerOn)

	require.NoError(t, repo.SetPowerState(ctx, plug.ID, true))
	got, err = repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.True(t, got.PowerOn)

	require.NoError(t, repo.SetPowerState(ctx, plug.ID, false))
	got, err = repo.FindByID(ctx, plug.ID)
	require.NoError(t, err)
	assert.False(t, got.PowerOn)
}

func TestPlugRepository_ListNamespacesByUserID(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	ctx := context.Background()

	userA := &models.User{Email: "a@example.com", PasswordHash: "hash"}
	userB := &models.User{Email: "b@example.com", PasswordHash: "hash"}
	require.NoError(t, userRepo.Create(ctx, userA))
	require.NoError(t, userRepo.Create(ctx, userB))

	repo := repository.NewPlugRepository(db)

	require.NoError(t, repo.Create(ctx, &models.Plug{UserID: userA.ID, Name: "p1", Namespace: "ns-aaa", MqttTopic: "t1"}))
	require.NoError(t, repo.Create(ctx, &models.Plug{UserID: userA.ID, Name: "p2", Namespace: "ns-bbb", MqttTopic: "t2"}))
	require.NoError(t, repo.Create(ctx, &models.Plug{UserID: userB.ID, Name: "p3", Namespace: "ns-ccc", MqttTopic: "t3"}))

	nsA, err := repo.ListNamespacesByUserID(ctx, userA.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ns-aaa", "ns-bbb"}, nsA)

	nsB, err := repo.ListNamespacesByUserID(ctx, userB.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ns-ccc"}, nsB)

	nsNone, err := repo.ListNamespacesByUserID(ctx, "no-such-user")
	require.NoError(t, err)
	assert.Empty(t, nsNone)
}
