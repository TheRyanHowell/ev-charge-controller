package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

// TariffHandler serves the current user's electricity tariff settings.
type TariffHandler struct {
	service *services.TariffService
}

func NewTariffHandler(service *services.TariffService) *TariffHandler {
	return &TariffHandler{service: service}
}

// Get handles GET /api/tariff-settings.
func (h *TariffHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		slog.Error("failed to get tariff settings", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while fetching tariff settings.", err.Error())
		return
	}
	if err := writeJSON(w, http.StatusOK, settings); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}

// Put handles PUT /api/tariff-settings, replacing the user's tariff wholesale.
func (h *TariffHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req models.TariffSettings
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	if err := h.service.UpdateSettings(r.Context(), &req); err != nil {
		if errors.Is(err, services.ErrInvalidTariff) {
			problemJSON(w, http.StatusBadRequest, "about:blank#invalid-tariff", "Bad Request", err.Error())
			return
		}
		slog.Error("failed to update tariff settings", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while saving tariff settings.", err.Error())
		return
	}

	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		slog.Error("failed to reload tariff settings", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Tariff saved but could not be reloaded.", err.Error())
		return
	}
	if err := writeJSON(w, http.StatusOK, settings); err != nil {
		slog.Error("error encoding response", append(logReq(r), "err", err)...)
	}
}
