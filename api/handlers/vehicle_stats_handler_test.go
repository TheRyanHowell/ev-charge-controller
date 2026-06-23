package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockVehicleStatsHandlerRepo struct {
	vehicle          *models.Vehicle
	vehicles         []models.Vehicle
	aggregates       *models.SessionAggregates
	dailyEnergy      []models.DailyEnergy
	minMaxData       map[string][2]float64
	findByIDErr      error
	listErr          error
	aggregatesErr    error
	dailyEnergyErr   error
	minMaxErr        error
}

func (m *mockVehicleStatsHandlerRepo) FindByID(_ context.Context, _ string) (*models.Vehicle, error) {
	return m.vehicle, m.findByIDErr
}

func (m *mockVehicleStatsHandlerRepo) List(_ context.Context) ([]models.Vehicle, error) {
	return m.vehicles, m.listErr
}

func (m *mockVehicleStatsHandlerRepo) GetSessionAggregates(_ context.Context, _ string, _ time.Time) (*models.SessionAggregates, error) {
	return m.aggregates, m.aggregatesErr
}

func (m *mockVehicleStatsHandlerRepo) GetDailyEnergy(_ context.Context, _ string, _ time.Time) ([]models.DailyEnergy, error) {
	return m.dailyEnergy, m.dailyEnergyErr
}

func (m *mockVehicleStatsHandlerRepo) GetAllVehiclesMinMaxBatteryKwh(_ context.Context) (map[string][2]float64, error) {
	if m.minMaxErr != nil {
		return nil, m.minMaxErr
	}
	if m.minMaxData != nil {
		return m.minMaxData, nil
	}
	return map[string][2]float64{}, nil
}

func TestVehicleStatsHandler_GetStats_Success(t *testing.T) {
	repo := &mockVehicleStatsHandlerRepo{
		vehicle: &models.Vehicle{
			ID:              "v1",
			TotalSessions:   3,
			TotalBatteryKwh: 9.0,
			TotalWallKwh:    11.25,
			TotalCo2Grams:   2812.5,
		},
		dailyEnergy: []models.DailyEnergy{
			{Date: "2025-06-10", BatteryKwh: 3.0, SessionCount: 1},
		},
	}
	svc := services.NewVehicleStatsService(repo)
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/v1/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetStats(rec, req, "v1")

	require.Equal(t, http.StatusOK, rec.Code)
	var stats services.VehicleStats
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
	assert.Equal(t, 3, stats.TotalSessions)
	assert.InDelta(t, 9.0, stats.TotalBatteryKwh, 0.01)
}

func TestVehicleStatsHandler_GetStats_MissingVehicleID(t *testing.T) {
	svc := services.NewVehicleStatsService(&mockVehicleStatsHandlerRepo{})
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetStats(rec, req, "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestVehicleStatsHandler_GetStats_ServiceError(t *testing.T) {
	repo := &mockVehicleStatsHandlerRepo{
		findByIDErr: assert.AnError,
	}
	svc := services.NewVehicleStatsService(repo)
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/v1/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetStats(rec, req, "v1")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestVehicleStatsHandler_GetStats_CustomRange(t *testing.T) {
	repo := &mockVehicleStatsHandlerRepo{
		aggregates: &models.SessionAggregates{
			TotalSessions:   1,
			TotalBatteryKwh: 5.0,
		},
		dailyEnergy: []models.DailyEnergy{
			{Date: "2025-06-10", BatteryKwh: 5.0, SessionCount: 1},
		},
	}
	svc := services.NewVehicleStatsService(repo)
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/v1/stats?range=week", nil)
	rec := httptest.NewRecorder()

	handler.GetStats(rec, req, "v1")

	require.Equal(t, http.StatusOK, rec.Code)
	var stats services.VehicleStats
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
	assert.Equal(t, 1, stats.TotalSessions)
}

func TestVehicleStatsHandler_GetAllStats_Success(t *testing.T) {
	repo := &mockVehicleStatsHandlerRepo{
		vehicles: []models.Vehicle{
			{ID: "v1", TotalSessions: 2, TotalBatteryKwh: 6.0, TotalWallKwh: 7.5, TotalCo2Grams: 1875},
			{ID: "v2", TotalSessions: 1, TotalBatteryKwh: 4.0, TotalWallKwh: 5.0, TotalCo2Grams: 1250},
		},
	}
	svc := services.NewVehicleStatsService(repo)
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetAllStats(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var stats []services.VehicleSummaryStats
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
	assert.Len(t, stats, 2)
}

func TestVehicleStatsHandler_GetAllStats_ServiceError(t *testing.T) {
	repo := &mockVehicleStatsHandlerRepo{
		listErr: assert.AnError,
	}
	svc := services.NewVehicleStatsService(repo)
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetAllStats(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestVehicleStatsHandler_GetAllStats_Empty(t *testing.T) {
	repo := &mockVehicleStatsHandlerRepo{
		vehicles: []models.Vehicle{},
	}
	svc := services.NewVehicleStatsService(repo)
	handler := NewVehicleStatsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/vehicles/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetAllStats(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var stats []services.VehicleSummaryStats
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
	assert.Empty(t, stats)
}
