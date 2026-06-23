package services

import (
	"context"
	"testing"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryService_GetHistory_NoSessions(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	results, err := service.GetHistory(context.Background(), HistoryQueryParams{
		Limit: 100,
	})
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestHistoryService_GetHistory_WithSessions(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	// Create a completed session
	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38, // 1.9 * 0.2
		TargetPercent: 80,
		TargetKwh:     1.52, // 1.9 * 0.8
		Status:        models.SessionStatusCompleted,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	endKwh := 1.52
	endPercent := 80.0
	batteryKwh := endKwh - session.StartKwh
	wallKwh := batteryKwh / 0.8
	require.NoError(t, sessRepo.UpdateEndWithStats(context.Background(), session.ID, session.CreatedAt, endKwh, endPercent, batteryKwh, wallKwh, 0, nil, 0, 0))

	results, err := service.GetHistory(context.Background(), HistoryQueryParams{
		Limit: 100,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Battery kWh = endKwh - startKwh = 1.52 - 0.38 = 1.14
	assert.NotNil(t, results[0].TotalBatteryKwh)
	assert.InDelta(t, 1.14, *results[0].TotalBatteryKwh, 0.001)
}

func TestHistoryService_GetHistory_FilterByVehicle(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	// Create sessions for different vehicles
	session1 := &models.ChargeSession{
		VehicleID:    testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent: 20,
		StartKwh:     0.38,
		TargetPercent: 80,
		TargetKwh:    1.52,
		Status:       models.SessionStatusCompleted,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session1))

	session2 := &models.ChargeSession{
		VehicleID:    testVehicleID2,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent: 30,
		StartKwh:     1.638,
		TargetPercent: 90,
		TargetKwh:    4.914,
		Status:       models.SessionStatusCompleted,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session2))

	// Filter by testVehicleID
	results, err := service.GetHistory(context.Background(), HistoryQueryParams{
		VehicleID: testVehicleID,
		Limit:     100,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, testVehicleID, results[0].VehicleID)
}

func TestHistoryService_GetHistory_FilterByDate(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	// Create a session
	session := &models.ChargeSession{
		VehicleID:    testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent: 20,
		StartKwh:     0.38,
		TargetPercent: 80,
		TargetKwh:    1.52,
		Status:       models.SessionStatusCompleted,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	today := session.CreatedAt.Format("2006-01-02")
	results, err := service.GetHistory(context.Background(), HistoryQueryParams{
		Date:  today,
		Limit: 100,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestHistoryService_GetHistory_NoEndKwh(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	// Create a session without end kWh
	session := &models.ChargeSession{
		VehicleID:    testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent: 20,
		StartKwh:     0.38,
		TargetPercent: 80,
		TargetKwh:    1.52,
		Status:       models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))

	results, err := service.GetHistory(context.Background(), HistoryQueryParams{
		Limit: 100,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].TotalBatteryKwh)
}

func TestHistoryService_GetHistory_ActiveSessionWithEnergy(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	session := &models.ChargeSession{
		VehicleID:     testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartPercent:  20,
		StartKwh:      0.38,
		TargetPercent: 80,
		TargetKwh:     1.52,
		Status:        models.SessionStatusActive,
	}
	require.NoError(t, sessRepo.Create(context.Background(), session))
	require.NoError(t, sessRepo.UpdateLastBlendedKwh(context.Background(), session.ID, 0.75))

	results, err := service.GetHistory(context.Background(), HistoryQueryParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NotNil(t, results[0].TotalBatteryKwh)
	assert.InDelta(t, 0.37, *results[0].TotalBatteryKwh, 0.001) // 0.75 - 0.38 = 0.37 (delta, not absolute position)
}

func TestHistoryService_GetHistory_LimitAndOffset(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)

	service := NewHistoryService(sessRepo, sessRepo)

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		session := &models.ChargeSession{
			VehicleID:    testVehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
			StartPercent: 20,
			StartKwh:     0.38,
			TargetPercent: 80,
			TargetKwh:    1.52,
			Status:       models.SessionStatusCompleted,
		}
		require.NoError(t, sessRepo.Create(context.Background(), session))
	}

	// Test limit
	results, err := service.GetHistory(context.Background(), HistoryQueryParams{
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Test offset
	results, err = service.GetHistory(context.Background(), HistoryQueryParams{
		Limit:  2,
		Offset: 2,
	})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
