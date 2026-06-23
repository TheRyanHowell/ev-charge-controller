package services

import (
	"testing"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChartDataService_GetPowerReadings_NoSession(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()
	repo := repository.NewChargeSessionRepository(db)
	svc := NewChartDataService(repo)
	readings, err := svc.GetPowerReadings(t.Context(), "", "")
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestChartDataService_GetSOCSnapshots_NoSession(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()
	repo := repository.NewChargeSessionRepository(db)
	svc := NewChartDataService(repo)
	snapshots, err := svc.GetSOCSnapshots(t.Context(), "", "")
	require.NoError(t, err)
	assert.Empty(t, snapshots)
}

func TestChartDataService_GetPowerReadings_WithSession(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()
	repo := repository.NewChargeSessionRepository(db)
	svc := NewChartDataService(repo)

	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		Status:    models.SessionStatusCompleted,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	readings, err := svc.GetPowerReadings(t.Context(), session.ID, "")
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestChartDataService_GetSOCSnapshots_WithSession(t *testing.T) {
	db := setupServiceTestDB(t)
	defer db.Close()
	repo := repository.NewChargeSessionRepository(db)
	svc := NewChartDataService(repo)

	session := &models.ChargeSession{
		VehicleID: testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		Status:    models.SessionStatusCompleted,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	snapshots, err := svc.GetSOCSnapshots(t.Context(), session.ID, "")
	require.NoError(t, err)
	assert.Empty(t, snapshots)
}
