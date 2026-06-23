package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

// maxPushEndpointLen bounds the accepted push endpoint URL length.
const maxPushEndpointLen = 2048

var errInvalidPushEndpoint = errors.New("push endpoint must be an absolute https URL")

// validatePushEndpoint ensures a client-supplied push endpoint is a well-formed,
// bounded, absolute https URL with a host. The server later POSTs to it, so an
// unvalidated value would be a server-side request forgery (SSRF) vector.
func validatePushEndpoint(endpoint string) error {
	if endpoint == "" || len(endpoint) > maxPushEndpointLen {
		return errInvalidPushEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errInvalidPushEndpoint
	}
	return nil
}

type PushHandler struct {
	service *services.PushService
}

func NewPushHandler(service *services.PushService) *PushHandler {
	return &PushHandler{
		service: service,
	}
}

func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Endpoint string `json:"endpoint"`
		P256dh   string `json:"p256dhKey"`
		Auth     string `json:"authKey"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	if req.P256dh == "" || req.Auth == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-fields", "Bad Request", "Push subscription details are incomplete. Please try subscribing again.")
		return
	}

	// The server later POSTs to this endpoint, so an unvalidated value is an
	// SSRF vector. Require a well-formed absolute https URL of bounded length.
	if err := validatePushEndpoint(req.Endpoint); err != nil {
		problemJSON(w, http.StatusBadRequest, "about:blank#invalid-endpoint", "Bad Request", "Push subscription endpoint is invalid.")
		return
	}

	id := req.ID
	if id == "" {
		id = uuid.New().String()
	}

	userID, _ := internal.UserIDFromContext(r.Context())
	sub := &models.PushSubscription{
		ID:        id,
		Endpoint:  req.Endpoint,
		P256dhKey: req.P256dh,
		AuthKey:   req.Auth,
	}
	if userID != "" {
		sub.UserID = &userID
	}

	if err := h.service.UpsertSubscription(r.Context(), sub); err != nil {
		slog.Error("failed to save push subscription", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while saving your notification preferences.", err.Error())
		return
	}

	_ = writeJSON(w, http.StatusCreated, sub)
}

func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodeJSONStrict(w, r, &req) {
		return
	}

	if req.Endpoint == "" {
		problemJSON(w, http.StatusBadRequest, "about:blank#missing-fields", "Bad Request", "Push subscription endpoint is required.")
		return
	}

	if err := h.service.RemoveSubscriptionByEndpoint(r.Context(), req.Endpoint); err != nil {
		slog.Error("failed to remove push subscription", append(logReq(r), "err", err)...)
		problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "Something went wrong while removing your notification preferences.", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
