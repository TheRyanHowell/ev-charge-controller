package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/services"
)

const refreshCookieName = "refresh_token"

// refreshTokenMaxAge is the lifetime of the refresh token httpOnly cookie in seconds (30 days).
const refreshTokenMaxAge = 30 * 24 * 60 * 60

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	auth *services.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(auth *services.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Register handles POST /api/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid JSON")
		return
	}
	if req.Email == "" || req.Password == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-fields", "Bad Request", "email and password required")
		return
	}
	user, err := h.auth.Register(r.Context(), req.Email, req.Password)
	if errors.Is(err, services.ErrEmailTaken) {
		problemJSON(w, http.StatusConflict, "about:blank#email-taken", "Conflict", "email already registered")
		return
	}
	if err != nil {
		problemJSON(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "")
		return
	}
	_ = writeJSON(w, http.StatusCreated, user)
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid JSON")
		return
	}
	user, pair, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if errors.Is(err, services.ErrInvalidCredentials) {
		problemJSON(w, http.StatusUnauthorized, "about:blank#invalid-credentials", "Unauthorized", "invalid email or password")
		return
	}
	if err != nil {
		problemJSON(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "")
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	_ = writeJSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresAt":   pair.ExpiresAt,
		"user":        user,
	})
}

// Refresh handles POST /api/auth/refresh - rotates the refresh token and issues a new access token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawToken := h.extractRefreshToken(r)
	if rawToken == "" {
		problemJSON(w, http.StatusUnauthorized, "about:blank#missing-token", "Unauthorized", "refresh token required")
		return
	}
	pair, err := h.auth.Refresh(r.Context(), rawToken)
	if errors.Is(err, services.ErrInvalidToken) {
		problemJSON(w, http.StatusUnauthorized, "about:blank#invalid-token", "Unauthorized", "invalid or expired refresh token")
		return
	}
	if err != nil {
		problemJSON(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "")
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	_ = writeJSON(w, http.StatusOK, map[string]any{
		"accessToken": pair.AccessToken,
		"expiresAt":   pair.ExpiresAt,
	})
}

// Logout handles POST /api/auth/logout - revokes the refresh token.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	rawToken := h.extractRefreshToken(r)
	if rawToken != "" {
		_ = h.auth.Logout(r.Context(), rawToken)
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /api/auth/me - returns the authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "not authenticated")
		return
	}
	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil || user == nil {
		problemJSON(w, http.StatusNotFound, "about:blank#user-not-found", "Not Found", "user not found")
		return
	}
	_ = writeJSON(w, http.StatusOK, user)
}

// extractRefreshToken reads the refresh token from the httpOnly cookie,
// falling back to the request body.
func (h *AuthHandler) extractRefreshToken(r *http.Request) string {
	if c, err := r.Cookie(refreshCookieName); err == nil {
		return c.Value
	}
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.RefreshToken
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   refreshTokenMaxAge,
	})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
