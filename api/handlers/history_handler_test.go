package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHistoryTestDB(t *testing.T) *sql.DB {
	return setupHandlerTestDB(t)
}

func TestHistoryHandler_AllSessions(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	endedAt2 := time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC)
	createdAt1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt1,
		EndedAt:    &endedAt,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_2",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt2,
		EndedAt:    &endedAt2,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.0),
		TargetKwh:  1.0,
		StartPct:   20,
		EndPct:     ptrF(50),
		TargetPct:  50,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=100", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestHistoryHandler_FilteredByVehicle(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	endedAt1 := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	endedAt2 := time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC)
	createdAt1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt1,
		EndedAt:    &endedAt1,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_2",
		VehicleID:  "rm1s",
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt2,
		EndedAt:    &endedAt2,
		StartKwh:   0.76,
		EndKwh:     ptrF(2.28),
		TargetKwh:  2.28,
		StartPct:   20,
		EndPct:     ptrF(60),
		TargetPct:  60,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?vehicleId=rm1s&limit=100", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "rm1s", sessions[0].VehicleID)
}

func TestHistoryHandler_EmptyResults(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=100", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHistoryHandler_EmptyFilteredResults(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?vehicleId=nonexistent&limit=100", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHistoryHandler_SessionWithZeroStartKwh(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_zero",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt,
		EndedAt:    &endedAt,
		StartKwh:   0.0,
		EndKwh:     ptrF(0.5),
		TargetKwh:  0.5,
		StartPct:   0,
		EndPct:     ptrF(25),
		TargetPct:  25,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=100", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var responses []struct {
		models.ChargeSession
		TotalBatteryKwh *float64 `json:"totalBatteryKwh,omitempty"`
	}
	err := json.NewDecoder(rr.Body).Decode(&responses)
	require.NoError(t, err)
	require.Len(t, responses, 1)

	// totalBatteryKwh = endKwh - startKwh = 0.5 - 0 = 0.5
	require.NotNil(t, responses[0].TotalBatteryKwh)
	assert.InDelta(t, 0.5, *responses[0].TotalBatteryKwh, 0.001)
}

func TestHistoryHandler_WithLimit(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	createdAt3 := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	endedAt1 := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	endedAt2 := time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC)
	endedAt3 := time.Date(2025, 1, 3, 1, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt1,
		EndedAt:    &endedAt1,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_2",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt2,
		EndedAt:    &endedAt2,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.0),
		TargetKwh:  1.0,
		StartPct:   20,
		EndPct:     ptrF(50),
		TargetPct:  50,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_3",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt3,
		EndedAt:    &endedAt3,
		StartKwh:   0.5,
		EndKwh:     ptrF(1.5),
		TargetKwh:  1.5,
		StartPct:   25,
		EndPct:     ptrF(75),
		TargetPct:  75,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=2", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestHistoryHandler_WithVehicleAndLimit(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	createdAt3 := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	endedAt1 := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	endedAt2 := time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC)
	endedAt3 := time.Date(2025, 1, 3, 1, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt1,
		EndedAt:    &endedAt1,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_2",
		VehicleID:  "rm1s",
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt2,
		EndedAt:    &endedAt2,
		StartKwh:   0.76,
		EndKwh:     ptrF(2.28),
		TargetKwh:  2.28,
		StartPct:   20,
		EndPct:     ptrF(60),
		TargetPct:  60,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_3",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt3,
		EndedAt:    &endedAt3,
		StartKwh:   0.5,
		EndKwh:     ptrF(1.5),
		TargetKwh:  1.5,
		StartPct:   25,
		EndPct:     ptrF(75),
		TargetPct:  75,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?vehicleId=rm1&limit=1", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "rm1", sessions[0].VehicleID)
}

