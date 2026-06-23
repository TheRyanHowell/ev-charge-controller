package services

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVehicleStatsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	seedTestUser(t, db)
	seedTestVehicle(t, db)
	_, err = db.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic, created_at) VALUES (?, ?, 'Test', 'test', 'test', CURRENT_TIMESTAMP)`,
		testPlugID, testUserID)
	require.NoError(t, err)
	return db
}

func seedCompletedSession(t *testing.T, db *sql.DB, vehicleID string, batteryKwh, wallKwh, co2Grams float64, createdAt time.Time) {
	t.Helper()
	repo := repository.NewChargeSessionRepository(db)
	s := &models.ChargeSession{
		VehicleID: vehicleID,
		UserID:    testUserIDPtr,
		PlugID:    testPlugIDPtr,
		StartKwh:  0.38,
		Status:    "completed",
		CreatedAt: createdAt,
	}
	require.NoError(t, repo.Create(t.Context(), s))
	endKwh := s.StartKwh + batteryKwh
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), s.ID, s.CreatedAt, endKwh, 80, batteryKwh, wallKwh, co2Grams, nil, 0, 0))
}

func updateVehicleAggregates(t *testing.T, db *sql.DB, vehicleID string, totalSessions int, totalBatteryKwh, totalWallKwh, totalCo2Grams, minSessionKwh, maxSessionKwh float64, lastSessionAt *time.Time) {
	t.Helper()
	lastSessionStr := ""
	if lastSessionAt != nil {
		lastSessionStr = lastSessionAt.Format(time.RFC3339)
	}
	_, err := db.Exec(
		`UPDATE vehicles SET total_sessions = ?, total_battery_kwh = ?, total_wall_kwh = ?, total_co2_grams = ?,
		 min_session_battery_kwh = ?, max_session_battery_kwh = ?, last_session_at = ?
		 WHERE id = ?`,
		totalSessions, totalBatteryKwh, totalWallKwh, totalCo2Grams, minSessionKwh, maxSessionKwh, lastSessionStr, vehicleID,
	)
	require.NoError(t, err)
}

func TestVehicleStatsService_GetStats_Lifetime(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	now := time.Now()
	seedCompletedSession(t, db, testVehicleID, 1.5, 1.875, 468.75, now.Add(-48*time.Hour))
	seedCompletedSession(t, db, testVehicleID, 2.0, 2.5, 625, now.Add(-24*time.Hour))
	seedCompletedSession(t, db, testVehicleID, 1.0, 1.25, 312.5, now)

	lastSession := now
	updateVehicleAggregates(t, db, testVehicleID, 3, 4.5, 5.625, 1406.25, 1.0, 2.0, &lastSession)

	ctx := context.Background()
	stats, err := service.GetStats(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeLifetime,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 3, stats.TotalSessions)
	assert.InDelta(t, 4.5, stats.TotalBatteryKwh, 0.01)
	assert.InDelta(t, 1.5, stats.AvgSessionKwh, 0.01)
	assert.InDelta(t, 1406.25, stats.TotalCo2Grams, 0.01)
	assert.NotNil(t, stats.AvgCarbonGCo2Kwh)
	assert.InDelta(t, 250.0, *stats.AvgCarbonGCo2Kwh, 0.01)
	assert.InDelta(t, 1.0, stats.MinSessionBatteryKwh, 0.01)
	assert.InDelta(t, 2.0, stats.MaxSessionBatteryKwh, 0.01)
	assert.Len(t, stats.DailyEnergy, 3)
}

func TestVehicleStatsService_GetStats_Week(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	now := time.Now()
	// Session 1 day ago (in 7-day range)
	seedCompletedSession(t, db, testVehicleID, 1.5, 1.875, 468.75, now.Add(-24*time.Hour))
	// Session 10 days ago (out of 7-day range)
	seedCompletedSession(t, db, testVehicleID, 2.0, 2.5, 625, now.AddDate(0, 0, -10))

	ctx := context.Background()
	stats, err := service.GetStats(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeWeek,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats.TotalSessions)
	assert.InDelta(t, 1.5, stats.TotalBatteryKwh, 0.01)
	assert.InDelta(t, 1.5, stats.AvgSessionKwh, 0.01)
	assert.InDelta(t, 468.75, stats.TotalCo2Grams, 0.01)
	assert.InDelta(t, 1.5, stats.MinSessionBatteryKwh, 0.01)
	assert.InDelta(t, 1.5, stats.MaxSessionBatteryKwh, 0.01)
	assert.Len(t, stats.DailyEnergy, 1)
}

func TestVehicleStatsService_GetStats_NoData(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	ctx := context.Background()
	stats, err := service.GetStats(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeWeek,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, float64(0), stats.TotalBatteryKwh)
	assert.Equal(t, float64(0), stats.AvgSessionKwh)
	assert.Nil(t, stats.AvgCarbonGCo2Kwh)
	assert.Equal(t, float64(0), stats.TotalCo2Grams)
	assert.Empty(t, stats.DailyEnergy)
}

func TestVehicleStatsService_GetStats_VehicleNotFound(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	ctx := context.Background()
	stats, err := service.GetStats(ctx, VehicleStatsQueryParams{
		VehicleID: "nonexistent-vehicle",
		Range:     TimeRangeLifetime,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalSessions)
	assert.Empty(t, stats.DailyEnergy)
}

func TestVehicleStatsService_GetDailyEnergy_Lifetime(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	now := time.Now()
	// Day 1: 2 sessions
	seedCompletedSession(t, db, testVehicleID, 1.0, 1.25, 250, now.Add(-48*time.Hour))
	seedCompletedSession(t, db, testVehicleID, 0.5, 0.625, 125, now.Add(-48*time.Hour))
	// Day 2: 1 session
	seedCompletedSession(t, db, testVehicleID, 2.0, 2.5, 625, now.Add(-24*time.Hour))

	ctx := context.Background()
	daily, err := service.GetDailyEnergy(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeLifetime,
	})
	require.NoError(t, err)
	require.Len(t, daily, 2)

	assert.InDelta(t, 1.5, daily[0].BatteryKwh, 0.01)
	assert.Equal(t, 2, daily[0].SessionCount)
	assert.InDelta(t, 375, daily[0].Co2Grams, 0.01)
	assert.NotNil(t, daily[0].AvgCarbonIntensityGCo2PerKwh)
	assert.InDelta(t, 200.0, *daily[0].AvgCarbonIntensityGCo2PerKwh, 0.01)

	assert.InDelta(t, 2.0, daily[1].BatteryKwh, 0.01)
	assert.Equal(t, 1, daily[1].SessionCount)
	assert.InDelta(t, 625, daily[1].Co2Grams, 0.01)
}

func TestVehicleStatsService_GetDailyEnergy_Week(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	now := time.Now()
	// In range
	seedCompletedSession(t, db, testVehicleID, 1.5, 1.875, 468.75, now.Add(-24*time.Hour))
	// Out of 7-day range
	seedCompletedSession(t, db, testVehicleID, 2.0, 2.5, 625, now.AddDate(0, 0, -10))

	ctx := context.Background()
	daily, err := service.GetDailyEnergy(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeWeek,
	})
	require.NoError(t, err)
	require.Len(t, daily, 1)
	assert.InDelta(t, 1.5, daily[0].BatteryKwh, 0.01)
}

func TestVehicleStatsService_GetAllVehiclesStats(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	now := time.Now()
	// Vehicle 1: 2 sessions
	seedCompletedSession(t, db, testVehicleID, 1.5, 1.875, 468.75, now.Add(-24*time.Hour))
	seedCompletedSession(t, db, testVehicleID, 1.0, 1.25, 312.5, now)
	last1 := now
	updateVehicleAggregates(t, db, testVehicleID, 2, 2.5, 3.125, 781.25, 1.0, 1.5, &last1)

	// Vehicle 2: 1 session
	seedCompletedSession(t, db, testVehicleID2, 2.0, 2.5, 625, now.Add(-12*time.Hour))
	last2 := now.Add(-12 * time.Hour)
	updateVehicleAggregates(t, db, testVehicleID2, 1, 2.0, 2.5, 625, 2.0, 2.0, &last2)

	ctx := context.Background()
	summaries, err := service.GetAllVehiclesStats(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 3)

	v1 := summaries[0]
	v2 := summaries[1]

	if v1.VehicleID == testVehicleID {
		assert.Equal(t, 2, v1.TotalSessions)
		assert.InDelta(t, 2.5, v1.TotalBatteryKwh, 0.01)
		assert.InDelta(t, 1.5625, v1.AvgSessionWallKwh, 0.01)
		assert.InDelta(t, 781.25, v1.TotalCo2Grams, 0.01)
		assert.NotNil(t, v1.LastSessionAt)
		assert.InDelta(t, 1.0, v1.MinSessionBatteryKwh, 0.01)
		assert.InDelta(t, 1.5, v1.MaxSessionBatteryKwh, 0.01)
	} else {
		assert.Equal(t, testVehicleID2, v1.VehicleID)
		assert.Equal(t, 1, v1.TotalSessions)
		assert.InDelta(t, 2.0, v1.TotalBatteryKwh, 0.01)
		assert.InDelta(t, 2.5, v1.AvgSessionWallKwh, 0.01)
		assert.InDelta(t, 625, v1.TotalCo2Grams, 0.01)
		assert.NotNil(t, v1.LastSessionAt)
		assert.InDelta(t, 2.0, v1.MinSessionBatteryKwh, 0.01)
		assert.InDelta(t, 2.0, v1.MaxSessionBatteryKwh, 0.01)

		assert.Equal(t, testVehicleID, v2.VehicleID)
		assert.Equal(t, 2, v2.TotalSessions)
	}
}

func TestVehicleStatsService_GetAllVehiclesStats_Empty(t *testing.T) {
	db := setupVehicleStatsDB(t)
	// Remove seeded vehicles
	_, err := db.Exec(`DELETE FROM vehicles`)
	require.NoError(t, err)

	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	ctx := context.Background()
	summaries, err := service.GetAllVehiclesStats(ctx)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestVehicleStatsService_getCutoff(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		rangeVal TimeRange
		expected time.Time
	}{
		{"week", TimeRangeWeek, now.AddDate(0, 0, -7)},
		{"month", TimeRangeMonth, now.AddDate(0, -1, 0)},
		{"year", TimeRangeYear, now.AddDate(-1, 0, 0)},
		{"lifetime", TimeRangeLifetime, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCutoff(now, tt.rangeVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVehicleStatsService_GetStats_Lifetime_NoSessions(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	ctx := context.Background()
	stats, err := service.GetStats(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeLifetime,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, float64(0), stats.TotalBatteryKwh)
	assert.Empty(t, stats.DailyEnergy)
}

func TestVehicleStatsService_GetStats_UserFiltered(t *testing.T) {
	db := setupVehicleStatsDB(t)
	service := NewVehicleStatsServiceWithRepos(
		repository.NewVehicleRepository(db),
		repository.NewChargeSessionRepository(db),
	)

	now := time.Now()
	seedCompletedSession(t, db, testVehicleID, 1.5, 1.875, 468.75, now.Add(-24*time.Hour))

	ctx := internal.WithUserID(t.Context(), testUserID)
	stats, err := service.GetStats(ctx, VehicleStatsQueryParams{
		VehicleID: testVehicleID,
		Range:     TimeRangeWeek,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats.TotalSessions)
	assert.InDelta(t, 1.5, stats.TotalBatteryKwh, 0.01)
}

func TestVehicleStatsService_mapVehicleToStats(t *testing.T) {
	now := time.Now()
	vehicle := &models.Vehicle{
		ID:              "v1",
		TotalSessions:   3,
		TotalBatteryKwh: 4.5,
		TotalWallKwh:    5.625,
		TotalCo2Grams:   1406.25,
		LastSessionAt:   &now,
	}

	stats := mapVehicleToStats(vehicle)

	assert.Equal(t, 3, stats.TotalSessions)
	assert.Equal(t, 4.5, stats.TotalBatteryKwh)
	assert.InDelta(t, 1.5, stats.AvgSessionKwh, 0.01)
	assert.Equal(t, 1406.25, stats.TotalCo2Grams)
	assert.NotNil(t, stats.AvgCarbonGCo2Kwh)
	assert.InDelta(t, 250.0, *stats.AvgCarbonGCo2Kwh, 0.01)
}

func TestVehicleStatsService_mapVehicleToStats_ZeroSessions(t *testing.T) {
	vehicle := &models.Vehicle{
		ID:              "v1",
		TotalSessions:   0,
		TotalBatteryKwh: 0,
		TotalWallKwh:    0,
		TotalCo2Grams:   0,
	}

	stats := mapVehicleToStats(vehicle)

	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, float64(0), stats.AvgSessionKwh)
	assert.Nil(t, stats.AvgCarbonGCo2Kwh)
}

func TestVehicleStatsService_mapVehicleToSummary(t *testing.T) {
	now := time.Now()
	vehicle := &models.Vehicle{
		ID:                   "v1",
		TotalSessions:        2,
		TotalBatteryKwh:      3.0,
		TotalWallKwh:         3.75,
		TotalCo2Grams:        937.5,
		MinSessionBatteryKwh: 1.0,
		MaxSessionBatteryKwh: 2.0,
		LastSessionAt:        &now,
	}

	summary := mapVehicleToSummary(vehicle)

	assert.Equal(t, "v1", summary.VehicleID)
	assert.Equal(t, 2, summary.TotalSessions)
	assert.Equal(t, 3.0, summary.TotalBatteryKwh)
	assert.InDelta(t, 1.875, summary.AvgSessionWallKwh, 0.01)
	assert.Equal(t, 937.5, summary.TotalCo2Grams)
	assert.InDelta(t, 1.0, summary.MinSessionBatteryKwh, 0.01)
	assert.InDelta(t, 2.0, summary.MaxSessionBatteryKwh, 0.01)
	assert.NotNil(t, summary.LastSessionAt)
}

func TestVehicleStatsService_emptyStats(t *testing.T) {
	stats := emptyStats()

	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, float64(0), stats.TotalBatteryKwh)
	assert.Equal(t, float64(0), stats.AvgSessionKwh)
	assert.Nil(t, stats.AvgCarbonGCo2Kwh)
	assert.Equal(t, float64(0), stats.TotalCo2Grams)
	assert.Empty(t, stats.DailyEnergy)
}

// --- Mock implementations for error-path tests ---

type mockVehicleReader struct {
	vehicles []models.Vehicle
	vehicle  *models.Vehicle
	listErr  error
	findErr  error
}

func (m *mockVehicleReader) FindByID(_ context.Context, _ string) (*models.Vehicle, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.vehicle, nil
}

func (m *mockVehicleReader) List(_ context.Context) ([]models.Vehicle, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.vehicles, nil
}

type mockSessionAggregator struct {
	aggregates  *models.SessionAggregates
	dailyEnergy []models.DailyEnergy
	aggErr      error
	dailyErr    error
}

func (m *mockSessionAggregator) GetSessionAggregates(_ context.Context, _ string, _ time.Time) (*models.SessionAggregates, error) {
	if m.aggErr != nil {
		return nil, m.aggErr
	}
	return m.aggregates, nil
}

func (m *mockSessionAggregator) GetDailyEnergy(_ context.Context, _ string, _ time.Time) ([]models.DailyEnergy, error) {
	if m.dailyErr != nil {
		return nil, m.dailyErr
	}
	return m.dailyEnergy, nil
}

func TestVehicleStatsService_getLifetimeStats_FindByIDError(t *testing.T) {
	expectedErr := fmt.Errorf("database connection lost")
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{findErr: expectedErr},
		&mockSessionAggregator{},
	)

	stats, err := service.GetStats(context.Background(), VehicleStatsQueryParams{
		VehicleID: "v1",
		Range:     TimeRangeLifetime,
	})
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, stats)
}

func TestVehicleStatsService_getLifetimeStats_GetDailyEnergyError(t *testing.T) {
	expectedErr := fmt.Errorf("daily energy query failed")
	now := time.Now()
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{
			vehicle: &models.Vehicle{
				ID:              "v1",
				TotalSessions:   2,
				TotalBatteryKwh: 3.0,
				TotalWallKwh:    3.75,
				TotalCo2Grams:   937.5,
				LastSessionAt:   &now,
			},
		},
		&mockSessionAggregator{dailyErr: expectedErr},
	)

	stats, err := service.GetStats(context.Background(), VehicleStatsQueryParams{
		VehicleID: "v1",
		Range:     TimeRangeLifetime,
	})
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, stats)
}

func TestVehicleStatsService_getRangeStats_GetSessionAggregatesError(t *testing.T) {
	expectedErr := fmt.Errorf("aggregates query failed")
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{},
		&mockSessionAggregator{aggErr: expectedErr},
	)

	stats, err := service.GetStats(context.Background(), VehicleStatsQueryParams{
		VehicleID: "v1",
		Range:     TimeRangeWeek,
	})
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, stats)
}

func TestVehicleStatsService_getRangeStats_GetDailyEnergyError(t *testing.T) {
	expectedErr := fmt.Errorf("daily energy query failed")
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{},
		&mockSessionAggregator{
			aggregates: &models.SessionAggregates{
				TotalSessions:   3,
				TotalBatteryKwh: 4.5,
				TotalCo2Grams:   1125,
			},
			dailyErr: expectedErr,
		},
	)

	stats, err := service.GetStats(context.Background(), VehicleStatsQueryParams{
		VehicleID: "v1",
		Range:     TimeRangeMonth,
	})
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, stats)
}

func TestVehicleStatsService_getRangeStats_NoAggregates(t *testing.T) {
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{},
		&mockSessionAggregator{aggregates: nil},
	)

	stats, err := service.GetStats(context.Background(), VehicleStatsQueryParams{
		VehicleID: "v1",
		Range:     TimeRangeWeek,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalSessions)
	assert.Empty(t, stats.DailyEnergy)
}

func TestVehicleStatsService_getRangeStats_ZeroSessions(t *testing.T) {
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{},
		&mockSessionAggregator{
			aggregates: &models.SessionAggregates{
				TotalSessions:   0,
				TotalBatteryKwh: 0,
			},
		},
	)

	stats, err := service.GetStats(context.Background(), VehicleStatsQueryParams{
		VehicleID: "v1",
		Range:     TimeRangeYear,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalSessions)
	assert.Empty(t, stats.DailyEnergy)
}

func TestVehicleStatsService_GetAllVehiclesStats_ListError(t *testing.T) {
	expectedErr := fmt.Errorf("list vehicles failed")
	service := NewVehicleStatsServiceWithRepos(
		&mockVehicleReader{listErr: expectedErr},
		&mockSessionAggregator{},
	)

	summaries, err := service.GetAllVehiclesStats(context.Background())
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, summaries)
}

func TestVehicleStatsService_NewVehicleStatsService(t *testing.T) {
	// Verify the public constructor works with a combined mock repo
	repo := &statsRepoAdapter{
		vehicleRepo:     &mockVehicleReader{},
		sessionAggregator: &mockSessionAggregator{},
	}
	service := NewVehicleStatsService(repo)
	assert.NotNil(t, service)
}
