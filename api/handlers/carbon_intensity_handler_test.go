package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/carbonintensity"
)

func newTestCarbonIntensityClient(handler http.HandlerFunc) *carbonintensity.Client {
	server := httptest.NewServer(handler)
	client := carbonintensity.NewClient()
	client.SetBaseURL(server.URL)
	return client
}

func TestCarbonIntensityHandler_GetCurrent_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": [{
				"from": "2024-01-01T12:00Z",
				"to": "2024-01-01T12:30Z",
				"intensity": {
					"forecast": 200,
					"actual": 195,
					"index": "low"
				}
			}]
		}`))
	})
	client := newTestCarbonIntensityClient(handler)
	h := NewCarbonIntensityHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/carbon-intensity", nil)
	rec := httptest.NewRecorder()

	h.GetCurrent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var result carbonintensity.CarbonIntensity
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Forecast != 200 {
		t.Errorf("expected forecast 200, got %d", result.Forecast)
	}
}

func TestCarbonIntensityHandler_GetCurrent_APIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	client := newTestCarbonIntensityClient(handler)
	h := NewCarbonIntensityHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/carbon-intensity", nil)
	rec := httptest.NewRecorder()

	h.GetCurrent(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
}
