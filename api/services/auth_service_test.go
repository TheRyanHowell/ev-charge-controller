package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	return db
}

func TestAuthService_Register_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, err := svc.Register(context.Background(), "test@example.com", "password123")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "test@example.com", user.Email)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, user.PasswordHash)
}

func TestAuthService_Register_EmailTaken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, err := svc.Register(context.Background(), "test@example.com", "different")
	assert.Equal(t, ErrEmailTaken, err)
}

func TestAuthService_Login_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")

	user, pair, err := svc.Login(context.Background(), "test@example.com", "password123")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "test@example.com", user.Email)
	require.NotNil(t, pair)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(accessTokenTTL), pair.ExpiresAt, time.Second)
}

func TestAuthService_Login_InvalidEmail(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, pair, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	assert.Equal(t, ErrInvalidCredentials, err)
	assert.Nil(t, user)
	assert.Nil(t, pair)
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")

	user, pair, err := svc.Login(context.Background(), "test@example.com", "wrong")
	assert.Equal(t, ErrInvalidCredentials, err)
	assert.Nil(t, user)
	assert.Nil(t, pair)
}

func TestAuthService_Refresh_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, newPair)
	assert.NotEqual(t, pair.RefreshToken, newPair.RefreshToken)
	assert.NotEmpty(t, newPair.AccessToken)
}

func TestAuthService_Refresh_InvalidToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.Refresh(context.Background(), "invalid-token")
	assert.Equal(t, ErrInvalidToken, err)
}

func TestAuthService_Refresh_RevokedToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// First refresh consumes the token
	_, _ = svc.Refresh(context.Background(), pair.RefreshToken)

	// Second refresh should fail (token revoked)
	_, err := svc.Refresh(context.Background(), pair.RefreshToken)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestAuthService_Logout_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	err := svc.Logout(context.Background(), pair.RefreshToken)
	require.NoError(t, err)

	// Token should be revoked
	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestAuthService_Logout_AlreadyRevoked(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	// Logout with unknown token should not error (noop)
	err := svc.Logout(context.Background(), "unknown-token")
	assert.NoError(t, err)
}

func TestAuthService_GetUser_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, _ := svc.Register(context.Background(), "test@example.com", "password123")

	found, err := svc.GetUser(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "test@example.com", found.Email)
}

func TestAuthService_GetUser_NotFound(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	found, err := svc.GetUser(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAuthService_ValidateAccessToken_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, _ := svc.Register(context.Background(), "test@example.com", "password123")

	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	uid, err := svc.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, uid)
}

func TestAuthService_ValidateAccessToken_InvalidToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	uid, err := svc.ValidateAccessToken("invalid.token.here")
	assert.Equal(t, ErrInvalidToken, err)
	assert.Empty(t, uid)
}

func TestAuthService_ValidateAccessToken_WrongSecret(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Create a new service with a different secret
	wrongSvc := NewAuthService(userRepo, tokenRepo, "different-secret")

	uid, err := wrongSvc.ValidateAccessToken(pair.AccessToken)
	assert.Equal(t, ErrInvalidToken, err)
	assert.Empty(t, uid)
}

func TestAuthService_TokenPair_ExpiresAt(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Access token expires in ~1 hour
	assert.WithinDuration(t, time.Now().Add(1*time.Hour), pair.ExpiresAt, 5*time.Second)
}

func TestAuthService_Refresh_IssuesNewPair(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair1, _ := svc.Login(context.Background(), "test@example.com", "password123")

	pair2, _ := svc.Refresh(context.Background(), pair1.RefreshToken)

	// New refresh token (rotation)
	assert.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken)
	// New access token is issued (may have same claims if within same second)
	assert.NotEmpty(t, pair2.AccessToken)
}

func TestAuthService_MultipleRefreshes(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Chain of refreshes
	for i := 0; i < 3; i++ {
		newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
		require.NoError(t, err)
		pair = newPair
	}

	// Final token should be valid
	uid, err := svc.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.NotEmpty(t, uid)
}

