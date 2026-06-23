package middleware

import (
	"net/http"
	"os"
)

const defaultCorsOrigin = "http://localhost:3000"

// GetCorsOrigin returns the configured CORS origin from the CORS_ORIGIN env var.
func GetCorsOrigin() string {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		return defaultCorsOrigin
	}
	return origin
}

// setCorsHeaders validates the request Origin and sets CORS headers.
// Returns true if the origin is allowed.
func setCorsHeaders(w http.ResponseWriter, r *http.Request, allowedOrigin string) bool {
	requestOrigin := r.Header.Get("Origin")
	if requestOrigin == "" {
		// No Origin header - same-origin request or curl, allow it
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		return true
	}
	if requestOrigin != allowedOrigin {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", requestOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	return true
}

// CorsHandler wraps any http.Handler with CORS headers.
// Works on the mux level so OPTIONS preflight requests get headers
// before ServeMux returns 405.
func CorsHandler(handler http.Handler) http.Handler {
	allowedOrigin := GetCorsOrigin()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !setCorsHeaders(w, r, allowedOrigin) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
