package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"ev-charge-controller/api/carbonintensity"
)

// CarbonIntensityHandler serves the current carbon intensity.
type CarbonIntensityHandler struct {
	client *carbonintensity.Client
}

func NewCarbonIntensityHandler(client *carbonintensity.Client) *CarbonIntensityHandler {
	return &CarbonIntensityHandler{client: client}
}

func (h *CarbonIntensityHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	intensity, err := h.client.GetCurrent(r.Context())
	if err != nil {
		slog.Warn("Failed to fetch carbon intensity", "err", err)
		problemJSON(w, http.StatusServiceUnavailable, "about:blank", "Unavailable", "carbon intensity API unavailable")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(intensity); err != nil {
		slog.Error("Error encoding carbon intensity response", append(logReq(r), "err", err)...)
	}
}