func TestHashToken_Deterministic(t *testing.T) {
	hash1 := hashToken("same-token")
	hash2 := hashToken("same-token")
	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64) // SHA-256 hex = 64 chars
}

func TestHashToken_DifferentTokens(t *testing.T) {
	hash1 := hashToken("token-a")
	hash2 := hashToken("token-b")
	assert.NotEqual(t, hash1, hash2)
}

func TestGenerateRawToken_Unique(t *testing.T) {
	t1, err := generateRawToken()
	require.NoError(t, err)
	t2, err := generateRawToken()
	require.NoError(t, err)
	assert.NotEqual(t, t1, t2)
	assert.Len(t, t1, 64) // 32 bytes hex = 64 chars
}

func TestAuthService_Register_CreatesUserInDB(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, _ := svc.Register(context.Background(), "test@example.com", "password123")

	// Verify user exists in DB
	found, err := userRepo.FindByEmail(context.Background(), "test@example.com")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, user.ID, found.ID)
}

func TestAuthService_Login_CreatesRefreshTokenInDB(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Verify refresh token exists in DB
	storedToken, err := tokenRepo.FindByHash(context.Background(), hashToken(pair.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, storedToken)
	assert.WithinDuration(t, time.Now().Add(refreshTokenTTL), storedToken.ExpiresAt, time.Second)
	assert.Nil(t, storedToken.RevokedAt)
}

func TestAuthService_Login_HashesRefreshToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Raw token should NOT be stored as-is
	var storedHash string
	err := db.QueryRow("SELECT token_hash FROM refresh_tokens WHERE token_hash = ?", pair.RefreshToken).Scan(&storedHash)
	assert.Error(t, err) // Should not find raw token in DB
}

func TestAuthService_Logout_RevokesToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	err := svc.Logout(context.Background(), pair.RefreshToken)
	require.NoError(t, err)

	// Verify token is revoked in DB
	storedToken, err := tokenRepo.FindByHash(context.Background(), hashToken(pair.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, storedToken)
	assert.NotNil(t, storedToken.RevokedAt)
}

func TestAuthService_Refresh_RevokesOldToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	oldHash := hashToken(pair.RefreshToken)
	_, _ = svc.Refresh(context.Background(), pair.RefreshToken)

	// Old token should be revoked
	storedToken, err := tokenRepo.FindByHash(context.Background(), oldHash)
	require.NoError(t, err)
	require.NotNil(t, storedToken)
	assert.NotNil(t, storedToken.RevokedAt)
}

func TestAuthService_SignAccessToken_ValidJWT(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	user, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Verify the access token contains the correct claims
	uid, err := svc.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, uid)
}

func TestAuthService_NewAuthService(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	require.NotNil(t, svc)
	assert.NotNil(t, svc.users)
	assert.NotNil(t, svc.tokens)
	assert.Equal(t, []byte("test-secret"), svc.secret)
}

func TestAuthService_Register_HashPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, _ := svc.Register(context.Background(), "test@example.com", "password123")

	// Password hash should not be the plaintext password
	assert.NotEqual(t, "password123", user.PasswordHash)
	// Hash should start with argon2id identifier
	assert.Contains(t, user.PasswordHash, "$argon2id$")
}

func TestAuthService_TokenPair_Struct(t *testing.T) {
	pair := &TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	assert.Equal(t, "access", pair.AccessToken)
	assert.Equal(t, "refresh", pair.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), pair.ExpiresAt, time.Second)
}

func TestAuthService_Claims_Struct(t *testing.T) {
	claims := &Claims{
		UserID: "user-123",
	}

	assert.Equal(t, "user-123", claims.UserID)
}

