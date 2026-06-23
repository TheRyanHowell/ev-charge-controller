package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTariffUserID = "test-user-tariff"

func setupTariffHandlerTest(t *testing.T) (*TariffHandler, *sql.DB) {
	db := setupHandlerTestDB(t)
	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testTariffUserID, "tariff-handler-test@example.com", "")
	require.NoError(t, err)

	svc := services.NewTariffService(repository.NewTariffRepository(db))
	return NewTariffHandler(svc), db
}

func tariffRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/tariff-settings", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(internal.WithUserID(req.Context(), testTariffUserID))
}

func TestTariffHandler_Get_DefaultWhenUnset(t *testing.T) {
	handler, db := setupTariffHandlerTest(t)
	defer db.Close()

	rr := httptest.NewRecorder()
	handler.Get(rr, tariffRequest(http.MethodGet, ""))

	assert.Equal(t, http.StatusOK, rr.Code)
	var got models.TariffSettings
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.InDelta(t, 24.83, got.BaseRatePence, 1e-9)
	assert.Empty(t, got.OffPeakWindows)
}

func TestTariffHandler_Put_PersistsAndReturns(t *testing.T) {
	handler, db := setupTariffHandlerTest(t)
	defer db.Close()

	body := `{"baseRatePence":28.5,"offPeakWindows":[{"start":"00:30","end":"04:30","ratePence":7}]}`
	rr := httptest.NewRecorder()
	handler.Put(rr, tariffRequest(http.MethodPut, body))

	require.Equal(t, http.StatusOK, rr.Code)
	var got models.TariffSettings
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.InDelta(t, 28.5, got.BaseRatePence, 1e-9)
	require.Len(t, got.OffPeakWindows, 1)
	assert.Equal(t, "00:30", got.OffPeakWindows[0].Start)

	// A follow-up GET returns the persisted tariff.
	rr2 := httptest.NewRecorder()
	handler.Get(rr2, tariffRequest(http.MethodGet, ""))
	require.Equal(t, http.StatusOK, rr2.Code)
	var reloaded models.TariffSettings
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&reloaded))
	assert.InDelta(t, 28.5, reloaded.BaseRatePence, 1e-9)
}

func TestTariffHandler_Put_InvalidReturns400(t *testing.T) {
	handler, db := setupTariffHandlerTest(t)
	defer db.Close()

	body := `{"baseRatePence":-5,"offPeakWindows":[]}`
	rr := httptest.NewRecorder()
	handler.Put(rr, tariffRequest(http.MethodPut, body))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTariffHandler_Put_MalformedJSONReturns400(t *testing.T) {
	handler, db := setupTariffHandlerTest(t)
	defer db.Close()

	rr := httptest.NewRecorder()
	handler.Put(rr, tariffRequest(http.MethodPut, "{not json"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
