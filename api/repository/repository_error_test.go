package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Error-path tests for plug, vehicle, vehicle_model, user, schedule,
// push_subscription, and refresh_token repositories
// ---------------------------------------------------------------------------

func setupPlugDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	testdb.SeedFullTestDB(t, db)
	return db
}

// --- PlugRepository error paths ---

func TestPlugRepository_Create_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Create(ctx, &models.Plug{
		UserID:      "test-user",
		Name:        "Test Plug",
		Namespace:   "test-ns",
		MqttTopic:   "test-topic",
		TLS:         true,
		Online:      false,
		Initialized: false,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPlugRepository_FindByID_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	// Close DB to force scan error
	db.Close()

	_, err := repo.FindByID(t.Context(), plug.ID)
	assert.Error(t, err)
}

func TestPlugRepository_FindByID_NotFound(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug, err := repo.FindByID(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, plug)
}

func TestPlugRepository_NamespaceAndSlug_NotFound(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	ns, slug, err := repo.NamespaceAndSlug(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, ns)
	assert.Empty(t, slug)
}

func TestPlugRepository_NamespaceAndSlug_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewPlugRepository(db)
	db.Close()

	ns, slug, err := repo.NamespaceAndSlug(t.Context(), "any-id")
	assert.Error(t, err)
	assert.Empty(t, ns)
	assert.Empty(t, slug)
}

func TestPlugRepository_FindByNamespaceAndSlug_NotFound(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug, err := repo.FindByNamespaceAndSlug(t.Context(), "nonexistent-ns", "nonexistent-slug")
	assert.NoError(t, err)
	assert.Nil(t, plug)
}

func TestPlugRepository_FindByNamespaceAndSlug_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewPlugRepository(db)
	db.Close()

	_, err := repo.FindByNamespaceAndSlug(t.Context(), "any-ns", "any-slug")
	assert.Error(t, err)
}

func TestPlugRepository_List_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	plugs, err := repo.List(ctx, "test-user")
	assert.Nil(t, plugs)
	assert.Error(t, err)
}

