package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSchedulePlugID = "test-plug-schedule"
	testScheduleUserID = "test-user-schedule"
)

func setupScheduleHandlerTest(t *testing.T) (*ScheduleHandler, *sql.DB) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testScheduleUserID, "schedule-handler-test@example.com", "")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic) VALUES (?, ?, ?, ?, ?)`,
		testSchedulePlugID, testScheduleUserID, "Schedule Test Plug", "ns-scheduletest", "schedule-topic")
	require.NoError(t, err)

	chargeService := services.NewChargeSessionService(
		context.Background(),
		repository.NewChargeSessionRepository(db),
		repository.NewVehicleRepository(db),
		nil,
		nil,
		nil,
		nil,
	)
	t.Cleanup(func() { chargeService.Shutdown() })

	scheduleService := services.NewScheduleService(
		repository.NewScheduleRepository(db),
		repository.NewPlugRepository(db),
		repository.NewVehicleRepository(db),
		chargeService,
	)
	return NewScheduleHandler(scheduleService), db
}

func TestScheduleHandler_UpsertByPlug(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"time": "03:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	err := json.NewDecoder(rr.Body).Decode(&schedule)
	assert.NoError(t, err)
	assert.Equal(t, "03:00", schedule.Time)
	assert.True(t, schedule.Enabled)
}

func TestScheduleHandler_UpsertByPlug_InvalidID(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"time": "03:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs//schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", "")
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_UpsertByPlug_InvalidBody(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_UpsertByPlug_InvalidTime(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"time": "25:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_UpsertByPlug_ReadyBy(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"time": "03:00", "readyBy": "07:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	err := json.NewDecoder(rr.Body).Decode(&schedule)
	assert.NoError(t, err)
	assert.Equal(t, "03:00", schedule.Time)
	require.NotNil(t, schedule.ReadyBy)
	assert.Equal(t, "07:00", *schedule.ReadyBy)
}

func TestScheduleHandler_UpsertByPlug_ReadyByEqualsTime(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"time": "03:00", "readyBy": "03:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_UpsertByPlug_ReadyByInvalidFormat(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"time": "03:00", "readyBy": "25:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_GetByPlug(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	// Create a schedule first
	reqBody := `{"time": "03:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Now retrieve it
	req = httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	rr = httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	err := json.NewDecoder(rr.Body).Decode(&schedule)
	assert.NoError(t, err)
	assert.Equal(t, "03:00", schedule.Time)
}

