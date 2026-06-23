package handlers

import (
	"log/slog"
	"net/http"

	"ev-charge-controller/api/services"
)

// SOCSnapshotsHandler serves SOC snapshots for a charge session.
type SOCSnapshotsHandler struct {
	service *services.ChartDataService
}

func NewSOCSnapshotsHandler(service *services.ChartDataService) *SOCSnapshotsHandler {
	return &SOCSnapshotsHandler{service: service}
}

func (h *SOCSnapshotsHandler) GetSnapshots(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	vehicleID := r.URL.Query().Get("vehicleId")

	snapshots, err := h.service.GetSOCSnapshots(r.Context(), sessionID, vehicleID)
	if err != nil {
		slog.Error("failed to get SOC snapshots", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching battery level data.", err.Error())
		return
	}

	if len(snapshots) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := writeJSON(w, http.StatusOK, snapshots); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}