func TestPlugRepository_Update_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	plug.Name = "Updated"
	err := repo.Update(ctx, plug)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPlugRepository_Delete_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Delete(ctx, plug.ID, "test-user")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPlugRepository_SetInitialized_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.SetInitialized(ctx, plug.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPlugRepository_SetOnline_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.SetOnline(ctx, plug.ID, true)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPlugRepository_UpdateLastOfflineNotifiedAt_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateLastOfflineNotifiedAt(ctx, plug.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPlugRepository_ListNamespacesByUserID_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	plug := &models.Plug{
		UserID:    "test-user",
		Name:      "Test Plug",
		Namespace: "test-ns",
		MqttTopic: "test-topic",
	}
	require.NoError(t, repo.Create(t.Context(), plug))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	ns, err := repo.ListNamespacesByUserID(ctx, "test-user")
	assert.Nil(t, ns)
	assert.Error(t, err)
}

func TestPlugRepository_ListNamespacesByUserID_Empty(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPlugRepository(db)

	// Query a user that has no plugs (seed creates plugs for "test-user")
	ns, err := repo.ListNamespacesByUserID(t.Context(), "no-plugs-user")
	assert.NoError(t, err)
	assert.Empty(t, ns)
}

// --- VehicleRepository error paths ---

func TestVehicleRepository_CreateInstance_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.CreateInstance(ctx, &models.Vehicle{
		UserID:         repoTestUserIDPtr,
		ModelID:        "rm1",
		Name:           "Test Vehicle",
		CurrentPercent: 20.0,
		TargetPercent:  80.0,
		CreatedAt:      time.Now(),
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVehicleRepository_DeleteInstance_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	vehicle := &models.Vehicle{
		UserID:         repoTestUserIDPtr,
		ModelID:        "rm1",
		Name:           "Test Vehicle",
		CurrentPercent: 20.0,
		TargetPercent:  80.0,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateInstance(t.Context(), vehicle))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.DeleteInstance(ctx, vehicle.ID, "test-user")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVehicleRepository_FindByID_NotFound(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	v, err := repo.FindByID(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, v)
}

func TestVehicleRepository_FindByID_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewVehicleRepository(db)
	db.Close()

	_, err := repo.FindByID(t.Context(), "any-id")
	assert.Error(t, err)
}

func TestVehicleRepository_FindByIDs_Empty(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	result, err := repo.FindByIDs(t.Context(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestVehicleRepository_FindByIDs_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := repo.FindByIDs(ctx, []string{"rm1"})
	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestVehicleRepository_UpdatePercents_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	vehicle := &models.Vehicle{
		UserID:         repoTestUserIDPtr,
		ModelID:        "rm1",
		Name:           "Test Vehicle",
		CurrentPercent: 20.0,
		TargetPercent:  80.0,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateInstance(t.Context(), vehicle))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdatePercents(ctx, vehicle.ID, 50.0, 90.0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVehicleRepository_List_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	vehicles, err := repo.List(ctx)
	assert.Nil(t, vehicles)
	assert.Error(t, err)
}

func TestVehicleRepository_UpdateName_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	vehicle := &models.Vehicle{
		UserID:         repoTestUserIDPtr,
		ModelID:        "rm1",
		Name:           "Test Vehicle",
		CurrentPercent: 20.0,
		TargetPercent:  80.0,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateInstance(t.Context(), vehicle))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateName(ctx, vehicle.ID, "New Name", "test-user")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVehicleRepository_IncrementLifetimeStats_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	vehicle := &models.Vehicle{
		UserID:         repoTestUserIDPtr,
		ModelID:        "rm1",
		Name:           "Test Vehicle",
		CurrentPercent: 20.0,
		TargetPercent:  80.0,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateInstance(t.Context(), vehicle))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.IncrementLifetimeStats(ctx, vehicle.ID, 1.5, 1.9, 500, 0, time.Now())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVehicleRepository_DecrementLifetimeStats_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleRepository(db)

	vehicle := &models.Vehicle{
		UserID:         repoTestUserIDPtr,
		ModelID:        "rm1",
		Name:           "Test Vehicle",
		CurrentPercent: 20.0,
		TargetPercent:  80.0,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateInstance(t.Context(), vehicle))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.DecrementLifetimeStats(ctx, vehicle.ID, 1.5, 1.9, 500, 0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- VehicleModelRepository error paths ---

func TestVehicleModelRepository_List_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleModelRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	models, err := repo.List(ctx)
	assert.Nil(t, models)
	assert.Error(t, err)
}

func TestVehicleModelRepository_FindByID_NotFound(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewVehicleModelRepository(db)

	m, err := repo.FindByID(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, m)
}

func TestVehicleModelRepository_FindByID_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewVehicleModelRepository(db)
	db.Close()

	_, err := repo.FindByID(t.Context(), "any-id")
	assert.Error(t, err)
}

// --- UserRepository error paths ---

func TestUserRepository_Create_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Create(ctx, &models.User{
		Email:        "test@example.com",
		PasswordHash: "hashed",
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestUserRepository_FindByEmail_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewUserRepository(db)
	db.Close()

	_, err := repo.FindByEmail(t.Context(), "any@example.com")
	assert.Error(t, err)
}

func TestUserRepository_FindByID_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewUserRepository(db)
	db.Close()

	_, err := repo.FindByID(t.Context(), "any-id")
	assert.Error(t, err)
}

// --- ScheduleRepository error paths ---

func TestScheduleRepository_Get_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewScheduleRepository(db)
	db.Close()

	_, err := repo.Get(t.Context())
	assert.Error(t, err)
}

func TestScheduleRepository_Upsert_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewScheduleRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Upsert(ctx, &models.Schedule{
		ID:      "sched-1",
		Time:    "06:00",
		Enabled: true,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestScheduleRepository_UpsertByPlugID_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewScheduleRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpsertByPlugID(ctx, &models.Schedule{
		ID:      "sched-plug-1",
		PlugID:  &repoTestPlugIDStr,
		Time:    "06:00",
		Enabled: true,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestScheduleRepository_GetByPlugID_NotFound(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewScheduleRepository(db)

	s, err := repo.GetByPlugID(t.Context(), "nonexistent-plug")
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestScheduleRepository_GetByPlugID_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewScheduleRepository(db)
	db.Close()

	_, err := repo.GetByPlugID(t.Context(), "any-plug")
	assert.Error(t, err)
}

func TestScheduleRepository_ListAll_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewScheduleRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	schedules, err := repo.ListAll(ctx)
	assert.Nil(t, schedules)
	assert.Error(t, err)
}

func TestScheduleRepository_ListAll_Empty(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewScheduleRepository(db)

	schedules, err := repo.ListAll(t.Context())
	assert.NoError(t, err)
	assert.Empty(t, schedules)
}

// --- PushSubscriptionRepository error paths ---

func TestPushSubscriptionRepository_Upsert_NoUserID(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	err := repo.Upsert(t.Context(), &models.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://example.com/push",
		P256dhKey: "key1",
		AuthKey:   "auth1",
		UserID:    nil,
	})
	assert.ErrorIs(t, err, ErrPushSubscriptionNoUserID)
}

func TestPushSubscriptionRepository_Upsert_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	userID := "test-user"
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Upsert(ctx, &models.PushSubscription{
		ID:        "sub-cancel",
		Endpoint:  "https://example.com/push",
		P256dhKey: "key2",
		AuthKey:   "auth2",
		UserID:    &userID,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPushSubscriptionRepository_RemoveByEndpoint_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.RemoveByEndpoint(ctx, "https://example.com/push")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPushSubscriptionRepository_GetAll_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	subs, err := repo.GetAll(ctx)
	assert.Nil(t, subs)
	assert.Error(t, err)
}

func TestPushSubscriptionRepository_GetAll_Empty_ErrorPath(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	subs, err := repo.GetAll(t.Context())
	assert.NoError(t, err)
	assert.Empty(t, subs)
}

// --- RefreshTokenRepository error paths ---

func TestRefreshTokenRepository_Create_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewRefreshTokenRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Create(ctx, &models.RefreshToken{
		UserID:    "test-user",
		TokenHash: "hash123",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRefreshTokenRepository_FindByHash_NotFound_ErrorPath(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewRefreshTokenRepository(db)

	tok, err := repo.FindByHash(t.Context(), "nonexistent-hash")
	assert.NoError(t, err)
	assert.Nil(t, tok)
}

func TestRefreshTokenRepository_FindByHash_Error(t *testing.T) {
	db := setupPlugDB(t)

	repo := NewRefreshTokenRepository(db)
	db.Close()

	_, err := repo.FindByHash(t.Context(), "any-hash")
	assert.Error(t, err)
}

func TestRefreshTokenRepository_Revoke_ContextCanceled(t *testing.T) {
	db := setupPlugDB(t)
	defer db.Close()

	repo := NewRefreshTokenRepository(db)

	token := &models.RefreshToken{
		UserID:    "test-user",
		TokenHash: "hash123",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, repo.Create(t.Context(), token))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Revoke(ctx, token.ID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
