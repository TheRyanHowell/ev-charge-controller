package handlers

import (
	"log/slog"
	"net/http"

	"ev-charge-controller/api/services"
)

// PowerReadingsHandler serves power readings for a charge session.
type PowerReadingsHandler struct {
	service *services.ChartDataService
}

func NewPowerReadingsHandler(service *services.ChartDataService) *PowerReadingsHandler {
	return &PowerReadingsHandler{service: service}
}

func (h *PowerReadingsHandler) GetReadings(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if err := validateSessionID(sessionID); err != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank#bad-request", "Bad Request", err.Error())
		return
	}

	vehicleID := r.URL.Query().Get("vehicleId")
	if err := validateVehicleID(vehicleID); err != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank#bad-request", "Bad Request", err.Error())
		return
	}

	readings, err := h.service.GetPowerReadings(r.Context(), sessionID, vehicleID)
	if err != nil {
		slog.Error("failed to get power readings", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching power readings.", err.Error())
		return
	}

	if len(readings) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := writeJSON(w, http.StatusOK, readings); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}
