package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

type ScheduleHandler struct {
	service *services.ScheduleService
}

func NewScheduleHandler(service *services.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{service: service}
}

func (h *ScheduleHandler) Service() *services.ScheduleService {
	return h.service
}

// attachEstimatedStart populates EstimatedStartTime (single-stage) or
// EstimatedPlan (two-stage) on enabled carbon-aware schedules using the
// current carbon intensity forecast, so the UI can show when charging is
// actually expected to happen instead of just the ready-by deadline.
func (h *ScheduleHandler) attachEstimatedStart(ctx context.Context, schedule *models.Schedule) {
	if schedule.TwoStage {
		if plan, ok := h.service.EstimateCarbonAwareTwoStagePlan(ctx, schedule); ok {
			schedule.EstimatedPlan = &plan
		}
		return
	}
	if start, ok := h.service.EstimateCarbonAwareStart(ctx, schedule); ok {
		schedule.EstimatedStartTime = &start
	}
}

// UpsertByPlug handles PATCH /api/plugs/{id}/schedule.
func (h *ScheduleHandler) UpsertByPlug(w http.ResponseWriter, r *http.Request) {
	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	var req struct {
		Type        string  `json:"type"`
		Time        string  `json:"time"`
		WindowStart string  `json:"windowStart"`
		WindowEnd   string  `json:"windowEnd"`
		ReadyBy     *string `json:"readyBy"`
		Enabled     bool    `json:"enabled"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	// Default to "daily" when type is omitted (backward compat).
	if req.Type == "" {
		req.Type = string(models.ScheduleTypeDaily)
	}

	userID, _ := internal.UserIDFromContext(r.Context())

	var schedule *models.Schedule
	var err error

	switch models.ScheduleType(req.Type) {
	case models.ScheduleTypeDaily:
		schedule, err = h.service.UpsertByPlugID(r.Context(), plugID, userID, req.Time, req.ReadyBy, req.Enabled)
	case models.ScheduleTypeCarbonAware:
		schedule, err = h.service.UpsertCarbonAware(r.Context(), plugID, userID, req.WindowStart, req.WindowEnd, req.Enabled)
	default:
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-schedule-type", "Bad Request", "Schedule type must be 'daily' or 'carbon_aware'.")
		return
	}

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidScheduleTime):
			problemJSONDebug(w, http.StatusBadRequest, "about:blank#invalid-schedule-time", "Bad Request", "Schedule time must be in HH:MM format (e.g., 06:30).", err.Error())
		case errors.Is(err, services.ErrWindowRequired):
			problemJSONDebug(w, http.StatusBadRequest, "about:blank#window-required", "Bad Request", "windowStart and windowEnd are required for carbon_aware schedule.", err.Error())
		case errors.Is(err, services.ErrWindowEqual):
			problemJSONDebug(w, http.StatusBadRequest, "about:blank#window-equal", "Bad Request", "windowStart and windowEnd must differ.", err.Error())
		case errors.Is(err, services.ErrReadyByEqualsTime):
			problemJSONDebug(w, http.StatusBadRequest, "about:blank#ready-by-equal", "Bad Request", "readyBy must differ from time.", err.Error())
		case errors.Is(err, services.ErrMaintenancePlugSchedule):
			problemJSON(w, http.StatusBadRequest, "about:blank#maintenance-plug-schedule", "Bad Request", "Schedules are not supported for maintenance plugs.")
		default:
			problemJSONDebug(w, http.StatusBadRequest, "about:blank#invalid-schedule", "Bad Request", "Invalid schedule.", err.Error())
		}
		return
	}

	h.attachEstimatedStart(r.Context(), schedule)

	if err := writeJSON(w, http.StatusOK, schedule); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// GetByPlug handles GET /api/plugs/{id}/schedule.
func (h *ScheduleHandler) GetByPlug(w http.ResponseWriter, r *http.Request) {
	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	schedule, err := h.service.GetByPlugID(r.Context(), plugID)
	if err != nil {
		slog.Error("failed to get schedule", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching the schedule.", err.Error())
		return
	}

	if schedule == nil {
		problemJSON(w, http.StatusNotFound, "about:blank#schedule-not-found", "Not Found", "Schedule not found.")
		return
	}

	h.attachEstimatedStart(r.Context(), schedule)

	if err := writeJSON(w, http.StatusOK, schedule); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}
