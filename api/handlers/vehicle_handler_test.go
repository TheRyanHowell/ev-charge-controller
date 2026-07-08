package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVehicleHandlerDB(t *testing.T) *sql.DB {
	return setupHandlerTestDB(t)
}

func newVehicleHandler(db *sql.DB) *VehicleHandler {
	return NewVehicleHandler(services.NewVehicleService(
		repository.NewVehicleRepository(db),
		repository.NewVehicleModelRepository(db),
		repository.NewChargeSessionRepository(db),
		&sync.Mutex{},
	))
}

// insertHandlerVehicle creates a vehicle instance directly in the DB and returns its ID.
func insertHandlerVehicle(t *testing.T, db *sql.DB, id, userID, modelID string) {
	t.Helper()
	require.NoError(t, testdb.InsertVehicle(db, id, userID, modelID, modelID, 20, 80))
}

// reqWithUser adds a user ID to the request context.
func reqWithUser(r *http.Request, userID string) *http.Request {
	return r.WithContext(internal.WithUserID(r.Context(), userID))
}

func TestVehicleHandler_ListModels(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicle-models", nil)
	rr := httptest.NewRecorder()
	handler.ListModels(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var mods []models.VehicleModel
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&mods))
	assert.Len(t, mods, 3)
	byID := make(map[string]string)
	for _, m := range mods {
		byID[m.ID] = m.Name
	}
	assert.Equal(t, "Maeving RM1", byID["rm1"])
}

func TestVehicleHandler_List(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	userID := "u1"
	insertHandlerVehicle(t, db, "v1", userID, "rm1")
	insertHandlerVehicle(t, db, "v2", userID, "rm1s")

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles", nil)
	req = reqWithUser(req, userID)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var vehicles []models.Vehicle
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&vehicles))
	assert.Len(t, vehicles, 2)
}

func TestVehicleHandler_List_Empty(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles", nil)
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Should return empty array (nil slice marshals as null, not []-verify body)
}

func TestVehicleHandler_Create(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"modelId":"rm1","name":"My RM1"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var v models.Vehicle
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&v))
	assert.Equal(t, "My RM1", v.Name)
	assert.Equal(t, "rm1", v.ModelID)
	assert.InDelta(t, 2.026, v.CapacityKwh, 0.001)
}

func TestVehicleHandler_Create_DefaultName(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"modelId":"rm2"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var v models.Vehicle
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&v))
	assert.Equal(t, "Maeving RM2", v.Name)
}

