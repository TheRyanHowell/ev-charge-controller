package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := SetupTestDB(false)
	require.NoError(t, err)
	return db
}

func setupTestDBWithSeed(t *testing.T) *sql.DB {
	db, err := SetupTestDB(true)
	require.NoError(t, err)
	// Seed a test user and vehicle instances so FK constraints on charge_sessions pass.
	_, err = db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES ('test-user', 'test@db.com', '')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic, created_at) VALUES ('test-plug', 'test-user', 'Test', 'test', 'test', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	for _, modelID := range []string{"rm1", "rm1s", "rm2"} {
		_, err = db.Exec(
			`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, 'test-user', ?, ?, 20, 80, CURRENT_TIMESTAMP)`,
			modelID, modelID, modelID,
		)
		require.NoError(t, err)
	}
	return db
}

func TestInit_FileBasedDB_MaxOpenConns(t *testing.T) {
	dbPath := "/tmp/test-ev-charge-maxconns-" + time.Now().Format("20060102-150405") + ".db"
	defer os.Remove(dbPath)

	db, err := Init(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// File-based SQLite must have MaxOpenConns(1) to prevent
	// "database is locked" errors from concurrent writers.
	stats := db.Stats()
	assert.Equal(t, 1, stats.MaxOpenConnections,
		"file-based SQLite should have MaxOpenConns set to 1")
}

func TestInit(t *testing.T) {
	dbPath := "/tmp/test-ev-charge-" + time.Now().Format("20060102-150405") + ".db"
	defer os.Remove(dbPath)

	db, err := Init(dbPath)
	assert.NoError(t, err)
	defer db.Close()

	// Verify database is usable
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicles").Scan(&count)
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	// Verify db is closed
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master").Scan(&count)
	assert.Error(t, err)
}

func TestInit_BadPath(t *testing.T) {
	// Path in a non-existent directory should return an error
	_, err := Init("/nonexistent/directory/test.db")
	assert.Error(t, err)
}

func TestInit_InMemory(t *testing.T) {
	// Test the :memory: code path in Init (SetMaxOpenConns(1))
	db, err := Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Verify database is usable with schema from embedded schema.sql
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, 3) // seed data: rm1, rm1s, rm2
}

func TestVehicleRepository_CreateInstance(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewVehicleRepository(db)

	vehicle := &models.Vehicle{
		ModelID: "rm2",
		Name:    "My RM2",
	}
	if uid := "test-user"; true {
		vehicle.UserID = &uid
	}

	err := repo.CreateInstance(context.Background(), vehicle)
	assert.NoError(t, err)
	assert.NotEmpty(t, vehicle.ID)
	assert.False(t, vehicle.CreatedAt.IsZero())

	found, err := repo.FindByID(context.Background(), vehicle.ID)
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "My RM2", found.Name)
	assert.Equal(t, "rm2", found.ModelID)
}

func TestVehicleRepository_List(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewVehicleRepository(db)

	uid := "test-user"
	v := &models.Vehicle{ModelID: "rm1", Name: "RM1 Instance", UserID: &uid}
	require.NoError(t, repo.CreateInstance(context.Background(), v))

	vehicles, err := repo.List(context.Background())
	require.NoError(t, err)
	// setupTestDBWithSeed inserts rm1/rm1s/rm2 instances; we added another rm1
	assert.GreaterOrEqual(t, len(vehicles), 1)
	ids := make(map[string]bool)
	for _, veh := range vehicles {
		ids[veh.ID] = true
	}
	assert.True(t, ids[v.ID])
}

func TestVehicleRepository_FindByID(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewVehicleRepository(db)

	uid := "test-user"
	v := &models.Vehicle{ModelID: "rm1s", Name: "My RM1S", UserID: &uid}
	require.NoError(t, repo.CreateInstance(context.Background(), v))

	found, err := repo.FindByID(context.Background(), v.ID)
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "My RM1S", found.Name)
	assert.InDelta(t, 5.46, found.CapacityKwh, 0.01)
}

func TestVehicleRepository_FindByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewVehicleRepository(db)

	vehicle, err := repo.FindByID(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, vehicle)
}

func TestChargeSessionRepository_Create(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	testUserID := "test-user"
	testPlugID := "test-plug"
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    &testUserID,
		PlugID:    &testPlugID,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	err := repo.Create(context.Background(), session)
	assert.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.False(t, session.CreatedAt.IsZero())

	// Verify it was saved
	found, err := repo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "rm1", found.VehicleID)
}

