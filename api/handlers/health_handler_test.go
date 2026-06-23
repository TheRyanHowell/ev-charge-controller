package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHealthChecker struct {
	pingErr error
}

func (m *mockHealthChecker) Ping(ctx context.Context) error {
	return m.pingErr
}

func TestHealthHandler_OK(t *testing.T) {
	checker := &mockHealthChecker{}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	HealthHandler(checker)(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
}

func TestHealthHandler_DBError(t *testing.T) {
	checker := &mockHealthChecker{pingErr: errors.New("database unavailable")}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	HealthHandler(checker)(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var result ProblemDetails
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "about:blank#db-unavailable", result.Type)
	assert.Equal(t, "Service Unavailable", result.Title)
	assert.Equal(t, http.StatusServiceUnavailable, result.Status)
	assert.Equal(t, "Database health check failed", result.Detail)
}

func TestNewDBHealthChecker(t *testing.T) {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	defer db.Close()

	checker := NewDBHealthChecker(db)
	require.NotNil(t, checker)

	err = checker.Ping(t.Context())
	assert.NoError(t, err)
}

func TestHealthHandler_ContextCancellation(t *testing.T) {
	checker := &mockHealthChecker{pingErr: context.DeadlineExceeded}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	HealthHandler(checker)(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
