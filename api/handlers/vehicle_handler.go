package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

type VehicleHandler struct {
	service *services.VehicleService
}

func NewVehicleHandler(service *services.VehicleService) *VehicleHandler {
	return &VehicleHandler{service: service}
}

// ListModels handles GET /api/vehicle-models - returns the global catalog.
func (h *VehicleHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.service.ListModels(r.Context())
	if err != nil {
		slog.Error("failed to list vehicle models", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}
	if err := writeJSON(w, http.StatusOK, models); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// List handles GET /api/vehicles - returns the user's vehicle instances.
func (h *VehicleHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicles, err := h.service.List(r.Context())
	if err != nil {
		slog.Error("failed to list vehicles", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching vehicles.", err.Error())
		return
	}
	if vehicles == nil {
		vehicles = []models.Vehicle{}
	}
	if err := writeJSON(w, http.StatusOK, vehicles); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// Create handles POST /api/vehicles - creates a new instance from a catalog model.
func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	var req struct {
		ModelID string `json:"modelId"`
		Name    string `json:"name"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}
	if req.ModelID == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-field", "Bad Request", "modelId is required.")
		return
	}

	v, err := h.service.AddVehicle(r.Context(), userID, req.ModelID, req.Name)
	if err != nil {
		if errors.Is(err, services.ErrVehicleModelNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#model-not-found", "Not Found", "Vehicle model not found.")
			return
		}
		if errors.Is(err, services.ErrDuplicateVehicleName) {
			problemJSON(w, http.StatusConflict, "about:blank#duplicate-name", "Conflict", "A vehicle with this name already exists.")
			return
		}
		slog.Error("failed to create vehicle", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	if err := writeJSON(w, http.StatusCreated, v); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// Delete handles DELETE /api/vehicles/{id} - removes a user's vehicle instance.
func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	id := r.PathValue("id")
	if !isValidID(id) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid vehicle ID.")
		return
	}

	if err := h.service.DeleteVehicle(r.Context(), userID, id); err != nil {
		if errors.Is(err, services.ErrVehicleNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Vehicle not found.")
			return
		}
		slog.Error("failed to delete vehicle", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Patch handles PATCH /api/vehicles/{id} - updates SoC percents, name, or notification prefs.
func (h *VehicleHandler) Patch(w http.ResponseWriter, r *http.Request, id string) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	var req struct {
		Name                    *string  `json:"name"`
		CurrentPercent          *float64 `json:"currentPercent"`
		TargetPercent           *float64 `json:"targetPercent"`
		NotifyChargeComplete    *bool    `json:"notifyChargeComplete"`
		NotifyChargerOffline    *bool    `json:"notifyChargerOffline"`
		NotifyMaintenanceOffline *bool   `json:"notifyMaintenanceOffline"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	if req.Name == nil && req.CurrentPercent == nil && req.TargetPercent == nil &&
		req.NotifyChargeComplete == nil && req.NotifyChargerOffline == nil && req.NotifyMaintenanceOffline == nil {
		problemJSON(w, http.StatusBadRequest, "about:blank#no-op", "Bad Request", "Request must include at least one field.")
		return
	}

	ctx := r.Context()

	if req.Name != nil {
		if err := h.service.UpdateName(ctx, id, *req.Name); err != nil {
			switch {
			case errors.Is(err, services.ErrVehicleNotFound):
				problemJSON(w, http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Vehicle not found.")
			case errors.Is(err, services.ErrDuplicateVehicleName):
				problemJSON(w, http.StatusConflict, "about:blank#duplicate-name", "Conflict", "A vehicle with this name already exists.")
			default:
				problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
			}
			return
		}
	}

	if req.CurrentPercent != nil || req.TargetPercent != nil {
		if err := h.service.UpdatePercents(ctx, id, req.CurrentPercent, req.TargetPercent); err != nil {
			switch {
			case errors.Is(err, services.ErrCurrentLockedDuringSession):
				problemJSON(w, http.StatusConflict, "about:blank#session-active", "Conflict", "Current battery level cannot be changed while a charge session is active.")
			case errors.Is(err, services.ErrVehicleNotFound):
				problemJSON(w, http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Vehicle not found.")
			case errors.Is(err, services.ErrCurrentPercentOutOfRange):
				problemJSON(w, http.StatusBadRequest, "about:blank#invalid-percent", "Bad Request", "Current battery level must be between 0 and 100.")
			case errors.Is(err, services.ErrTargetPercentOutOfRange):
				problemJSON(w, http.StatusBadRequest, "about:blank#invalid-target-percent", "Bad Request", "Charge target must be between 100.")
			case errors.Is(err, services.ErrCurrentExceedsTarget):
				problemJSON(w, http.StatusBadRequest, "about:blank#target-must-be-higher", "Bad Request", "Current battery level must be less than or equal to the charge target.")
			default:
				problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
			}
			return
		}
	}

	if req.NotifyChargeComplete != nil || req.NotifyChargerOffline != nil || req.NotifyMaintenanceOffline != nil {
		// Read current prefs to merge with the partial update.
		v, err := h.service.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, services.ErrVehicleNotFound) {
				problemJSON(w, http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Vehicle not found.")
			} else {
				problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
			}
			return
		}
		ncc := v.NotifyChargeComplete
		nco := v.NotifyChargerOffline
		nmo := v.NotifyMaintenanceOffline
		if req.NotifyChargeComplete != nil {
			ncc = *req.NotifyChargeComplete
		}
		if req.NotifyChargerOffline != nil {
			nco = *req.NotifyChargerOffline
		}
		if req.NotifyMaintenanceOffline != nil {
			nmo = *req.NotifyMaintenanceOffline
		}
		if err := h.service.UpdateNotificationPrefs(ctx, userID, id, ncc, nco, nmo); err != nil {
			if errors.Is(err, services.ErrVehicleNotFound) {
				problemJSON(w, http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Vehicle not found.")
				return
			}
			problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetByID handles GET /api/vehicles/{id}.
func (h *VehicleHandler) GetByID(w http.ResponseWriter, r *http.Request, id string) {
	vehicle, err := h.service.FindByID(r.Context(), id)
	if errors.Is(err, services.ErrVehicleNotFound) {
		problemJSON(w, http.StatusNotFound, "about:blank#vehicle-not-found", "Not Found", "Vehicle not found.")
		return
	}
	if err != nil {
		slog.Error("failed to get vehicle", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching the vehicle.", err.Error())
		return
	}
	if err := writeJSON(w, http.StatusOK, vehicle); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}