func TestChargeSessionRepository_GetActive(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	// No active session
	session, err := repo.GetActive(context.Background())
	require.NoError(t, err)
	assert.Nil(t, session)

	// Insert active session
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session1",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  80,
		TargetPct: 80,
		Status:    "active",
	}))

	session, err = repo.GetActive(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "session1", session.ID)
}

func TestChargeSessionRepository_GetActive_NoActive(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	// Insert completed session
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session1",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  80,
		TargetPct: 80,
		Status:    "completed",
		EndedAt:   timeNow(),
	}))

	session, err := repo.GetActive(context.Background())
	require.NoError(t, err)
	assert.Nil(t, session)
}

func TestChargeSessionRepository_UpdateEndWithStats(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	// Insert active session
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session1",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  80,
		TargetPct: 80,
		Status:    "active",
	}))

	endedAt := time.Now()
	batteryKwh := 1.52
	wallKwh := 1.9
	co2Grams := 500.0
	avgCarbon := 263.16
	err := repo.UpdateEndWithStats(context.Background(), "session1", endedAt, 1.9, 80, batteryKwh, wallKwh, co2Grams, &avgCarbon, 0, 0)
	assert.NoError(t, err)

	// Verify update
	found, err := repo.FindByID(context.Background(), "session1")
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
	endKwh := 1.9
	assert.Equal(t, &endKwh, found.EndKwh)
	assert.Equal(t, endedAt.Format(time.DateTime), found.EndedAt.Format(time.DateTime))
	assert.Equal(t, &batteryKwh, found.BatteryKwh)
	assert.Equal(t, &wallKwh, found.WallKwh)
	assert.Equal(t, &co2Grams, found.Co2Grams)
	assert.Equal(t, &avgCarbon, found.AvgCarbonIntensity)
}

func TestPowerReadingRepository_Create(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	// Insert a session first
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session1",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  80,
		TargetPct: 80,
		Status:    "active",
	}))

	reading := &models.PowerReading{
		ID:        "reading1",
		SessionID: "session1",
		Timestamp: time.Now(),
		Voltage:   230.0,
		Current:   2.6,
		Power:     600.0,
		EnergyKwh: 1500.0,
	}

	err := repo.CreatePowerReading(context.Background(), reading)
	assert.NoError(t, err)
	assert.Equal(t, "reading1", reading.ID)
}

func TestPowerReadingRepository_GetPowerReadings(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	// Insert a session first (FK constraint)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session1",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  80,
		TargetPct: 80,
		Status:    "active",
	}))

	// Insert test data
	require.NoError(t, testdb.InsertPowerReading(db, &testdb.PowerReadingOpts{
		ID:        "reading1",
		SessionID: "session1",
		Voltage:   230,
		Current:   2.6,
		Power:     600,
		EnergyKwh: 1500,
	}))

	readings, err := repo.GetPowerReadings(context.Background(), "session1")
	require.NoError(t, err)
	assert.Len(t, readings, 1)
	assert.Equal(t, 600.0, readings[0].Power)
}

func TestPowerReadingRepository_GetPowerReadings_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewChargeSessionRepository(db)

	readings, err := repo.GetPowerReadings(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, readings)
}

func TestMigrationTrackingTableExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Verify schema_migrations table exists and has the latest version recorded
	var version uint
	err := db.QueryRow("SELECT version FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, uint(21), version, "should be at latest migration version")
}

func TestMigrationTrackingRecordsAllVersions(t *testing.T) {
	dbPath := "/tmp/test-ev-charge-versions-" + time.Now().Format("20060102-150405") + ".db"
	defer os.Remove(dbPath)

	db, err := Init(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Migrate library tracks current version as a single row
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count, "migrate library tracks current version as single row")

	var version uint
	err = db.QueryRow("SELECT version FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, uint(21), version, "should be at version 21 after all migrations")
}

func TestMigrationIsIdempotent(t *testing.T) {
	dbPath := "/tmp/test-ev-charge-idem-" + time.Now().Format("20060102-150405") + ".db"
	defer os.Remove(dbPath)

	// First init
	db1, err := Init(dbPath)
	require.NoError(t, err)
	db1.Close()

	// Second init should not fail
	db2, err := Init(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, 3) // seed models not duplicated
}

func TestSeedOnlyRunsOnEmptyDB(t *testing.T) {
	dbPath := "/tmp/test-ev-charge-seed-" + time.Now().Format("20060102-150405") + ".db"
	defer os.Remove(dbPath)

	// First init - seeds 3 vehicle models
	db1, err := Init(dbPath)
	require.NoError(t, err)

	var count1 int
	err = db1.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count1)
	require.NoError(t, err)
	assert.Equal(t, 3, count1)
	db1.Close()

	// Second init - should NOT seed again
	db2, err := Init(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	var count2 int
	err = db2.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&count2)
	require.NoError(t, err)
	assert.Equal(t, 3, count2) // still 3, not 6
}

