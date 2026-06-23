package services

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/testdb"
	"ev-charge-controller/api/tasmota"

	"github.com/stretchr/testify/require"
)

const (
	testPlugID     = testdb.DefaultPlugID
	testUserID     = testdb.DefaultUserID
	testVehicleID  = testdb.DefaultVehicleID
	testVehicleID2 = "rm2"
)

var (
	testUserIDStr  = testUserID
	testPlugIDStr  = testPlugID
	testUserIDPtr  = &testUserIDStr
	testPlugIDPtr  = &testPlugIDStr
)

// mockPlugController implements internal.PlugController for tests.
type mockPlugController struct {
	mu          sync.RWMutex
	energy      map[string]*tasmota.EnergyData
	powerOn     map[string]bool
	setPowerErr error
}

func newMockPlugCtrl() *mockPlugController {
	return &mockPlugController{
		energy:  make(map[string]*tasmota.EnergyData),
		powerOn: make(map[string]bool),
	}
}

func (m *mockPlugController) SetPower(_ context.Context, plugID string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.powerOn[plugID] = on
	return m.setPowerErr
}

func (m *mockPlugController) SetPowerAndWait(_ context.Context, plugID string, on bool, _ time.Duration) (bool, error) {
	if err := m.SetPower(context.Background(), plugID, on); err != nil {
		return false, err
	}
	return true, nil
}

func (m *mockPlugController) LastEnergy(plugID string) *tasmota.EnergyData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.energy[plugID]
}

func (m *mockPlugController) SetEnergy(plugID string, e *tasmota.EnergyData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.energy[plugID] = e
}

func setupServiceTestDB(t *testing.T) *sql.DB {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	testdb.SeedFullTestDB(t, db)
	return db
}

// insertRawVehicle inserts a vehicle_model + vehicle instance pair for tests that need
// direct DB setup with specific capacity or range values.
func insertRawVehicle(t *testing.T, db *sql.DB, id string, capacityKwh, rangeMinMi, rangeMaxMi float64) {
	t.Helper()
	require.NoError(t, testdb.InsertVehicleWithModel(db, id, testUserID, id, capacityKwh, 600, 0.8, rangeMinMi, rangeMaxMi))
}

// seedTestUser inserts a test user into the users table (idempotent).
func seedTestUser(t *testing.T, db *sql.DB) {
	t.Helper()
	testdb.SeedDefaultUser(t, db)
}

// seedTestVehicle creates vehicle instances with known IDs for use in service tests.
func seedTestVehicle(t *testing.T, db *sql.DB) {
	t.Helper()
	testdb.SeedDefaultVehicles(t, db)
}

// insertActiveSession inserts an active charge session for testing.
func insertActiveSession(t *testing.T, db *sql.DB, id, vehicleID string, startKwh, targetKwh, startPct, targetPct float64, startTotalKwh *float64, startedAt *time.Time) {
	t.Helper()
	if startedAt == nil {
		now := time.Now()
		startedAt = &now
	}
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:            id,
		VehicleID:     vehicleID,
		UserID:        testUserID,
		PlugID:        testPlugID,
		Status:        "active",
		StartKwh:      startKwh,
		TargetKwh:     targetKwh,
		StartPct:      startPct,
		TargetPct:     targetPct,
		StartTotalKwh: startTotalKwh,
		StartedAt:     startedAt,
	}))
}

// insertSession inserts a charge session with the given status.
func insertSession(t *testing.T, db *sql.DB, id, vehicleID, status string, startKwh, targetKwh, startPct, targetPct float64, startTotalKwh *float64) {
	t.Helper()
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:            id,
		VehicleID:     vehicleID,
		UserID:        testUserID,
		PlugID:        testPlugID,
		Status:        status,
		StartKwh:      startKwh,
		TargetKwh:     targetKwh,
		StartPct:      startPct,
		TargetPct:     targetPct,
		StartTotalKwh: startTotalKwh,
	}))
}


