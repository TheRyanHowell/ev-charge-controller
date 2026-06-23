package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-secret-key-for-unit-tests"

func setupAuthHandler(t *testing.T) *AuthHandler {
	t.Helper()
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, testJWTSecret)
	return NewAuthHandler(authSvc)
}

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", jsonBody(t, map[string]string{
		"email": "alice@example.com", "password": "secret123",
	}))
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "alice@example.com", body["email"])
	assert.Nil(t, body["passwordHash"])
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	h.Register(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	h := setupAuthHandler(t)

	body := jsonBody(t, map[string]string{"email": "alice@example.com", "password": "pw"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	h.Register(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", jsonBody(t, map[string]string{
		"email": "alice@example.com", "password": "other",
	}))
	rr := httptest.NewRecorder()
	h.Register(rr, req2)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestAuthHandler_Register_MissingFields(t *testing.T) {
	h := setupAuthHandler(t)

	for _, body := range []map[string]string{
		{"email": ""},
		{"password": "pw"},
		{},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", jsonBody(t, body))
		rr := httptest.NewRecorder()
		h.Register(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h := setupAuthHandler(t)

	// Register first
	regReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", jsonBody(t, map[string]string{
		"email": "bob@example.com", "password": "mypassword",
	}))
	h.Register(httptest.NewRecorder(), regReq)

	// Login
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"email": "bob@example.com", "password": "mypassword",
	}))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotEmpty(t, body["accessToken"])
	assert.NotEmpty(t, body["expiresAt"])

	// Refresh cookie should be set
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == refreshCookieName {
			found = true
			assert.True(t, c.HttpOnly)
		}
	}
	assert.True(t, found, "refresh_token cookie not set")
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"email": "nobody@example.com", "password": "wrong",
	}))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	h := setupAuthHandler(t)

	// Register + login to get tokens
	h.Register(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/auth/register", jsonBody(t, map[string]string{
		"email": "carol@example.com", "password": "pw",
	})))
	loginRR := httptest.NewRecorder()
	h.Login(loginRR, httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"email": "carol@example.com", "password": "pw",
	})))

	var loginBody map[string]any
	require.NoError(t, json.NewDecoder(loginRR.Body).Decode(&loginBody))

	// Extract refresh token from cookie
	var refreshToken string
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == refreshCookieName {
			refreshToken = c.Value
		}
	}
	require.NotEmpty(t, refreshToken)

	// Refresh
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": refreshToken,
	}))
	rr := httptest.NewRecorder()
	h.Refresh(rr, refreshReq)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotEmpty(t, body["accessToken"])
}

func TestAuthHandler_Refresh_MissingToken(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", jsonBody(t, map[string]string{}))
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthHandler_Logout(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAuthHandler_Me_Authenticated(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, testJWTSecret)
	h := NewAuthHandler(authSvc)

	// Create user directly
	user, err := authSvc.Register(t.Context(), "dave@example.com", "pw")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = req.WithContext(internal.WithUserID(req.Context(), user.ID))
	rr := httptest.NewRecorder()
	h.Me(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "dave@example.com", body["email"])
}

func TestAuthHandler_Me_Unauthenticated(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rr := httptest.NewRecorder()
	h.Me(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	h := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": "nonexistent-fake-token",
	}))
	rr := httptest.NewRecorder()
	h.Refresh(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthHandler_Refresh_ViaCookie(t *testing.T) {
	h := setupAuthHandler(t)

	h.Register(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/auth/register", jsonBody(t, map[string]string{
		"email": "eve@example.com", "password": "pw",
	})))
	loginRR := httptest.NewRecorder()
	h.Login(loginRR, httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"email": "eve@example.com", "password": "pw",
	})))

	var refreshToken string
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == refreshCookieName {
			refreshToken = c.Value
		}
	}
	require.NotEmpty(t, refreshToken)

	refreshReq := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	refreshReq.AddCookie(&http.Cookie{
		Name:  refreshCookieName,
		Value: refreshToken,
	})
	rr := httptest.NewRecorder()
	h.Refresh(rr, refreshReq)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.NotEmpty(t, body["accessToken"])
}

func TestAuthHandler_Me_DBError(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, testJWTSecret)
	h := NewAuthHandler(authSvc)

	// Create a user so we have a valid ID
	user, err := authSvc.Register(t.Context(), "frank@example.com", "pw")
	require.NoError(t, err)

	// Close DB to trigger error on GetUser
	require.NoError(t, db.Close())

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = req.WithContext(internal.WithUserID(req.Context(), user.ID))
	rr := httptest.NewRecorder()
	h.Me(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