func TestVehicleHandler_Create_ModelNotFound(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"modelId":"unknown"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestVehicleHandler_Create_MissingModelId(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"name":"oops"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVehicleHandler_Create_Unauthorized(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"modelId":"rm1"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVehicleHandler_Delete(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	userID := "u1"
	insertHandlerVehicle(t, db, "v1", userID, "rm1")

	req, _ := http.NewRequest(http.MethodDelete, "/api/vehicles/v1", nil)
	req = reqWithUser(req, userID)
	req.SetPathValue("id", "v1")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestVehicleHandler_Delete_NotFound(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	req, _ := http.NewRequest(http.MethodDelete, "/api/vehicles/nonexistent", nil)
	req = reqWithUser(req, "u1")
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestVehicleHandler_GetByID(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles/v1", nil)
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req, "v1")

	assert.Equal(t, http.StatusOK, rr.Code)
	var v models.Vehicle
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&v))
	assert.Equal(t, "rm1", v.ModelID)
	assert.InDelta(t, 2.026, v.CapacityKwh, 0.001)
}

func TestVehicleHandler_GetByID_NotFound(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles/nonexistent", nil)
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestVehicleHandler_Patch_UpdateBothPercents(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")

	body := strings.NewReader(`{"currentPercent": 50.0, "targetPercent": 75.0}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusNoContent, rr.Code)

	var cur, tgt float64
	require.NoError(t, db.QueryRow(`SELECT current_percent, target_percent FROM vehicles WHERE id = 'v1'`).Scan(&cur, &tgt))
	assert.Equal(t, 50.0, cur)
	assert.Equal(t, 75.0, tgt)
}

func TestVehicleHandler_Patch_NoFields(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVehicleHandler_Patch_InvalidPercents(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")

	body := strings.NewReader(`{"currentPercent": 80.0, "targetPercent": 50.0}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVehicleHandler_Patch_UpdateNotificationPrefs(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")

	body := strings.NewReader(`{"notifyChargeStarted": false, "notifyChargeComplete": false}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusNoContent, rr.Code)

	var ncs, ncc, nco, nmo int
	require.NoError(t, db.QueryRow(`SELECT notify_charge_started, notify_charge_complete, notify_charger_offline, notify_maintenance_offline FROM vehicles WHERE id = 'v1'`).Scan(&ncs, &ncc, &nco, &nmo))
	assert.Equal(t, 0, ncs)
	assert.Equal(t, 0, ncc)
	assert.Equal(t, 1, nco)
	assert.Equal(t, 1, nmo)
}

func TestVehicleHandler_List_DBError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestVehicleHandler_Patch_UpdateCurrentRejectedDuringSession(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:         "cs1",
		VehicleID:  "v1",
		UserID:     "u1",
		PlugID:     "test-plug",
		StartKwh:   0.4,
		StartPct:   20,
		TargetKwh:  1.6,
		TargetPct:  80,
		Status:     "active",
	}))

	body := strings.NewReader(`{"currentPercent": 50.0}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestVehicleHandler_Patch_UpdateName(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")

	body := strings.NewReader(`{"name":"New Name"}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusNoContent, rr.Code)

	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM vehicles WHERE id = 'v1'`).Scan(&name))
	assert.Equal(t, "New Name", name)
}

func TestVehicleHandler_Patch_UpdateName_Duplicate(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")
	insertHandlerVehicle(t, db, "v2", "u1", "rm2")

	body := strings.NewReader(`{"name":"rm2"}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestVehicleHandler_Patch_UpdateName_NotFound(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"name":"New Name"}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestVehicleHandler_Patch_UpdateNameAndPercents(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	insertHandlerVehicle(t, db, "v1", "u1", "rm1")

	body := strings.NewReader(`{"name":"New Name", "currentPercent": 50.0}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusNoContent, rr.Code)

	var name string
	var cur float64
	require.NoError(t, db.QueryRow(`SELECT name, current_percent FROM vehicles WHERE id = 'v1'`).Scan(&name, &cur))
	assert.Equal(t, "New Name", name)
	assert.Equal(t, 50.0, cur)
}

func TestVehicleHandler_ListModels_DBError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicle-models", nil)
	rr := httptest.NewRecorder()
	handler.ListModels(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestVehicleHandler_Create_DuplicateName(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"modelId":"rm1"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	body2 := strings.NewReader(`{"modelId":"rm1"}`)
	req2, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body2)
	req2.Header.Set("Content-Type", "application/json")
	req2 = reqWithUser(req2, "u1")
	rr2 := httptest.NewRecorder()
	handler.Create(rr2, req2)

	assert.Equal(t, http.StatusConflict, rr2.Code)
}

func TestVehicleHandler_Create_InternalError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	body := strings.NewReader(`{"modelId":"rm1","name":"My RM1"}`)
	req, _ := http.NewRequest(http.MethodPost, "/api/vehicles", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestVehicleHandler_Delete_InvalidID(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	req, _ := http.NewRequest(http.MethodDelete, "/api/vehicles/", nil)
	req = reqWithUser(req, "u1")
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVehicleHandler_Delete_Unauthorized(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	req, _ := http.NewRequest(http.MethodDelete, "/api/vehicles/v1", nil)
	req.SetPathValue("id", "v1")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVehicleHandler_Delete_InternalError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	req, _ := http.NewRequest(http.MethodDelete, "/api/vehicles/v1", nil)
	req = reqWithUser(req, "u1")
	req.SetPathValue("id", "v1")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestVehicleHandler_GetByID_InternalError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles/v1", nil)
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req, "v1")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestVehicleHandler_Patch_InvalidJSON(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`not json`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVehicleHandler_Patch_UpdatePercents_NotFound(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	defer db.Close()
	handler := newVehicleHandler(db)

	body := strings.NewReader(`{"currentPercent": 50.0}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "nonexistent")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestVehicleHandler_Patch_UpdatePercents_InternalError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	body := strings.NewReader(`{"currentPercent": 50.0}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestVehicleHandler_Patch_UpdateName_InternalError(t *testing.T) {
	db := setupVehicleHandlerDB(t)
	handler := newVehicleHandler(db)
	db.Close()

	body := strings.NewReader(`{"name":"New Name"}`)
	req, _ := http.NewRequest(http.MethodPatch, "/api/vehicles/v1", body)
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUser(req, "u1")
	rr := httptest.NewRecorder()
	handler.Patch(rr, req, "v1")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