func TestScheduleHandler_GetByPlug_InvalidID(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/plugs//schedule", nil)
	req = withPathValue(req, "id", "")
	rr := httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_GetByPlug_NotFound(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	rr := httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestScheduleHandler_GetByPlug_DBError(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	rr := httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestScheduleHandler_Service(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	service := handler.Service()
	require.NotNil(t, service)
}

func TestScheduleHandler_UpsertByPlug_CarbonAware(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"type":"carbon_aware","windowStart":"22:00","windowEnd":"06:00","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	err := json.NewDecoder(rr.Body).Decode(&schedule)
	assert.NoError(t, err)
	assert.Equal(t, models.ScheduleTypeCarbonAware, schedule.Type)
	require.NotNil(t, schedule.WindowStart)
	require.NotNil(t, schedule.WindowEnd)
	assert.Equal(t, "22:00", *schedule.WindowStart)
	assert.Equal(t, "06:00", *schedule.WindowEnd)
	assert.True(t, schedule.Enabled)
}

func TestScheduleHandler_UpsertByPlug_CarbonAware_TwoStage(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"type":"carbon_aware","windowStart":"22:00","windowEnd":"06:00","twoStage":true,"enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	assert.True(t, schedule.TwoStage)
}

func TestScheduleHandler_GetByPlug_CarbonAware_TwoStage_AttachesEstimatedPlan(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()
	assignVehicleToSchedulePlug(t, db)

	reqBody := `{"type":"carbon_aware","windowStart":"00:00","windowEnd":"23:59","twoStage":true,"enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	handler.Service().SetCarbonAwareDeps(
		&mockCarbonForecaster{buckets: flatForecastBuckets(time.Now())},
		func(_ *models.Vehicle, _, _ float64) (int, error) { return 30, nil },
		nil,
	)

	req = httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr = httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	require.NotNil(t, schedule.EstimatedPlan, "expected estimatedPlan to be populated for a two-stage schedule")
	timeRe := `^([01]\d|2[0-3]):[0-5]\d$`
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage1Start)
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage1End)
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage2Start)
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage2End)
	assert.Nil(t, schedule.EstimatedStartTime, "two-stage schedules use EstimatedPlan, not EstimatedStartTime")
}

func TestScheduleHandler_GetByPlug_Daily_TwoStage_AttachesEstimatedPlan(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()
	assignVehicleToSchedulePlug(t, db) // rm1 spec, current=20, target=80

	reqBody := `{"type":"daily","time":"01:00","readyBy":"07:00","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr = httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	require.NotNil(t, schedule.EstimatedPlan, "expected estimatedPlan to be populated for a daily two-stage schedule")
	timeRe := `^([01]\d|2[0-3]):[0-5]\d$`
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage1Start)
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage1End)
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage2Start)
	assert.Regexp(t, timeRe, schedule.EstimatedPlan.Stage2End)
	assert.Nil(t, schedule.EstimatedStartTime, "daily schedules never use EstimatedStartTime")
}

func TestScheduleHandler_GetByPlug_Daily_SingleStage_NoEstimatedPlan(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()
	assignVehicleToSchedulePlug(t, db)

	reqBody := `{"type":"daily","time":"01:00","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr = httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	assert.Nil(t, schedule.EstimatedPlan, "single-stage daily schedules have no two-stage plan to estimate")
}

func TestScheduleHandler_UpsertByPlug_CarbonAware_MissingWindow(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"type":"carbon_aware","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_UpsertByPlug_CarbonAware_EqualWindows(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"type":"carbon_aware","windowStart":"09:00","windowEnd":"09:00","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_UpsertByPlug_InvalidType(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()

	reqBody := `{"type":"solar_powered","time":"09:00","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()

	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// withPathValue sets a URL path value on the request (Go 1.22+ pattern matching).
func withPathValue(r *http.Request, key, value string) *http.Request {
	r.SetPathValue(key, value)
	return r
}

// mockCarbonForecaster implements internal.CarbonForecaster for handler-level tests.
type mockCarbonForecaster struct {
	buckets []carbonintensity.ForecastBucket
}

func (m *mockCarbonForecaster) GetForecast(context.Context, time.Time, time.Time) ([]carbonintensity.ForecastBucket, error) {
	return m.buckets, nil
}

// assignVehicleToSchedulePlug seeds a vehicle below its target and assigns it to the
// schedule test plug, so carbon-aware estimation has a candidate to work with.
func assignVehicleToSchedulePlug(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"sched-veh", testScheduleUserID, "rm1", "Schedule Vehicle", 20.0, 80.0, time.Now())
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE plugs SET vehicle_id = ? WHERE id = ?`, "sched-veh", testSchedulePlugID)
	require.NoError(t, err)
}

// flatForecastBuckets returns 48 hours of equal-intensity 30-min buckets starting a day
// before now, so any window around "now" is covered regardless of test execution time.
func flatForecastBuckets(now time.Time) []carbonintensity.ForecastBucket {
	from := now.Add(-24 * time.Hour).Truncate(30 * time.Minute)
	buckets := make([]carbonintensity.ForecastBucket, 0, 96)
	for i := 0; i < 96; i++ {
		buckets = append(buckets, carbonintensity.ForecastBucket{
			From:         from.Add(time.Duration(i) * 30 * time.Minute),
			To:           from.Add(time.Duration(i+1) * 30 * time.Minute),
			ForecastGCo2: 100,
		})
	}
	return buckets
}

func TestScheduleHandler_GetByPlug_CarbonAware_AttachesEstimatedStart(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()
	assignVehicleToSchedulePlug(t, db)

	reqBody := `{"type":"carbon_aware","windowStart":"00:00","windowEnd":"23:59","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	handler.Service().SetCarbonAwareDeps(
		&mockCarbonForecaster{buckets: flatForecastBuckets(time.Now())},
		func(_ *models.Vehicle, _, _ float64) (int, error) { return 30, nil },
		nil,
	)

	req = httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr = httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	require.NotNil(t, schedule.EstimatedStartTime, "expected estimatedStartTime to be populated")
	assert.Regexp(t, `^([01]\d|2[0-3]):[0-5]\d$`, *schedule.EstimatedStartTime)
}

func TestScheduleHandler_UpsertByPlug_CarbonAware_AttachesEstimatedStart(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()
	assignVehicleToSchedulePlug(t, db)

	handler.Service().SetCarbonAwareDeps(
		&mockCarbonForecaster{buckets: flatForecastBuckets(time.Now())},
		func(_ *models.Vehicle, _, _ float64) (int, error) { return 30, nil },
		nil,
	)

	reqBody := `{"type":"carbon_aware","windowStart":"00:00","windowEnd":"23:59","enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	require.NotNil(t, schedule.EstimatedStartTime, "expected estimatedStartTime in PATCH response")
	assert.Regexp(t, `^([01]\d|2[0-3]):[0-5]\d$`, *schedule.EstimatedStartTime)
}

func TestScheduleHandler_GetByPlug_Daily_NoEstimatedStart(t *testing.T) {
	handler, db := setupScheduleHandlerTest(t)
	defer db.Close()
	assignVehicleToSchedulePlug(t, db)

	handler.Service().SetCarbonAwareDeps(
		&mockCarbonForecaster{buckets: flatForecastBuckets(time.Now())},
		func(_ *models.Vehicle, _, _ float64) (int, error) { return 30, nil },
		nil,
	)

	reqBody := `{"time": "03:00", "enabled": true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/"+testSchedulePlugID+"/schedule", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req = withPathValue(req, "id", testSchedulePlugID)
	req = req.WithContext(internal.WithUserID(req.Context(), testScheduleUserID))
	rr := httptest.NewRecorder()
	handler.UpsertByPlug(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/plugs/"+testSchedulePlugID+"/schedule", nil)
	req = withPathValue(req, "id", testSchedulePlugID)
	rr = httptest.NewRecorder()
	handler.GetByPlug(rr, req)

	var schedule models.Schedule
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&schedule))
	assert.Nil(t, schedule.EstimatedStartTime, "daily schedules should never get an estimated start time")
}
