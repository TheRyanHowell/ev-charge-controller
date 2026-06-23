package handlers

import (
	"log/slog"
	"net/http"

	"ev-charge-controller/api/services"
)

// VehicleStatsHandler serves vehicle statistics.
type VehicleStatsHandler struct {
	service *services.VehicleStatsService
}

func NewVehicleStatsHandler(service *services.VehicleStatsService) *VehicleStatsHandler {
	return &VehicleStatsHandler{
		service: service,
	}
}

func (h *VehicleStatsHandler) GetStats(w http.ResponseWriter, r *http.Request, vehicleID string) {
	if vehicleID == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-vehicle-id", "Bad Request", "Vehicle ID is required.")
		return
	}

	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = string(services.TimeRangeLifetime)
	}

	params := services.VehicleStatsQueryParams{
		VehicleID: vehicleID,
		Range:     services.TimeRange(rangeStr),
	}

	stats, err := h.service.GetStats(r.Context(), params)
	if err != nil {
		slog.Error("failed to fetch vehicle stats", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching vehicle statistics.", err.Error())
		return
	}

	if err := writeJSON(w, http.StatusOK, stats); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// GetAllStats handles GET /api/vehicles/stats - returns summary stats for all vehicles.
func (h *VehicleStatsHandler) GetAllStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetAllVehiclesStats(r.Context())
	if err != nil {
		slog.Error("failed to fetch all vehicle stats", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching vehicle statistics.", err.Error())
		return
	}

	if stats == nil {
		stats = []services.VehicleSummaryStats{}
	}
	if err := writeJSON(w, http.StatusOK, stats); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}