func TestAuthService_Login_PopulatesUserID(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	user, _ := svc.Register(context.Background(), "test@example.com", "password123")

	loginUser, _, err := svc.Login(context.Background(), "test@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, user.ID, loginUser.ID)
	assert.Equal(t, user.Email, loginUser.Email)
}

func TestAuthService_Refresh_ExpiredToken(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// Manually expire the token in DB
	_, err := db.Exec("UPDATE refresh_tokens SET expires_at = datetime('now', '-1 hour')")
	require.NoError(t, err)

	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestAuthService_SentinelErrors(t *testing.T) {
	// Verify sentinel errors are exported and non-nil
	assert.NotNil(t, ErrInvalidCredentials)
	assert.NotNil(t, ErrInvalidToken)
	assert.NotNil(t, ErrEmailTaken)

	// Verify error messages
	assert.Equal(t, "invalid credentials", ErrInvalidCredentials.Error())
	assert.Equal(t, "invalid or expired token", ErrInvalidToken.Error())
	assert.Equal(t, "email already registered", ErrEmailTaken.Error())
}

func TestAuthService_UserModel(t *testing.T) {
	user := &models.User{
		ID:           "test-id",
		Email:        "test@example.com",
		PasswordHash: "hashed",
	}

	assert.Equal(t, "test-id", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hashed", user.PasswordHash)
	assert.True(t, user.CreatedAt.IsZero())
}

func TestAuthService_RefreshTokenModel(t *testing.T) {
	now := time.Now()
	tok := &models.RefreshToken{
		ID:        "rt-id",
		UserID:    "user-id",
		TokenHash: "hash",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}

	assert.Equal(t, "rt-id", tok.ID)
	assert.Equal(t, "user-id", tok.UserID)
	assert.Equal(t, "hash", tok.TokenHash)
	assert.Nil(t, tok.RevokedAt)
}

func TestAuthService_Logout_Idempotent(t *testing.T) {
	db := setupAuthTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _ = svc.Register(context.Background(), "test@example.com", "password123")
	_, pair, _ := svc.Login(context.Background(), "test@example.com", "password123")

	// First logout
	require.NoError(t, svc.Logout(context.Background(), pair.RefreshToken))
	// Second logout (already revoked)
	require.NoError(t, svc.Logout(context.Background(), pair.RefreshToken))
}

// --- Mock repos for error path testing ---

type mockUserRepo struct {
	findByEmailFn func(ctx context.Context, email string) (*models.User, error)
	findByIDFn    func(ctx context.Context, id string) (*models.User, error)
	createFn      func(ctx context.Context, user *models.User) error
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return m.findByEmailFn(ctx, email)
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*models.User, error) {
	return m.findByIDFn(ctx, id)
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	return m.createFn(ctx, user)
}

type mockTokenRepo struct {
	findByHashFn func(ctx context.Context, hash string) (*models.RefreshToken, error)
	revokeFn     func(ctx context.Context, id string) error
	createFn     func(ctx context.Context, token *models.RefreshToken) error
}

func (m *mockTokenRepo) FindByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	return m.findByHashFn(ctx, hash)
}

func (m *mockTokenRepo) Revoke(ctx context.Context, id string) error {
	return m.revokeFn(ctx, id)
}

func (m *mockTokenRepo) Create(ctx context.Context, token *models.RefreshToken) error {
	return m.createFn(ctx, token)
}

func TestAuthService_Register_UserCreateError(t *testing.T) {
	wantErr := errors.New("db connection lost")
	userRepo := &mockUserRepo{
		findByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
			return nil, nil
		},
		createFn: func(_ context.Context, _ *models.User) error {
			return wantErr
		},
	}
	tokenRepo := &mockTokenRepo{
		createFn: func(_ context.Context, _ *models.RefreshToken) error { return nil },
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.Register(context.Background(), "new@example.com", "password123")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_Register_FindByEmailError(t *testing.T) {
	wantErr := errors.New("db query failed")
	userRepo := &mockUserRepo{
		findByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
			return nil, wantErr
		},
	}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.Register(context.Background(), "new@example.com", "password123")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_Login_IssueTokenPairError(t *testing.T) {
	wantErr := errors.New("token creation failed")
	testUser := &models.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$...",
	}

	// We need a real hash to pass argon2id comparison
	hash, _ := argon2id.CreateHash("password123", argon2id.DefaultParams)
	testUser.PasswordHash = hash

	userRepo := &mockUserRepo{
		findByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
			return testUser, nil
		},
	}
	tokenRepo := &mockTokenRepo{
		createFn: func(_ context.Context, _ *models.RefreshToken) error {
			return wantErr
		},
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _, err := svc.Login(context.Background(), "test@example.com", "password123")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_Refresh_FindByHashError(t *testing.T) {
	wantErr := errors.New("db query failed")
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{
		findByHashFn: func(_ context.Context, _ string) (*models.RefreshToken, error) {
			return nil, wantErr
		},
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.Refresh(context.Background(), "some-token")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_Refresh_RevokeError(t *testing.T) {
	wantErr := errors.New("db update failed")
	now := time.Now()
	token := &models.RefreshToken{
		ID:        "rt-1",
		UserID:    "user-1",
		TokenHash: "hash",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}

	userRepo := &mockUserRepo{
		findByIDFn: func(_ context.Context, _ string) (*models.User, error) {
			return &models.User{ID: "user-1", Email: "test@example.com"}, nil
		},
	}
	tokenRepo := &mockTokenRepo{
		findByHashFn: func(_ context.Context, _ string) (*models.RefreshToken, error) {
			return token, nil
		},
		revokeFn: func(_ context.Context, _ string) error {
			return wantErr
		},
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.Refresh(context.Background(), "some-token")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_ValidateAccessToken_ExpiredToken(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	// Create a token that's already expired
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
		UserID: "user-1",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	uid, err := svc.ValidateAccessToken(tokenStr)
	assert.Equal(t, ErrInvalidToken, err)
	assert.Empty(t, uid)
}

func TestAuthService_ValidateAccessToken_EmptyUserID(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	// Create a valid token but with empty UserID
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: "", // Empty userID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	uid, err := svc.ValidateAccessToken(tokenStr)
	assert.Equal(t, ErrInvalidToken, err)
	assert.Empty(t, uid)
}

func TestAuthService_Logout_FindByHashError(t *testing.T) {
	wantErr := errors.New("db query failed")
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{
		findByHashFn: func(_ context.Context, _ string) (*models.RefreshToken, error) {
			return nil, wantErr
		},
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	err := svc.Logout(context.Background(), "some-token")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_Logout_RevokeError(t *testing.T) {
	wantErr := errors.New("db update failed")
	now := time.Now()
	token := &models.RefreshToken{
		ID:        "rt-1",
		UserID:    "user-1",
		TokenHash: "hash",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{
		findByHashFn: func(_ context.Context, _ string) (*models.RefreshToken, error) {
			return token, nil
		},
		revokeFn: func(_ context.Context, _ string) error {
			return wantErr
		},
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	err := svc.Logout(context.Background(), "some-token")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}



func TestAuthService_Login_FindByEmailError(t *testing.T) {
	wantErr := errors.New("db query failed")
	userRepo := &mockUserRepo{
		findByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
			return nil, wantErr
		},
	}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, _, err := svc.Login(context.Background(), "test@example.com", "password123")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_GetUser_FindByIDError(t *testing.T) {
	wantErr := errors.New("db query failed")
	userRepo := &mockUserRepo{
		findByIDFn: func(_ context.Context, _ string) (*models.User, error) {
			return nil, wantErr
		},
	}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.GetUser(context.Background(), "user-1")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}

func TestAuthService_Refresh_IssueTokenPairError(t *testing.T) {
	wantErr := errors.New("token create failed")
	now := time.Now()
	token := &models.RefreshToken{
		ID:        "rt-1",
		UserID:    "user-1",
		TokenHash: "hash",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{
		findByHashFn: func(_ context.Context, _ string) (*models.RefreshToken, error) {
			return token, nil
		},
		revokeFn: func(_ context.Context, _ string) error {
			return nil
		},
		createFn: func(_ context.Context, _ *models.RefreshToken) error {
			return wantErr
		},
	}
	svc := NewAuthService(userRepo, tokenRepo, "test-secret")

	_, err := svc.Refresh(context.Background(), "some-token")
	assert.Error(t, err)
	assert.Equal(t, wantErr, err)
}