func TestHistoryHandler_WithDate(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt1 := time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC)
	createdAt3 := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	endedAt1 := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	endedAt2 := time.Date(2025, 1, 15, 16, 0, 0, 0, time.UTC)
	endedAt3 := time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt1,
		EndedAt:    &endedAt1,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_2",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt2,
		EndedAt:    &endedAt2,
		StartKwh:   0.5,
		EndKwh:     ptrF(1.2),
		TargetKwh:  1.2,
		StartPct:   25,
		EndPct:     ptrF(60),
		TargetPct:  60,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_3",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt3,
		EndedAt:    &endedAt3,
		StartKwh:   0.3,
		EndKwh:     ptrF(1.5),
		TargetKwh:  1.5,
		StartPct:   15,
		EndPct:     ptrF(75),
		TargetPct:  75,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?date=2025-01-15", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestHistoryHandler_WithVehicleAndDate(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt1 := time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	endedAt1 := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	endedAt2 := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_rm1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt1,
		EndedAt:    &endedAt1,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_rm2",
		VehicleID:  "rm2",
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt2,
		EndedAt:    &endedAt2,
		StartKwh:   1.0,
		EndKwh:     ptrF(3.0),
		TargetKwh:  3.0,
		StartPct:   20,
		EndPct:     ptrF(55),
		TargetPct:  55,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?vehicleId=rm1&date=2025-01-15", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "rm1", sessions[0].VehicleID)
}

func TestHistoryHandler_NoParams_Returns400(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_DateNoResults(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?date=2024-01-01", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHistoryHandler_LimitNoResults(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=10", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHistoryHandler_NegativeLimit_Returns400(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=-5", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_NegativeOffset_Returns400(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=10&offset=-1", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_NonNumericLimit_Returns400(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=abc", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_NonNumericOffset_Returns400(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=10&offset=xyz", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_ZeroLimit_Returns400(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=0", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_DefaultLimitWhenOmitted(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt,
		EndedAt:    &endedAt,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?date=2025-01-01", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var sessions []models.ChargeSession
	err := json.NewDecoder(rr.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestHistoryHandler_OffsetWithoutLimit(t *testing.T) {
	db := setupHistoryTestDB(t)
	defer db.Close()

	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)

	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs_1",
		VehicleID:  testdb.DefaultVehicleID,
		UserID:     testdb.DefaultUserID,
		PlugID:     testdb.DefaultPlugID,
		Status:     "completed",
		CreatedAt:  createdAt,
		EndedAt:    &endedAt,
		StartKwh:   0.4,
		EndKwh:     ptrF(1.6),
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     ptrF(80),
		TargetPct:  80,
	}))

	handler := NewHistoryHandler(services.NewHistoryService(repository.NewChargeSessionRepository(db), repository.NewChargeSessionRepository(db)))
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=100&offset=10", nil)
	rr := httptest.NewRecorder()

	handler.Get(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestParsePaginationParams(t *testing.T) {
	tests := []struct {
		name         string
		limitStr     string
		offsetStr    string
		wantLimit    int
		wantOffset   int
		wantErrIs    error
		wantErrAny   bool
	}{
		{
			name:       "both empty uses defaults",
			limitStr:   "",
			offsetStr:  "",
			wantLimit:  100,
			wantOffset: 0,
		},
		{
			name:       "valid limit and offset",
			limitStr:   "50",
			offsetStr:  "10",
			wantLimit:  50,
			wantOffset: 10,
		},
		{
			name:       "valid limit zero offset",
			limitStr:   "25",
			offsetStr:  "0",
			wantLimit:  25,
			wantOffset: 0,
		},
		{
			name:       "limit only offset defaults",
			limitStr:   "30",
			offsetStr:  "",
			wantLimit:  30,
			wantOffset: 0,
		},
		{
			name:      "negative limit",
			limitStr:  "-1",
			wantErrIs: ErrInvalidLimit,
		},
		{
			name:      "zero limit",
			limitStr:  "0",
			wantErrIs: ErrInvalidLimit,
		},
		{
			name:      "negative offset",
			limitStr:  "10",
			offsetStr: "-5",
			wantErrIs: ErrInvalidOffset,
		},
		{
			name:       "non-numeric limit",
			limitStr:   "abc",
			wantErrAny: true,
		},
		{
			name:       "non-numeric offset",
			limitStr:   "10",
			offsetStr:  "xyz",
			wantErrAny: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset, err := parsePaginationParams(tt.limitStr, tt.offsetStr)
			if tt.wantErrIs != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErrIs)
			} else if tt.wantErrAny {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantLimit, limit)
				assert.Equal(t, tt.wantOffset, offset)
			}
		})
	}
}

func ptrF(f float64) *float64 {
	return &f
}
