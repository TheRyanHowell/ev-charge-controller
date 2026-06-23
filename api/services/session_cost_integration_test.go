package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/tasmota"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionCompletion_FreezesTimeWeightedCost verifies that stopping a session
// computes and persists a time-weighted cost from its power readings + the user's
// tariff, and that the cost is rolled into the vehicle's lifetime stats.
func TestSessionCompletion_FreezesTimeWeightedCost(t *testing.T) {
	db := setupServiceTestDB(t)
	sessRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	tariffRepo := repository.NewTariffRepository(db)
	ctrl := newMockPlugCtrl()
	notifier := NewChargeNotifier(context.Background(), nil, vehicleRepo, nil)
	lock := newSessionLock()

	// Off-peak 00:30–04:30 @ 7p; base 24p. Readings below land inside the window (UTC).
	require.NoError(t, tariffRepo.Upsert(context.Background(), testUserID, &models.TariffSettings{
		BaseRatePence:  24,
		OffPeakWindows: []models.OffPeakWindow{{Start: "00:30", End: "04:30", RatePence: 7}},
	}))
	tariffSvc := NewTariffService(tariffRepo)

	service := NewSessionLifecycleService(sessRepo, sessRepo, vehicleRepo, nil, ctrl, sessRepo, notifier, lock)
	service.SetTariffProvider(tariffSvc)

	// Active session whose wall meter started at 1000 kWh.
	const sessionID = "cost-session"
	startTotal := 1000.0
	insertActiveSession(t, db, sessionID, testVehicleID, 10, 40, 20, 80, &startTotal, nil)

	// Two off-peak readings: +1 kWh each over the start baseline (cumulative wall total).
	insertReadingAt(t, db, sessionID, time.Date(2026, 6, 21, 2, 0, 0, 0, time.UTC), 1001)
	insertReadingAt(t, db, sessionID, time.Date(2026, 6, 21, 3, 0, 0, 0, time.UTC), 1002)

	// Final meter total at stop matches the last reading (2 kWh of wall energy).
	ctrl.SetEnergy(testPlugID, &tasmota.EnergyData{Total: 1002, Power: 600})

	_, err := service.StopWithPercent(context.Background(), sessionID, 80)
	require.NoError(t, err)

	completed, err := sessRepo.FindByID(context.Background(), sessionID)
	require.NoError(t, err)
	require.NotNil(t, completed.CostPence)
	require.NotNil(t, completed.OffPeakKwh)
	// 2 kWh wall, all off-peak @ 7p = 14p.
	assert.InDelta(t, 14.0, *completed.CostPence, 1e-6)
	assert.InDelta(t, 2.0, *completed.OffPeakKwh, 1e-6)

	// Lifetime cost rolled into the vehicle's precomputed stats.
	v, err := vehicleRepo.FindByID(context.Background(), testVehicleID)
	require.NoError(t, err)
	assert.InDelta(t, 14.0, v.TotalCostPence, 1e-6)
}

func insertReadingAt(t *testing.T, db *sql.DB, sessionID string, ts time.Time, cumulativeKwh float64) {
	t.Helper()
	require.NoError(t, testdb.InsertPowerReading(db, &testdb.PowerReadingOpts{
		SessionID: sessionID,
		Timestamp: ts,
		Power:     600,
		EnergyKwh: cumulativeKwh,
	}))
}
