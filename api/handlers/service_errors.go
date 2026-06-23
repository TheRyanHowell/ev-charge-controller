package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/services"
)

// problemSpec is the RFC 7807 response a service sentinel maps to.
type problemSpec struct {
	status int
	typ    string
	title  string
	detail string
}

// serviceErrorMappings is the single source of truth for service sentinel →
// HTTP status. Centralising it guarantees the same domain condition produces the
// same status across every handler. Concepts are grouped deliberately:
//   - "no session to act on" (not found / not active) → 404 / 409
//   - state conflicts (already active, cannot delete) → 409 Conflict
//   - invalid target values → 400 Bad Request
var serviceErrorMappings = []struct {
	err  error
	spec problemSpec
}{
	{services.ErrSessionNotFound, problemSpec{http.StatusNotFound, "about:blank#session-not-found", "Not Found", "Charge session not found."}},
	{services.ErrSessionNotActive, problemSpec{http.StatusConflict, "about:blank#session-not-active", "Conflict", "Session is not active."}},
	{services.ErrActiveSessionExists, problemSpec{http.StatusConflict, "about:blank#session-already-active", "Conflict", "A charge session is already in progress."}},
	{services.ErrCannotDeleteActiveSession, problemSpec{http.StatusConflict, "about:blank#cannot-delete-active", "Conflict", "Cannot delete a session that is currently active or pending."}},
	{services.ErrTargetOutOfRange, problemSpec{http.StatusBadRequest, "about:blank#invalid-target-percent", "Bad Request", "Charge target must be between 0 and 100."}},
	{services.ErrTargetBelowStart, problemSpec{http.StatusBadRequest, "about:blank#target-must-be-higher", "Bad Request", "Charge target must be higher than the starting battery level."}},
	{services.ErrTargetBelowCurrent, problemSpec{http.StatusBadRequest, "about:blank#target-must-be-higher", "Bad Request", "Charge target must be higher than the current battery level."}},
	{services.ErrVehicleConfigMissing, problemSpec{http.StatusBadRequest, "about:blank#vehicle-config-missing", "Bad Request", "Vehicle configuration is missing - please reselect your vehicle in Settings."}},
	{services.ErrVehicleNotFound, problemSpec{http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Selected vehicle not found."}},
}

// mapServiceError writes an RFC 7807 response for a service-layer error and
// reports whether it handled it. Returns false only for err == nil (caller
// proceeds with the success path). A recognised sentinel maps to its fixed
// status; any unrecognised error is logged and rendered as a 500.
func mapServiceError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	for _, m := range serviceErrorMappings {
		if errors.Is(err, m.err) {
			problemJSON(w, m.spec.status, m.spec.typ, m.spec.title, m.spec.detail)
			return true
		}
	}
	slog.Error("unhandled service error", append(logReq(r), "err", err)...)
	problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
	return true
}