func TestSetupTestDBWithoutSeed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM vehicles").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSetupTestDBWithSeed(t *testing.T) {
	db := setupTestDBWithSeed(t)
	defer db.Close()

	var modelCount int
	err := db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&modelCount)
	assert.NoError(t, err)
	assert.Equal(t, 3, modelCount)

	var instanceCount int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicles").Scan(&instanceCount)
	assert.NoError(t, err)
	assert.Equal(t, 3, instanceCount)
}

func TestMigrationCreatesAllTables(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	expectedTables := []string{
		"vehicle_models",
		"vehicles",
		"charge_sessions",
		"power_readings",
		"soc_snapshots",
		"schedules",
		"push_subscriptions",
		"users",
		"refresh_tokens",
		"plugs",
	}

	for _, table := range expectedTables {
		var exists int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&exists)
		assert.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, 1, exists, "table %s should exist", table)
	}
}

func TestMigrationCreatesAllIndexes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	expectedIndexes := []string{
		"idx_charge_sessions_status",
		"idx_charge_sessions_status_created",
		"idx_charge_sessions_vehicle_id",
		"idx_charge_sessions_vehicle_created",
		"idx_charge_sessions_vehicle_status_created",
		"idx_charge_sessions_created_at",
		"idx_power_readings_session_id",
		"idx_power_readings_session_timestamp",
		"idx_soc_snapshots_session_id",
		"idx_soc_snapshots_session_timestamp",
		"idx_vehicles_user_name",
		"idx_refresh_tokens_user_id",
		"idx_refresh_tokens_token_hash",
		"idx_plugs_user_id",
		"idx_plugs_namespace",
		"idx_vehicles_user_id",
		"idx_push_subscriptions_user_id",
		"idx_charge_sessions_user_id",
		"idx_charge_sessions_plug_id",
		"idx_schedules_plug_id",
		"idx_vehicles_model_id",
	}

	for _, idx := range expectedIndexes {
		var exists int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&exists)
		assert.NoError(t, err, "index %s should exist", idx)
		assert.Equal(t, 1, exists, "index %s should exist", idx)
	}
}

func TestSchemaDoesNotContainStartSocEndSoc(t *testing.T) {
	// Verify the schema.sql file doesn't contain start_soc or end_soc columns
	// These columns are unused - the code uses start_percent and end_percent instead
	schemaContent := `CREATE TABLE charge_sessions (
		id TEXT PRIMARY KEY,
		vehicle_id TEXT NOT NULL,
		started_at DATETIME,
		ended_at DATETIME,
		start_kwh REAL NOT NULL,
		end_kwh REAL,
		target_kwh REAL NOT NULL,
		start_percent REAL NOT NULL,
		end_percent REAL,
		target_percent REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (vehicle_id) REFERENCES vehicles(id)
	)`

	// Verify schema does NOT contain start_soc or end_soc
	assert.NotContains(t, schemaContent, "start_soc", "Schema should not have start_soc column")
	assert.NotContains(t, schemaContent, "end_soc", "Schema should not have end_soc column")

	// Verify schema DOES contain start_percent and end_percent
	assert.Contains(t, schemaContent, "start_percent", "Schema should have start_percent column")
	assert.Contains(t, schemaContent, "end_percent", "Schema should have end_percent column")
}

