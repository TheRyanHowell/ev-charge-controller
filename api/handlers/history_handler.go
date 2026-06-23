package handlers

import (
	"log/slog"
	"net/http"

	"ev-charge-controller/api/services"
)

// HistoryHandler serves charge session history.
type HistoryHandler struct {
	service *services.HistoryService
}

func NewHistoryHandler(service *services.HistoryService) *HistoryHandler {
	return &HistoryHandler{
		service: service,
	}
}

func (h *HistoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicleId")
	date := r.URL.Query().Get("date")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// At least one of date or limit is required
	if date == "" && limitStr == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank", "Bad Request", "date or limit parameter is required")
		return
	}

	limit, offset, parseErr := parsePaginationParams(limitStr, offsetStr)
	if parseErr != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-param", "Bad Request", parseErr.Error())
		return
	}

	params := services.HistoryQueryParams{
		VehicleID: vehicleID,
		Date:      date,
		Limit:     limit,
		Offset:    offset,
	}

	responses, err := h.service.GetHistory(r.Context(), params)
	if err != nil {
		slog.Error("failed to fetch charge history", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching charge history.", err.Error())
		return
	}
	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := writeJSON(w, http.StatusOK, responses); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

const defaultHistoryLimit = 100

// parsePaginationParams validates and parses limit and offset query parameters.
func parsePaginationParams(limitStr, offsetStr string) (limit, offset int, err error) {
	limit, err = validateLimit(limitStr, defaultHistoryLimit)
	if err != nil {
		return 0, 0, err
	}

	offset, err = validateOffset(offsetStr)
	if err != nil {
		return 0, 0, err
	}

	return limit, offset, nil
}
