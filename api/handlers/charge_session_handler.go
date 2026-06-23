package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

type ChargeSessionHandler struct {
	service *services.ChargeSessionService
}

func NewChargeSessionHandler(service *services.ChargeSessionService) *ChargeSessionHandler {
	return &ChargeSessionHandler{
		service: service,
	}
}

type StartRequest struct {
	PlugID        string  `json:"plugId"`
	VehicleID     string  `json:"vehicleId"`
	StartPercent  float64 `json:"startPercent"`
	TargetPercent float64 `json:"targetPercent"`
}

var (
	ErrInvalidStartPercent  = errors.New("starting battery level must be between 0 and 100")
	ErrInvalidTargetPercent = errors.New("charge target must be between 0 and 100")
	ErrTargetMustBeHigher   = errors.New("charge target must be higher than the current battery level")
	ErrVehicleIDRequired    = errors.New("no vehicle is selected - select one in Settings")
)

func (r *StartRequest) Validate() error {
	if r.VehicleID == "" {
		return ErrVehicleIDRequired
	}
	if r.StartPercent < 0 || r.StartPercent > models.MaxPercent {
		return ErrInvalidStartPercent
	}
	if r.TargetPercent < 0 || r.TargetPercent > models.MaxPercent {
		return ErrInvalidTargetPercent
	}
	if r.StartPercent >= r.TargetPercent {
		return ErrTargetMustBeHigher
	}
	return nil
}

func (h *ChargeSessionHandler) Start(w http.ResponseWriter, r *http.Request) {
	var req StartRequest
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	if err := req.Validate(); err != nil {
		switch {
		case errors.Is(err, ErrVehicleIDRequired):
			problemJSON(w, http.StatusBadRequest, "about:blank#missing-vehicle-id", "Bad Request", err.Error())
		case errors.Is(err, ErrInvalidStartPercent):
			problemJSON(w, http.StatusBadRequest, "about:blank#invalid-start-percent", "Bad Request", err.Error())
		case errors.Is(err, ErrInvalidTargetPercent):
			problemJSON(w, http.StatusBadRequest, "about:blank#invalid-target-percent", "Bad Request", err.Error())
		case errors.Is(err, ErrTargetMustBeHigher):
			problemJSON(w, http.StatusBadRequest, "about:blank#target-must-be-higher", "Bad Request", err.Error())
		}
		return
	}

	session, err := h.service.StartSession(r.Context(), req.PlugID, req.VehicleID, req.StartPercent, req.TargetPercent)
	if mapServiceError(w, r, err) {
		return
	}

	if err := writeJSON(w, http.StatusCreated, session); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

func (h *ChargeSessionHandler) Stop(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicleId")
	if vehicleID == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-vehicle-id", "Bad Request", "vehicleId query parameter is required.")
		return
	}

	result, err := h.service.StopByVehicle(r.Context(), vehicleID)
	if errors.Is(err, services.ErrSessionNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		slog.Error("failed to stop charge session", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while stopping the charge session.", err.Error())
		return
	}
	if !result.Stopped {
		slog.Warn("could not control smart plug relay", "tasmotaErr", result.TasmotaErr)
		problemJSONDebug(w, http.StatusServiceUnavailable, "about:blank#relay-control-failed", "Service Unavailable",
			"Could not control the smart plug. Please check that it is powered on and connected.", result.TasmotaErr)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ChargeSessionHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicleId")
	if vehicleID == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-vehicle-id", "Bad Request", "vehicleId query parameter is required.")
		return
	}

	session, err := h.service.GetActiveByVehicle(r.Context(), vehicleID)
	if err != nil {
		slog.Error("failed to get active charge session", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching the charge session.", err.Error())
		return
	}
	if session == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := writeJSON(w, http.StatusOK, session); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

func (h *ChargeSessionHandler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetPercent float64 `json:"targetPercent"`
		Status        string  `json:"status"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	// Route to stop if status is "stopped"
	if req.Status == "stopped" {
		h.Stop(w, r)
		return
	}

	vehicleID := r.URL.Query().Get("vehicleId")
	if vehicleID == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-vehicle-id", "Bad Request", "vehicleId query parameter is required.")
		return
	}

	if req.TargetPercent < 0 || req.TargetPercent > models.MaxPercent {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-target-percent", "Bad Request", "Charge target must be between 0 and 100.")
		return
	}

	// The service resolves the active session and applies the update; "no active
	// session" surfaces as ErrSessionNotFound (404) consistently with other paths.
	if mapServiceError(w, r, h.service.UpdateActiveTargetByVehicle(r.Context(), vehicleID, req.TargetPercent)) {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ChargeSessionHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-session-id", "Bad Request", "Session ID is required.")
		return
	}

	if mapServiceError(w, r, h.service.DeleteSession(r.Context(), id)) {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
