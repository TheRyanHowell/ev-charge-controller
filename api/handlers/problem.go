package handlers

import (
	"encoding/json"
	"net/http"

	"ev-charge-controller/api/middleware"
)

// debugResponsesEnabled controls whether the "debug" field is included in
// problem+json responses. Set once at startup via EnableDebugResponses.
var debugResponsesEnabled bool

// EnableDebugResponses opts in to including internal error detail in problem
// responses. Call this from main when Config.IsDev() is true.
func EnableDebugResponses() {
	debugResponsesEnabled = true
}

// ProblemDetails is an RFC 7807 Problem Details response with optional debug field.
type ProblemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Debug  string `json:"debug,omitempty"`
}

func problemJSON(w http.ResponseWriter, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:   errType,
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func problemJSONDebug(w http.ResponseWriter, status int, errType, title, detail, debug string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	pd := ProblemDetails{
		Type:   errType,
		Title:  title,
		Status: status,
		Detail: detail,
	}

	if debugResponsesEnabled {
		pd.Debug = debug
	}

	_ = json.NewEncoder(w).Encode(pd)
}

// logReq returns slog attributes for the request's correlation ID and path.
func logReq(r *http.Request) []any {
	id := middleware.GetRequestID(r.Context())
	if id == "" {
		return []any{"path", r.URL.Path}
	}
	return []any{"req_id", id, "path", r.URL.Path}
}
