package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

type PlugHandler struct {
	service    *services.MqttProvisioningService
	plugCtrl   internal.PlugController
	plugRepo   internal.PlugRepo
}

func NewPlugHandler(service *services.MqttProvisioningService) *PlugHandler {
	return &PlugHandler{service: service}
}

// SetPlugController wires in the MQTT controller for power toggle operations
// after the MQTT client connects at startup.
func (h *PlugHandler) SetPlugController(ctrl internal.PlugController, repo internal.PlugRepo) {
	h.plugCtrl = ctrl
	h.plugRepo = repo
}

// List handles GET /api/plugs - returns all plugs for the authenticated user.
func (h *PlugHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	plugs, err := h.service.ListPlugs(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list plugs", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	if plugs == nil {
		plugs = []models.Plug{}
	}
	if err := writeJSON(w, http.StatusOK, plugs); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// Create handles POST /api/plugs - creates a plug record. No MQTT password is generated yet.
func (h *PlugHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	var req struct {
		Name      string `json:"name"`
		MqttTopic string `json:"mqttTopic"`
		VehicleID string `json:"vehicleId"`
		Type      string `json:"type"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}
	if req.Name == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-name", "Bad Request", "name is required.")
		return
	}
	if req.Type != "" && req.Type != models.PlugTypeCharging && req.Type != models.PlugTypeMaintenance {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-type", "Bad Request", "type must be 'charging' or 'maintenance'.")
		return
	}

	plug, err := h.service.CreatePlug(r.Context(), userID, req.Name, "", req.VehicleID, req.Type)
	if err != nil {
		if errors.Is(err, services.ErrDuplicatePlugName) {
			problemJSON(w, http.StatusConflict, "about:blank#duplicate-name", "Conflict", "A plug with this name already exists.")
			return
		}
		slog.Error("failed to create plug", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	if err := writeJSON(w, http.StatusCreated, map[string]any{"plug": plug}); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// GetByID handles GET /api/plugs/{id} - returns a single plug by ID.
func (h *PlugHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	plug, err := h.service.GetPlug(r.Context(), userID, plugID)
	if err != nil {
		if errors.Is(err, services.ErrPlugNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#not-found", "Not Found", "Plug not found.")
			return
		}
		slog.Error("failed to get plug", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	if err := writeJSON(w, http.StatusOK, plug); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// Update handles PATCH /api/plugs/{id} - updates name and/or vehicle assignment.
func (h *PlugHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		VehicleID *string `json:"vehicleId"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	plug, err := h.service.UpdatePlug(r.Context(), userID, plugID, req.Name, req.VehicleID)
	if err != nil {
		if errors.Is(err, services.ErrPlugNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#not-found", "Not Found", "Plug not found.")
			return
		}
		slog.Error("failed to update plug", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	if err := writeJSON(w, http.StatusOK, plug); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// Delete handles DELETE /api/plugs/{id}.
func (h *PlugHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	if err := h.service.DeletePlug(r.Context(), userID, plugID); err != nil {
		if errors.Is(err, services.ErrPlugNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#not-found", "Not Found", "Plug not found.")
			return
		}
		slog.Error("failed to delete plug", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TogglePower handles PATCH /api/plugs/{id}/power - toggles the relay state of a
// maintenance plug. Only allowed on plugs with type == maintenance; charging plugs
// are controlled via charge sessions.
func (h *PlugHandler) TogglePower(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	var req struct {
		On bool `json:"on"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	plug, err := h.service.GetPlug(r.Context(), userID, plugID)
	if err != nil {
		if errors.Is(err, services.ErrPlugNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#not-found", "Not Found", "Plug not found.")
			return
		}
		slog.Error("failed to get plug", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong.", err.Error())
		return
	}

	if plug.Type != models.PlugTypeMaintenance {
		problemJSON(w, http.StatusBadRequest, "about:blank#wrong-plug-type", "Bad Request", "Power toggle is only allowed for maintenance plugs.")
		return
	}

	if h.plugCtrl == nil {
		problemJSON(w, http.StatusServiceUnavailable, "about:blank#mqtt-unavailable", "Service Unavailable", "MQTT not connected.")
		return
	}

	if _, err := h.plugCtrl.SetPowerAndWait(r.Context(), plugID, req.On, models.PowerConfirmationTimeout); err != nil {
		slog.Error("failed to set plug power", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Could not set power state.", err.Error())
		return
	}

	if h.plugRepo != nil {
		if err := h.plugRepo.SetPowerState(r.Context(), plugID, req.On); err != nil {
			slog.Warn("failed to persist power state", append(logReq(r), "err", err)...)
		}
	}

	plug.PowerOn = req.On
	if err := writeJSON(w, http.StatusOK, plug); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// ConfigureDevice handles POST /api/plugs/{id}/configure - provisions MQTT and
// optionally pushes config to a Tasmota device. If tasmotaIP is empty, only
// provisions MQTT and returns console commands. If tasmotaIP is set, also
// pushes commands to the device via HTTP.
func (h *PlugHandler) ConfigureDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := internal.UserIDFromContext(r.Context())
	if !ok {
		problemJSON(w, http.StatusUnauthorized, "about:blank#unauthorized", "Unauthorized", "Authentication required.")
		return
	}

	plugID := r.PathValue("id")
	if !isValidID(plugID) {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-id", "Bad Request", "Invalid plug ID.")
		return
	}

	var req struct {
		TasmotaIP     string `json:"tasmotaIP"`
		TasmotaPasswd string `json:"tasmotaPassword"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	consoleCommands, err := h.service.ConfigureTasmotaDevice(r.Context(), userID, plugID, req.TasmotaIP, req.TasmotaPasswd)
	if err != nil {
		if errors.Is(err, services.ErrPlugNotFound) {
			problemJSON(w, http.StatusNotFound, "about:blank#not-found", "Not Found", "Plug not found.")
			return
		}
		slog.Error("failed to configure tasmota device", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Could not configure device: "+err.Error(), err.Error())
		return
	}

	if err := writeJSON(w, http.StatusOK, map[string]string{"consoleCommands": consoleCommands}); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}
