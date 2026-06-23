package repository

import (
	"database/sql"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPushTestDB(t *testing.T) *sql.DB {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES ('test-user', 'push@test.com', '')`)
	require.NoError(t, err)
	return db
}

func TestPushSubscriptionRepository_Upsert_Create(t *testing.T) {
	db := setupPushTestDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	testUserID := "test-user"
	sub := &models.PushSubscription{
		ID:          "sub-1",
		UserID:      &testUserID,
		Endpoint:    "https://fcm.googleapis.com/fcm/send/abc",
		P256dhKey:   "BPGk3xJ5rK8sT2uV4wX6yZ0aB1cD2eF3gH4iJ5kL6mN7oP8qR9sT0uV1wX2yZ3aB4cD5eF6gH7iJ8kL9mN0oP1q",
		AuthKey:     "xYzAbCdEfGhIjKlMnOpQrStUvWxYz0",
	}

	err := repo.Upsert(t.Context(), sub)
	require.NoError(t, err)

	all, err := repo.GetAll(t.Context())
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "sub-1", all[0].ID)
	assert.Equal(t, "https://fcm.googleapis.com/fcm/send/abc", all[0].Endpoint)
}

func TestPushSubscriptionRepository_Upsert_ReplaceDuplicate(t *testing.T) {
	db := setupPushTestDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	// Insert first subscription
	testUserID := "test-user"
	sub1 := &models.PushSubscription{
		ID:       "sub-1",
		UserID:   &testUserID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/abc",
		P256dhKey: "BPGk3xJ5rK8sT2uV4wX6yZ0aB1cD2eF3gH4iJ5kL6mN7oP8qR9sT0uV1wX2yZ3aB4cD5eF6gH7iJ8kL9mN0oP1q",
		AuthKey:  "xYzAbCdEfGhIjKlMnOpQrStUvWxYz0",
	}
	require.NoError(t, repo.Upsert(t.Context(), sub1))

	// Upsert with same endpoint+key but different ID (simulates re-subscription)
	sub2 := &models.PushSubscription{
		ID:       "sub-2",
		UserID:   &testUserID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/abc",
		P256dhKey: "BPGk3xJ5rK8sT2uV4wX6yZ0aB1cD2eF3gH4iJ5kL6mN7oP8qR9sT0uV1wX2yZ3aB4cD5eF6gH7iJ8kL9mN0oP1q",
		AuthKey:  "aBcDeFgHiJkLmNoPqRsTuVwXyZ0Ab1",
	}
	require.NoError(t, repo.Upsert(t.Context(), sub2))

	all, err := repo.GetAll(t.Context())
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "sub-2", all[0].ID)
	assert.Equal(t, "aBcDeFgHiJkLmNoPqRsTuVwXyZ0Ab1", all[0].AuthKey)
}

func TestPushSubscriptionRepository_GetAll_Multiple(t *testing.T) {
	db := setupPushTestDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	testUserID := "test-user"
	subs := []*models.PushSubscription{
		{ID: "sub-1", UserID: &testUserID, Endpoint: "https://fcm.googleapis.com/fcm/send/abc", P256dhKey: "key1", AuthKey: "auth1"},
		{ID: "sub-2", UserID: &testUserID, Endpoint: "https://fcm.googleapis.com/fcm/send/def", P256dhKey: "key2", AuthKey: "auth2"},
		{ID: "sub-3", UserID: &testUserID, Endpoint: "https://fcm.googleapis.com/fcm/send/ghi", P256dhKey: "key3", AuthKey: "auth3"},
	}

	for _, sub := range subs {
		require.NoError(t, repo.Upsert(t.Context(), sub))
	}

	all, err := repo.GetAll(t.Context())
	require.NoError(t, err)
	require.Len(t, all, 3)
}

func TestPushSubscriptionRepository_RemoveByEndpoint(t *testing.T) {
	db := setupPushTestDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	testUserID := "test-user"
	require.NoError(t, repo.Upsert(t.Context(), &models.PushSubscription{ID: "sub-1", UserID: &testUserID, Endpoint: "https://fcm.googleapis.com/fcm/send/abc", P256dhKey: "key1", AuthKey: "auth1"}))
	require.NoError(t, repo.Upsert(t.Context(), &models.PushSubscription{ID: "sub-2", UserID: &testUserID, Endpoint: "https://fcm.googleapis.com/fcm/send/def", P256dhKey: "key2", AuthKey: "auth2"}))

	err := repo.RemoveByEndpoint(t.Context(), "https://fcm.googleapis.com/fcm/send/abc")
	require.NoError(t, err)

	all, err := repo.GetAll(t.Context())
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "sub-2", all[0].ID)
}

func TestPushSubscriptionRepository_RemoveByEndpoint_NotFound(t *testing.T) {
	db := setupPushTestDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	err := repo.RemoveByEndpoint(t.Context(), "https://nonexistent.endpoint")
	assert.NoError(t, err)
}

func TestPushSubscriptionRepository_GetAll_Empty(t *testing.T) {
	db := setupPushTestDB(t)
	defer db.Close()

	repo := NewPushSubscriptionRepository(db)

	all, err := repo.GetAll(t.Context())
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestPushSubscriptionRepository_GetAll_DBError(t *testing.T) {
	db := setupPushTestDB(t)

	repo := NewPushSubscriptionRepository(db)

	// Close DB to force error
	db.Close()

	_, err := repo.GetAll(t.Context())
	assert.Error(t, err)
}
