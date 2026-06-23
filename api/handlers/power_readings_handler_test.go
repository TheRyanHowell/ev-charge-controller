package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	handlerTestUserID  = "test-user"
	handlerTestPlugID  = "test-plug"
	handlerTestUserPtr = &handlerTestUserID
	handlerTestPlugPtr = &handlerTestPlugID
)

func TestPowerReadingsHandler_NoActiveSession(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)
	handler := NewPowerReadingsHandler(services.NewChartDataService(repo))

	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings", nil)
	rr := httptest.NewRecorder()
	handler.GetReadings(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestPowerReadingsHandler_ActiveSessionWithReadings(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Start a session
	reqBody := map[string]any{"vehicleId": "rm1", "startPercent": 20, "targetPercent": 80}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newTestChargeSessionHandler(t, db).Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	var session models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&session)
	require.NoError(t, err)

	// Insert sample power readings
	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID:        "pr_test_1",
		SessionID: session.ID,
		Power:     600,
		EnergyKwh: 0.1,
		Voltage:   230,
		Current:   2.6,
		Timestamp: time.Now(),
	})
	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID:        "pr_test_2",
		SessionID: session.ID,
		Power:     610,
		EnergyKwh: 0.2,
		Voltage:   230,
		Current:   2.65,
		Timestamp: time.Now().Add(5 * time.Second),
	})

	// Hit the power-readings endpoint
	req2, _ := http.NewRequest(http.MethodGet, "/api/power-readings", nil)
	rr2 := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)

	var readings []models.PowerReading
	err = json.Unmarshal(rr2.Body.Bytes(), &readings)
	require.NoError(t, err)
	assert.Len(t, readings, 2)
	assert.Equal(t, 600.0, readings[0].Power)
	assert.Equal(t, 610.0, readings[1].Power)
}

func TestPowerReadingsHandler_ActiveSessionEmptyReadings(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Start a session without any power readings
	reqBody := map[string]any{"vehicleId": "rm1", "startPercent": 20, "targetPercent": 80}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newTestChargeSessionHandler(t, db).Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Hit the power-readings endpoint
	req2, _ := http.NewRequest(http.MethodGet, "/api/power-readings", nil)
	rr2 := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr2, req2)

	assert.Equal(t, http.StatusNoContent, rr2.Code)
}

func TestPowerReadingsHandler_VehicleFallbackToCompleted(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Complete a session for rm1
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    handlerTestUserPtr,
		PlugID:    handlerTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      20.0,
		TargetKwh:     50.0,
		StartPercent:  20.0,
		TargetPercent: 50.0,
		Status:        "completed",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Insert readings for that session
	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID:        "pr_vf_1",
		SessionID: session.ID,
		Power:     700,
		EnergyKwh: 0.3,
		Voltage:   230,
		Current:   3.0,
		Timestamp: time.Now().Add(-90 * time.Minute),
	})

	// Request with matching vehicleId should return the completed session data
	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var readings []models.PowerReading
	_ = json.Unmarshal(rr.Body.Bytes(), &readings)
	require.Len(t, readings, 1)
	assert.Equal(t, 700.0, readings[0].Power)
}

func TestPowerReadingsHandler_VehicleNoMatchingSession(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Complete a session for rm1
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    handlerTestUserPtr,
		PlugID:    handlerTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      20.0,
		TargetKwh:     50.0,
		StartPercent:  20.0,
		TargetPercent: 50.0,
		Status:        "completed",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Request for non-existent vehicle should get 204
	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?vehicleId=unknown-vehicle", nil)
	rr := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestPowerReadingsHandler_VehiclePrefersActiveOverCompleted(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Completed session for rm1
	startTime := time.Now().Add(-3 * time.Hour)
	endTime := time.Now().Add(-2 * time.Hour)
	completed := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    handlerTestUserPtr,
		PlugID:    handlerTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      10.0,
		TargetKwh:     40.0,
		StartPercent:  10.0,
		TargetPercent: 40.0,
		Status:        "completed",
	}
	require.NoError(t, repo.Create(t.Context(), completed))
	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "pr_old", SessionID: completed.ID,
		Power: 100, EnergyKwh: 0.1, Voltage: 230, Current: 0.5,
		Timestamp: time.Now().Add(-2 * time.Hour),
	})

	// Active session for same vehicle
	activeStart := time.Now().Add(-30 * time.Minute)
	active := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    handlerTestUserPtr,
		PlugID:    handlerTestPlugPtr,
		CreatedAt:     activeStart,
		StartKwh:      40.0,
		TargetKwh:     60.0,
		StartPercent:  40.0,
		TargetPercent: 60.0,
		Status:        "active",
	}
	require.NoError(t, repo.Create(t.Context(), active))
	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "pr_new", SessionID: active.ID,
		Power: 3500, EnergyKwh: 1.5, Voltage: 230, Current: 15.2,
		Timestamp: time.Now().Add(-15 * time.Minute),
	})

	// Request should return active session data, not completed
	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var readings []models.PowerReading
	_ = json.Unmarshal(rr.Body.Bytes(), &readings)
	require.Len(t, readings, 1)
	assert.Equal(t, 3500.0, readings[0].Power)
}

func TestPowerReadingsHandler_BySessionID(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Create a completed session with power readings
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    handlerTestUserPtr,
		PlugID:    handlerTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      10.0,
		TargetKwh:     40.0,
		StartPercent:  10.0,
		TargetPercent: 40.0,
		Status:        "completed",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "pr_sid_1", SessionID: session.ID,
		Power: 800, EnergyKwh: 0.5, Voltage: 230, Current: 3.5,
		Timestamp: time.Now().Add(-90 * time.Minute),
	})
	_ = repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "pr_sid_2", SessionID: session.ID,
		Power: 820, EnergyKwh: 0.6, Voltage: 230, Current: 3.55,
		Timestamp: time.Now().Add(-60 * time.Minute),
	})

	// Request with sessionId should return that session's readings
	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?sessionId="+session.ID, nil)
	rr := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var readings []models.PowerReading
	_ = json.Unmarshal(rr.Body.Bytes(), &readings)
	require.Len(t, readings, 2)
	assert.Equal(t, 800.0, readings[0].Power)
	assert.Equal(t, 820.0, readings[1].Power)
}

func TestPowerReadingsHandler_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)
	handler := NewPowerReadingsHandler(services.NewChartDataService(repo))

	// Close DB to trigger error on ResolveChartSession
	require.NoError(t, db.Close())

	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings", nil)
	rr := httptest.NewRecorder()
	handler.GetReadings(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPowerReadingsHandler_GetPowerReadingsError(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Create a session so ResolveChartSession succeeds
	startTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    handlerTestUserPtr,
		PlugID:    handlerTestPlugPtr, CreatedAt: startTime, StartKwh: 20.0,
		TargetKwh: 50.0, StartPercent: 20.0, TargetPercent: 50.0,
		Status: "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Drop the power_readings table so GetPowerReadings fails
	_, dropErr := db.Exec(`DROP TABLE power_readings`)
	require.NoError(t, dropErr)

	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?sessionId="+session.ID, nil)
	rr := httptest.NewRecorder()
	NewPowerReadingsHandler(services.NewChartDataService(repo)).GetReadings(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