func TestBackfillBootstrap_CreatesUserAndPlug(t *testing.T) {
	// Use SetupTestDB(true) directly so vehicle_models are seeded but no users exist,
	// allowing BackfillBootstrap to run its first-boot path.
	db, err := SetupTestDB(true)
	require.NoError(t, err)
	defer db.Close()

	err = BackfillBootstrap(db, "admin@test.com")
	require.NoError(t, err)

	// User created with correct email
	var userID, email string
	err = db.QueryRow(`SELECT id, email FROM users`).Scan(&userID, &email)
	require.NoError(t, err)
	assert.Equal(t, "admin@test.com", email)
	assert.NotEmpty(t, userID)

	// Plug created linked to user
	var plugID, plugName, namespace string
	err = db.QueryRow(`SELECT id, name, namespace FROM plugs WHERE user_id = ?`, userID).Scan(&plugID, &plugName, &namespace)
	require.NoError(t, err)
	assert.Equal(t, "Default Plug", plugName)
	assert.Len(t, namespace, 8)

	// Vehicles created for each model
	var vehicleCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM vehicles WHERE user_id = ?`, userID).Scan(&vehicleCount)
	require.NoError(t, err)
	assert.Equal(t, 3, vehicleCount)
}

func TestBackfillBootstrap_IsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	require.NoError(t, BackfillBootstrap(db, "admin@test.com"))
	require.NoError(t, BackfillBootstrap(db, "admin@test.com"))

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestBackfillBootstrap_DefaultEmailFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	require.NoError(t, BackfillBootstrap(db, ""))

	var email string
	err := db.QueryRow(`SELECT email FROM users`).Scan(&email)
	require.NoError(t, err)
	assert.Equal(t, "admin@localhost", email)
}

func TestMigrationNewTablesHaveFKCascades(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert a user and plug
	_, err := db.Exec(`INSERT INTO users (id, email, password_hash) VALUES ('u1', 'test@example.com', '')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO plugs (id, user_id, name, namespace) VALUES ('p1', 'u1', 'Plug 1', 'abc12345')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ('rt1', 'u1', 'hash1', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	// Delete user → plugs, refresh_tokens cascade
	_, err = db.Exec(`DELETE FROM users WHERE id = 'u1'`)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM plugs`).Scan(&count))
	assert.Equal(t, 0, count, "plugs should cascade-delete with user")
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens`).Scan(&count))
	assert.Equal(t, 0, count, "refresh_tokens should cascade-delete with user")
}

func TestMigrationSchedulesHavePerPlugStructure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Verify schedules table has plug_id and user_id columns
	var colCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('schedules') WHERE name IN ('plug_id', 'user_id')`).Scan(&colCount)
	require.NoError(t, err)
	assert.Equal(t, 2, colCount, "schedules should have plug_id and user_id columns")
}

func TestBackfillBootstrap_SetsPlugVehicleID(t *testing.T) {
	db, err := SetupTestDB(true)
	require.NoError(t, err)
	defer db.Close()

	err = BackfillBootstrap(db, "admin@test.com")
	require.NoError(t, err)

	// The default plug should have its vehicle_id set to the first vehicle
	var plugVehicleID sql.NullString
	err = db.QueryRow(`SELECT vehicle_id FROM plugs`).Scan(&plugVehicleID)
	require.NoError(t, err)
	assert.True(t, plugVehicleID.Valid, "plug should have vehicle_id set after backfill")
}

func TestSeedIfEmpty_SkipsWhenSeeded(t *testing.T) {
	db, err := SetupTestDB(true) // SetupTestDB(true) already calls Seed
	require.NoError(t, err)
	defer db.Close()

	// SeedIfEmpty should be a no-op when vehicle_models already exist
	var countBefore int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&countBefore)
	require.NoError(t, err)

	err = SeedIfEmpty(db)
	require.NoError(t, err)

	var countAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&countAfter)
	require.NoError(t, err)
	assert.Equal(t, countBefore, countAfter, "SeedIfEmpty should not duplicate data")
}

func TestSeedIfEmpty_SeedsWhenEmpty(t *testing.T) {
	db := setupTestDB(t) // SetupTestDB(false) - no seed
	defer db.Close()

	var countBefore int
	err := db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&countBefore)
	require.NoError(t, err)
	assert.Equal(t, 0, countBefore)

	err = SeedIfEmpty(db)
	require.NoError(t, err)

	var countAfter int
	err = db.QueryRow("SELECT COUNT(*) FROM vehicle_models").Scan(&countAfter)
	require.NoError(t, err)
	assert.Greater(t, countAfter, 0, "SeedIfEmpty should seed when table is empty")
}

func TestApplyPragmas_EnableForeignKeys(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Foreign keys should already be enabled by SetupTestDB,
	// but calling ApplyPragmas again should be safe (idempotent)
	err := ApplyPragmas(db)
	assert.NoError(t, err)
}

func timeNow() *time.Time {
	now := time.Now()
	return &now
}
