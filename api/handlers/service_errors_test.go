package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
)

func TestMapServiceError_NilReturnsFalse(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, mapServiceError(rr, req, nil))
	assert.Equal(t, http.StatusOK, rr.Code) // nothing written
}

func TestMapServiceError_KnownSentinels(t *testing.T) {
	tests := map[error]int{
		services.ErrSessionNotFound:           http.StatusNotFound,
		services.ErrSessionNotActive:          http.StatusConflict,
		services.ErrActiveSessionExists:       http.StatusConflict,
		services.ErrCannotDeleteActiveSession: http.StatusConflict,
		services.ErrTargetOutOfRange:          http.StatusBadRequest,
		services.ErrTargetBelowStart:          http.StatusBadRequest,
		services.ErrTargetBelowCurrent:        http.StatusBadRequest,
		services.ErrVehicleConfigMissing:      http.StatusBadRequest,
		services.ErrVehicleHasNoBattery:       http.StatusBadRequest,
		services.ErrVehicleNotFound:           http.StatusNotFound,
	}

	for sentinel, wantStatus := range tests {
		t.Run(sentinel.Error(), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			// A wrapped sentinel must still match via errors.Is.
			rr := httptest.NewRecorder()
			assert.True(t, mapServiceError(rr, req, fmt.Errorf("context: %w", sentinel)))
			assert.Equal(t, wantStatus, rr.Code)
			assert.Equal(t, "application/problem+json", rr.Header().Get("Content-Type"))
		})
	}
}

func TestMapServiceError_UnknownErrorIs500(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.True(t, mapServiceError(rr, req, errors.New("boom")))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
