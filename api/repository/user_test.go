package repository

import (
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserTestDB(t *testing.T) *UserRepository {
	t.Helper()
	db, err := database.SetupTestDB(false)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewUserRepository(db)
}

func TestUserRepository_Create(t *testing.T) {
	repo := setupUserTestDB(t)

	user := &models.User{Email: "alice@example.com", PasswordHash: "hash123"}
	err := repo.Create(t.Context(), user)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	repo := setupUserTestDB(t)

	u1 := &models.User{Email: "alice@example.com", PasswordHash: "h1"}
	require.NoError(t, repo.Create(t.Context(), u1))

	u2 := &models.User{Email: "alice@example.com", PasswordHash: "h2"}
	err := repo.Create(t.Context(), u2)
	assert.Error(t, err)
}

func TestUserRepository_FindByEmail_Found(t *testing.T) {
	repo := setupUserTestDB(t)

	created := &models.User{Email: "bob@example.com", PasswordHash: "secret"}
	require.NoError(t, repo.Create(t.Context(), created))

	found, err := repo.FindByEmail(t.Context(), "bob@example.com")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "bob@example.com", found.Email)
	assert.Equal(t, "secret", found.PasswordHash)
}

func TestUserRepository_FindByEmail_NotFound(t *testing.T) {
	repo := setupUserTestDB(t)

	found, err := repo.FindByEmail(t.Context(), "nobody@example.com")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUserRepository_FindByID_Found(t *testing.T) {
	repo := setupUserTestDB(t)

	created := &models.User{Email: "carol@example.com", PasswordHash: "pw"}
	require.NoError(t, repo.Create(t.Context(), created))

	found, err := repo.FindByID(t.Context(), created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "carol@example.com", found.Email)
}

func TestUserRepository_FindByID_NotFound(t *testing.T) {
	repo := setupUserTestDB(t)

	found, err := repo.FindByID(t.Context(), "nonexistent-id")
	require.NoError(t, err)
	assert.Nil(t, found)
}
