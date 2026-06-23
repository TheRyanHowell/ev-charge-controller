package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     StartRequest
		wantErr error
	}{
		{
			name:    "valid request",
			req:     StartRequest{VehicleID: "v1", StartPercent: 20, TargetPercent: 80},
			wantErr: nil,
		},
		{
			name:    "missing vehicle ID",
			req:     StartRequest{StartPercent: 20, TargetPercent: 80},
			wantErr: ErrVehicleIDRequired,
		},
		{
			name:    "negative start percent",
			req:     StartRequest{VehicleID: "v1", StartPercent: -1, TargetPercent: 80},
			wantErr: ErrInvalidStartPercent,
		},
		{
			name:    "start percent over 100",
			req:     StartRequest{VehicleID: "v1", StartPercent: 101, TargetPercent: 80},
			wantErr: ErrInvalidStartPercent,
		},
		{
			name:    "negative target percent",
			req:     StartRequest{VehicleID: "v1", StartPercent: 20, TargetPercent: -1},
			wantErr: ErrInvalidTargetPercent,
		},
		{
			name:    "target percent over 100",
			req:     StartRequest{VehicleID: "v1", StartPercent: 20, TargetPercent: 101},
			wantErr: ErrInvalidTargetPercent,
		},
		{
			name:    "start equals target",
			req:     StartRequest{VehicleID: "v1", StartPercent: 50, TargetPercent: 50},
			wantErr: ErrTargetMustBeHigher,
		},
		{
			name:    "start greater than target",
			req:     StartRequest{VehicleID: "v1", StartPercent: 80, TargetPercent: 20},
			wantErr: ErrTargetMustBeHigher,
		},
		{
			name:    "boundary zero start",
			req:     StartRequest{VehicleID: "v1", StartPercent: 0, TargetPercent: 100},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// --- HTTP-level integration tests ---

func setupChargeSessionTestDB(t *testing.T) (*sql.DB, *ChargeSessionHandler) {
	db := setupHandlerTestDB(t)
	t.Cleanup(func() { db.Close() })

	return db, newTestChargeSessionHandler(t, db)
}

func TestChargeSessionHandler_Start_InvalidJSON(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_Start_MissingVehicleID(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(StartRequest{StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_Start_InvalidStartPercent(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: -1, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_Start_InvalidTargetPercent(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 101})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_Start_TargetNotHigher(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 80, TargetPercent: 50})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_Start_VehicleNotFound(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(StartRequest{VehicleID: "nonexistent", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestChargeSessionHandler_Start_ActiveSessionExists(t *testing.T) {
	db, handler := setupChargeSessionTestDB(t)

	// Create an active session first
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Transition to active
	_, err := db.Exec("UPDATE charge_sessions SET status = 'active' WHERE status = 'pending'")
	require.NoError(t, err)

	// Try to start another
	body2, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req2, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.Start(rr2, req2)

	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestChargeSessionHandler_Start_Success(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var session models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&session)
	require.NoError(t, err)
	assert.Equal(t, "rm1", session.VehicleID)
	assert.Equal(t, models.SessionStatusPending, session.Status)
}

func TestChargeSessionHandler_Stop_NoActiveSession(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions?vehicleId=rm1", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Stop(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Empty(t, rr.Body.String())
}

func TestChargeSessionHandler_Stop_Success(t *testing.T) {
	db, handler := setupChargeSessionTestDB(t)

	// Create and activate a session
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Transition to active
	_, err := db.Exec("UPDATE charge_sessions SET status = 'active' WHERE status = 'pending'")
	require.NoError(t, err)

	// Stop the session
	req2, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", nil)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.Stop(rr2, req2)

	assert.Equal(t, http.StatusNoContent, rr2.Code)
	assert.Empty(t, rr2.Body.String())
}

func TestChargeSessionHandler_GetActive_NoSession(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/charge-sessions?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()

	handler.GetActive(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestChargeSessionHandler_GetActive_SessionExists(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	// Create a session
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Get active session
	req2, _ := http.NewRequest(http.MethodGet, "/api/charge-sessions?vehicleId=rm1", nil)
	rr2 := httptest.NewRecorder()
	handler.GetActive(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)
	var session models.ChargeSession
	err := json.NewDecoder(rr2.Body).Decode(&session)
	require.NoError(t, err)
	assert.Equal(t, "rm1", session.VehicleID)
}

func TestChargeSessionHandler_UpdateTarget_InvalidJSON(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	req, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions/target", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateTarget(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_UpdateTarget_InvalidTargetPercent(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(map[string]float64{"targetPercent": 101})
	req, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions/target", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateTarget(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_UpdateTarget_NoActiveSession(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(map[string]float64{"targetPercent": 90})
	req, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateTarget(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// A session that exists but is not active (e.g. still pending) is a state
// conflict, not a missing resource: it must map to 409, consistently with the
// other state-conflict conditions.
func TestChargeSessionHandler_UpdateTarget_PendingSessionConflict(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	// Create a pending session (Start leaves it pending).
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Updating the target of a pending (not active) session conflicts.
	body2, _ := json.Marshal(map[string]float64{"targetPercent": 90})
	req2, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.UpdateTarget(rr2, req2)

	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestChargeSessionHandler_UpdateTarget_Success(t *testing.T) {
	db, handler := setupChargeSessionTestDB(t)

	// Create and activate a session
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Transition to active
	_, err := db.Exec("UPDATE charge_sessions SET status = 'active' WHERE status = 'pending'")
	require.NoError(t, err)

	// Update target
	body2, _ := json.Marshal(map[string]float64{"targetPercent": 90})
	req2, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.UpdateTarget(rr2, req2)

	assert.Equal(t, http.StatusNoContent, rr2.Code)
}

func TestChargeSessionHandler_Delete_MissingID(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	req, _ := http.NewRequest(http.MethodDelete, "/api/charge-sessions/", nil)
	rr := httptest.NewRecorder()

	handler.Delete(rr, req, "")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChargeSessionHandler_Delete_SessionNotFound(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	req, _ := http.NewRequest(http.MethodDelete, "/api/charge-sessions/nonexistent", nil)
	rr := httptest.NewRecorder()

	handler.Delete(rr, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestChargeSessionHandler_Delete_CannotDeleteActive(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	// Create a session
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Get the session ID
	var session models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&session)
	require.NoError(t, err)

	// Delete should fail for pending session
	req2, _ := http.NewRequest(http.MethodDelete, "/api/charge-sessions/"+session.ID, nil)
	rr2 := httptest.NewRecorder()
	handler.Delete(rr2, req2, session.ID)

	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestChargeSessionHandler_Delete_Success(t *testing.T) {
	db, handler := setupChargeSessionTestDB(t)

	// Create a completed session directly in DB
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "cs_del_test",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		StartKwh:  0.4,
		EndKwh:    floatPtr(1.6),
		StartPct:  20,
		EndPct:    floatPtr(80),
		TargetKwh: 1.6,
		TargetPct: 80,
		Status:    "completed",
	}))

	req, _ := http.NewRequest(http.MethodDelete, "/api/charge-sessions/cs_del_test", nil)
	rr := httptest.NewRecorder()
	handler.Delete(rr, req, "cs_del_test")

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestChargeSessionHandler_UsesInjectedService(t *testing.T) {
	db := setupHandlerTestDB(t)
	t.Cleanup(func() { db.Close() })

	service := services.NewChargeSessionService(
		context.Background(),
		repository.NewChargeSessionRepository(db),
		repository.NewVehicleRepository(db),
		nil,
		nil,
		nil,
		nil,
	)

	handler := NewChargeSessionHandler(service)

	assert.Same(t, service, handler.service, "handler must use the injected service instance, not create a new one")
}

func TestChargeSessionHandler_Stop_ServiceError(t *testing.T) {
	db := setupHandlerTestDB(t)
	handler := newTestChargeSessionHandler(t, db)
	db.Close()

	req, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()

	handler.Stop(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestChargeSessionHandler_GetActive_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	handler := newTestChargeSessionHandler(t, db)
	db.Close()

	req, _ := http.NewRequest(http.MethodGet, "/api/charge-sessions?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()

	handler.GetActive(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func floatPtr(f float64) *float64 { return &f }
