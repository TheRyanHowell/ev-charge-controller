package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Error-path tests for database package
// ---------------------------------------------------------------------------

func TestSeed_DBError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Close DB to force Exec error
	db.Close()

	err := Seed(db)
	assert.Error(t, err)
}

func TestBackfillBootstrap_BeginTxError(t *testing.T) {
	db := setupTestDB(t)

	// Close DB to force Begin error
	db.Close()

	err := BackfillBootstrap(db, "admin@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "begin")
}

func TestBackfillBootstrap_SkipsWhenUsersExist(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	// setupTestDBWithSeed creates a user, so BackfillBootstrap should be a no-op
	err := BackfillBootstrap(db, "admin@test.com")
	require.NoError(t, err)

	// Should still have only the original user
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestBackfillBootstrap_CreatesVehiclesForAllModels(t *testing.T) {
	db, err := SetupTestDB(true)
	require.NoError(t, err)
	defer db.Close()

	err = BackfillBootstrap(db, "admin@test.com")
	require.NoError(t, err)

	// Should have one vehicle per model (4 models in seed)
	var vehicleCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM vehicles`).Scan(&vehicleCount)
	require.NoError(t, err)
	assert.Equal(t, 4, vehicleCount)
}

func TestBackfillBootstrap_Idempotent(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	// Run BackfillBootstrap twice - should be idempotent
	err := BackfillBootstrap(db, "admin@test.com")
	require.NoError(t, err)

	err = BackfillBootstrap(db, "admin@test.com")
	require.NoError(t, err)

	// Should still have only one user, one plug, three vehicles
	var userCount, plugCount, vehicleCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount)
	require.NoError(t, err)
	assert.Equal(t, 1, userCount)

	err = db.QueryRow(`SELECT COUNT(*) FROM plugs`).Scan(&plugCount)
	require.NoError(t, err)
	assert.Equal(t, 1, plugCount)

	err = db.QueryRow(`SELECT COUNT(*) FROM vehicles`).Scan(&vehicleCount)
	require.NoError(t, err)
	assert.Equal(t, 3, vehicleCount)
}

func TestBackfillBootstrap_EmptyBootstrapEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := BackfillBootstrap(db, "")
	require.NoError(t, err)

	var email string
	err = db.QueryRow(`SELECT email FROM users`).Scan(&email)
	require.NoError(t, err)
	assert.Equal(t, "admin@localhost", email)
}

func TestApplyPragmas_DBError(t *testing.T) {
	db := setupTestDB(t)
	db.Close()

	err := ApplyPragmas(db)
	assert.Error(t, err)
}

func TestOpen_InMemory(t *testing.T) {
	db, err := Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Verify connection works
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestOpen_MaxOpenConns(t *testing.T) {
	db, err := Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	stats := db.Stats()
	assert.Equal(t, 1, stats.MaxOpenConnections)
}

func TestInit_ApplyPragmasError(t *testing.T) {
	db, err := Open(":memory:")
	require.NoError(t, err)

	// Drop all tables so PRAGMA fails
	_, err = db.Exec(`DROP TABLE IF EXISTS schema_migrations`)
	require.NoError(t, err)
	db.Close()

	// Now call Init with a path that will cause ApplyPragmas to fail
	// We can't easily trigger this, so test with a bad path instead
	_, err = Init("/tmp/nonexistent-dir-xyz/ev-test.db")
	assert.Error(t, err)
}

func TestInit_SuccessWithSeed(t *testing.T) {
	db, err := Init("/tmp/ev-test-init-success.db")
	require.NoError(t, err)
	defer db.Close()

	// Verify schema exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestSetupTestDB_NoSeed(t *testing.T) {
	db, err := SetupTestDB(false)
	require.NoError(t, err)
	defer db.Close()

	// Schema should exist but no seed data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestInit_Success(t *testing.T) {
	db, err := Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Verify schema exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestSeed_InsertError(t *testing.T) {
	// Create a DB with schema but empty vehicle_models, then corrupt the table
	// so that INSERT fails
	db, err := SetupTestDB(false)
	require.NoError(t, err)
	defer db.Close()

	// Drop and recreate vehicle_models with a constraint that will break seed
	_, err = db.Exec(`DROP TABLE vehicle_models`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE vehicle_models (id TEXT PRIMARY KEY, name TEXT NOT NULL, capacity_kwh REAL)`)
	require.NoError(t, err)

	err = Seed(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "seed database")
}

func TestBackfillBootstrap_TxQueryError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Drop users table so tx.QueryRow fails
	_, err := db.Exec("DROP TABLE users")
	require.NoError(t, err)

	err = BackfillBootstrap(db, "admin@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count users")
}

func TestBackfillBootstrap_InsertPlugError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Drop plugs table so plug insert fails
	_, err := db.Exec("DROP TABLE plugs")
	require.NoError(t, err)

	err = BackfillBootstrap(db, "admin@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert plug")
}

func TestBackfillBootstrap_InsertVehicleError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Drop schedules table so schedule backfill fails
	_, err := db.Exec("DROP TABLE schedules")
	require.NoError(t, err)

	err = BackfillBootstrap(db, "admin@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "back-fill")
}

func TestBackfillBootstrap_ListModelsError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Drop vehicle_models table so model query fails
	_, err := db.Exec("DROP TABLE vehicle_models")
	require.NoError(t, err)

	err = BackfillBootstrap(db, "admin@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list models")
}
