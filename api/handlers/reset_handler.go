package handlers

import (
	"log/slog"
	"net/http"

	"ev-charge-controller/api/services"
)

// ResetHandler handles POST /api/reset - resets the database to seed state
// and resets mock-tasmota instances. Dev-only endpoint.
type ResetHandler struct {
	seedSvc *services.SeedService
}

// NewResetHandler creates a new ResetHandler.
func NewResetHandler(seedSvc *services.SeedService) *ResetHandler {
	return &ResetHandler{seedSvc: seedSvc}
}

// Reset resets the database and mock-tasmota to a known seed state.
// This is a dev-only endpoint; it should never be exposed in production.
func (h *ResetHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problemJSONDebug(w, http.StatusMethodNotAllowed, "about:blank#method-not-allowed", "Method Not Allowed", "Only POST is allowed", "")
		return
	}

	slog.Info("Reset endpoint called - resetting database and mock-tasmota")

	if err := h.seedSvc.Reset(); err != nil {
		slog.Error("Reset failed", "err", err)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Reset Failed", err.Error(), err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"reset complete"}`))
}
