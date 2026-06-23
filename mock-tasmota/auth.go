package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

func (h *TasmotaHandler) checkAuth(r *http.Request, w http.ResponseWriter) bool {
	if h.authUsername == "" || h.authPassword == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization required"})
		return false
	}
	const prefix = "Basic "
	if !strings.HasPrefix(authHeader, prefix) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid authorization header"})
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, prefix))
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials encoding"})
		return false
	}
	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 || creds[0] != h.authUsername || creds[1] != h.authPassword {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return false
	}
	return true
}
