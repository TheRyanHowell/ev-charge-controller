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
	socTestUserID  = "test-user"
	socTestPlugID  = "test-plug"
	socTestUserPtr = &socTestUserID
	socTestPlugPtr = &socTestPlugID
)

func TestSOCSnapshotsHandler_NoActiveSession(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)
	handler := NewSOCSnapshotsHandler(services.NewChartDataService(repo))

	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots", nil)
	rr := httptest.NewRecorder()
	handler.GetSnapshots(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestSOCSnapshotsHandler_ActiveSessionWithSnapshots(t *testing.T) {
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

	// Insert sample SOC snapshots
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID:         "soc_test_1",
		SessionID:  session.ID,
		SocPercent: 25.0,
		Timestamp:  time.Now(),
	})
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID:         "soc_test_2",
		SessionID:  session.ID,
		SocPercent: 30.0,
		Timestamp:  time.Now().Add(5 * time.Second),
	})

	// Hit the soc-snapshots endpoint
	req2, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots", nil)
	rr2 := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)

	var snapshots []models.SOCSnapshot
	err = json.Unmarshal(rr2.Body.Bytes(), &snapshots)
	require.NoError(t, err)
	assert.Len(t, snapshots, 2)
	assert.Equal(t, 25.0, snapshots[0].SocPercent)
	assert.Equal(t, 30.0, snapshots[1].SocPercent)
}

func TestSOCSnapshotsHandler_ActiveSessionEmptySnapshots(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Start a session without any SOC snapshots
	reqBody := map[string]any{"vehicleId": "rm1", "startPercent": 20, "targetPercent": 80}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newTestChargeSessionHandler(t, db).Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Hit the soc-snapshots endpoint
	req2, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots", nil)
	rr2 := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr2, req2)

	assert.Equal(t, http.StatusNoContent, rr2.Code)
}

func TestSOCSnapshotsHandler_VehicleFallbackToCompleted(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Complete a session for rm1
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    socTestUserPtr,
		PlugID:    socTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      20.0,
		TargetKwh:     50.0,
		StartPercent:  20.0,
		TargetPercent: 50.0,
		Status:        "completed",
	}
	require.NoError(t, 	repo.Create(t.Context(), session))

	// Insert SOC snapshots
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "soc_vf_1", SessionID: session.ID,
		SocPercent: 25.0, Timestamp: time.Now().Add(-90 * time.Minute),
	})
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "soc_vf_2", SessionID: session.ID,
		SocPercent: 50.0, Timestamp: time.Now().Add(-60 * time.Minute),
	})

	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var snaps []models.SOCSnapshot
	_ = json.Unmarshal(rr.Body.Bytes(), &snaps)
	require.Len(t, snaps, 2)
	assert.Equal(t, 25.0, snaps[0].SocPercent)
	assert.Equal(t, 50.0, snaps[1].SocPercent)
}

func TestSOCSnapshotsHandler_VehicleNoMatchingSession(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Completed session for rm1
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    socTestUserPtr,
		PlugID:    socTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      20.0,
		TargetKwh:     50.0,
		StartPercent:  20.0,
		TargetPercent: 50.0,
		Status:        "completed",
	}
	require.NoError(t, 	repo.Create(t.Context(), session))

	// Request for non-existent vehicle should get 204
	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots?vehicleId=unknown-vehicle", nil)
	rr := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestSOCSnapshotsHandler_VehiclePrefersActiveOverCompleted(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Completed session for rm1
	startTime := time.Now().Add(-3 * time.Hour)
	endTime := time.Now().Add(-2 * time.Hour)
	completed := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    socTestUserPtr,
		PlugID:    socTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      10.0,
		TargetKwh:     40.0,
		StartPercent:  10.0,
		TargetPercent: 40.0,
		Status:        "completed",
	}
	require.NoError(t, 	repo.Create(t.Context(), completed))
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "soc_old", SessionID: completed.ID,
		SocPercent: 40.0, Timestamp: time.Now().Add(-2 * time.Hour),
	})

	// Active session for same vehicle
	activeStart := time.Now().Add(-30 * time.Minute)
	active := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    socTestUserPtr,
		PlugID:    socTestPlugPtr,
		CreatedAt:     activeStart,
		StartKwh:      40.0,
		TargetKwh:     60.0,
		StartPercent:  40.0,
		TargetPercent: 60.0,
		Status:        "active",
	}
	require.NoError(t, 	repo.Create(t.Context(), active))
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "soc_new", SessionID: active.ID,
		SocPercent: 45.0, Timestamp: time.Now().Add(-15 * time.Minute),
	})

	// Request should return active session data, not completed
	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots?vehicleId=rm1", nil)
	rr := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var snaps []models.SOCSnapshot
	_ = json.Unmarshal(rr.Body.Bytes(), &snaps)
	require.Len(t, snaps, 1)
	assert.Equal(t, 45.0, snaps[0].SocPercent)
}

func TestSOCSnapshotsHandler_BySessionID(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Create a completed session with SOC snapshots
	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID:     "rm1",
		UserID:    socTestUserPtr,
		PlugID:    socTestPlugPtr,
		CreatedAt:     startTime,
		EndedAt:       &endTime,
		StartKwh:      10.0,
		TargetKwh:     40.0,
		StartPercent:  10.0,
		TargetPercent: 40.0,
		Status:        "completed",
	}
	require.NoError(t, 	repo.Create(t.Context(), session))

	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "soc_sid_1", SessionID: session.ID,
		SocPercent: 15.0, Timestamp: time.Now().Add(-90 * time.Minute),
	})
	_ = 	repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "soc_sid_2", SessionID: session.ID,
		SocPercent: 25.0, Timestamp: time.Now().Add(-60 * time.Minute),
	})

	// Request with sessionId should return that session's snapshots
	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots?sessionId="+session.ID, nil)
	rr := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var snaps []models.SOCSnapshot
	_ = json.Unmarshal(rr.Body.Bytes(), &snaps)
	require.Len(t, snaps, 2)
	assert.Equal(t, 15.0, snaps[0].SocPercent)
	assert.Equal(t, 25.0, snaps[1].SocPercent)
}

func TestSOCSnapshotsHandler_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)
	handler := NewSOCSnapshotsHandler(services.NewChartDataService(repo))

	// Close DB to trigger error on ResolveChartSession
	require.NoError(t, db.Close())

	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots", nil)
	rr := httptest.NewRecorder()
	handler.GetSnapshots(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestSOCSnapshotsHandler_GetSOCSnapshotsError(t *testing.T) {
	db := setupHandlerTestDB(t)
	repo := repository.NewChargeSessionRepository(db)

	// Create a session so ResolveChartSession succeeds
	startTime := time.Now().Add(-1 * time.Hour)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    socTestUserPtr,
		PlugID:    socTestPlugPtr, CreatedAt: startTime, StartKwh: 20.0,
		TargetKwh: 50.0, StartPercent: 20.0, TargetPercent: 50.0,
		Status: "active",
	}
	require.NoError(t, 	repo.Create(t.Context(), session))

	// Drop the soc_snapshots table so GetSOCSnapshots fails
	_, dropErr := db.Exec(`DROP TABLE soc_snapshots`)
	require.NoError(t, dropErr)

	req, _ := http.NewRequest(http.MethodGet, "/api/soc-snapshots?sessionId="+session.ID, nil)
	rr := httptest.NewRecorder()
	NewSOCSnapshotsHandler(services.NewChartDataService(repo)).GetSnapshots(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
