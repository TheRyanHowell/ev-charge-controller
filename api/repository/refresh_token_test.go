package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRefreshTokenTestDB(t *testing.T) (*UserRepository, *RefreshTokenRepository) {
	t.Helper()
	db, err := database.SetupTestDB(false)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewUserRepository(db), NewRefreshTokenRepository(db)
}

func hashTestToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TestRefreshTokenRepository_Create(t *testing.T) {
	users, tokens := setupRefreshTokenTestDB(t)

	user := &models.User{Email: "tok@example.com", PasswordHash: "h"}
	require.NoError(t, users.Create(t.Context(), user))

	tok := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashTestToken("raw-token-1"),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err := tokens.Create(t.Context(), tok)
	require.NoError(t, err)
	assert.NotEmpty(t, tok.ID)
}

func TestRefreshTokenRepository_FindByHash_Found(t *testing.T) {
	users, tokens := setupRefreshTokenTestDB(t)

	user := &models.User{Email: "tok2@example.com", PasswordHash: "h"}
	require.NoError(t, users.Create(t.Context(), user))

	raw := "raw-token-abc"
	tok := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashTestToken(raw),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, tokens.Create(t.Context(), tok))

	found, err := tokens.FindByHash(t.Context(), hashTestToken(raw))
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tok.ID, found.ID)
	assert.Equal(t, user.ID, found.UserID)
	assert.Nil(t, found.RevokedAt)
}

func TestRefreshTokenRepository_FindByHash_NotFound(t *testing.T) {
	_, tokens := setupRefreshTokenTestDB(t)

	found, err := tokens.FindByHash(t.Context(), hashTestToken("nonexistent"))
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRefreshTokenRepository_Revoke(t *testing.T) {
	users, tokens := setupRefreshTokenTestDB(t)

	user := &models.User{Email: "tok3@example.com", PasswordHash: "h"}
	require.NoError(t, users.Create(t.Context(), user))

	tok := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashTestToken("raw-token-xyz"),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, tokens.Create(t.Context(), tok))

	err := tokens.Revoke(t.Context(), tok.ID)
	require.NoError(t, err)

	found, err := tokens.FindByHash(t.Context(), hashTestToken("raw-token-xyz"))
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotNil(t, found.RevokedAt)
}
