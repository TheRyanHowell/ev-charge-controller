package middleware

import (
	"net/http"
	"strings"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/services"
)

// RequireAuth validates a JWT from the Authorization: Bearer header or access_token
// cookie, injects the userID into the request context, and rejects unauthenticated
// requests with 401.
func RequireAuth(auth *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w)
				return
			}
			userID, err := auth.ValidateAccessToken(token)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(internal.WithUserID(r.Context(), userID)))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="ev-charge-controller"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"type":"about:blank","title":"Unauthorized","status":401,"detail":"authentication required"}`))
}
