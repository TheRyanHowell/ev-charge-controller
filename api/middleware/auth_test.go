package middleware

import (
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

const authTestSecret = "test-auth-middleware-secret"

func setupAuthMiddleware(t *testing.T) (*services.AuthService, func(http.Handler) http.Handler) {
	t.Helper()
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := services.NewAuthService(userRepo, tokenRepo, authTestSecret)
	return svc, RequireAuth(svc)
}

func TestRequireAuth_MissingToken_Returns401(t *testing.T) {
	_, mw := setupAuthMiddleware(t)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "problem+json")
	assert.NotEmpty(t, rr.Header().Get("WWW-Authenticate"))
}

func TestRequireAuth_InvalidToken_Returns401(t *testing.T) {
	_, mw := setupAuthMiddleware(t)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireAuth_ValidBearerToken_PassesThrough(t *testing.T) {
	svc, mw := setupAuthMiddleware(t)

	_, err := svc.Register(t.Context(), "valid@example.com", "pw")
	require.NoError(t, err)
	_, pair, err := svc.Login(t.Context(), "valid@example.com", "pw")
	require.NoError(t, err)

	var capturedID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID, _ = internal.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, capturedID)
}

func TestRequireAuth_ValidCookieToken_PassesThrough(t *testing.T) {
	svc, mw := setupAuthMiddleware(t)

	_, err := svc.Register(t.Context(), "cookie@example.com", "pw")
	require.NoError(t, err)
	_, pair, err := svc.Login(t.Context(), "cookie@example.com", "pw")
	require.NoError(t, err)

	var capturedID string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID, _ = internal.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: pair.AccessToken})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, capturedID)
}

func TestExtractBearerToken_BearerHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer mytoken123")
	assert.Equal(t, "mytoken123", extractBearerToken(req))
}

func TestExtractBearerToken_Cookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookietoken"})
	assert.Equal(t, "cookietoken", extractBearerToken(req))
}

func TestExtractBearerToken_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Empty(t, extractBearerToken(req))
}

func TestExtractBearerToken_BearerHeaderTakesPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer headertoken")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookietoken"})
	assert.Equal(t, "headertoken", extractBearerToken(req))
}
