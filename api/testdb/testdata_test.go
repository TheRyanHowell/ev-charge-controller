package testdb

import (
	"database/sql"
	"testing"
	"time"

	"ev-charge-controller/api/database"

	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsertUser(t *testing.T) {
	db := setupDB(t)

	err := InsertUser(db, "test-user", "test@example.com", "hash123")
	require.NoError(t, err)

	var id, email, hash string
	var createdAt time.Time
	err = db.QueryRow("SELECT id, email, password_hash, created_at FROM users WHERE id = ?", "test-user").Scan(&id, &email, &hash, &createdAt)
	require.NoError(t, err)
	require.Equal(t, "test-user", id)
	require.Equal(t, "test@example.com", email)
	require.Equal(t, "hash123", hash)
	require.False(t, createdAt.IsZero())

	// Idempotent - second call should not error
	err = InsertUser(db, "test-user", "different@example.com", "different")
	require.NoError(t, err)

	// Verify original values unchanged
	err = db.QueryRow("SELECT email FROM users WHERE id = 'test-user'").Scan(&email)
	require.NoError(t, err)
	require.Equal(t, "test@example.com", email)
}

func TestInsertPlug(t *testing.T) {
	db := setupDB(t)

	err := InsertUser(db, "test-user", "test@example.com", "")
	require.NoError(t, err)

	err = InsertPlug(db, "test-plug", "test-user", "Test Plug", "ns-test", "test-topic")
	require.NoError(t, err)

	var id, userID, name, namespace, mqttTopic string
	var createdAt time.Time
	err = db.QueryRow("SELECT id, user_id, name, namespace, mqtt_topic, created_at FROM plugs WHERE id = ?", "test-plug").Scan(&id, &userID, &name, &namespace, &mqttTopic, &createdAt)
	require.NoError(t, err)
	require.Equal(t, "test-plug", id)
	require.Equal(t, "test-user", userID)
	require.Equal(t, "Test Plug", name)
	require.Equal(t, "ns-test", namespace)
	require.Equal(t, "test-topic", mqttTopic)
	require.False(t, createdAt.IsZero())

	// Idempotent
	err = InsertPlug(db, "test-plug", "test-user", "Different Name", "ns-different", "different-topic")
	require.NoError(t, err)

	var nameAfter string
	err = db.QueryRow("SELECT name FROM plugs WHERE id = 'test-plug'").Scan(&nameAfter)
	require.NoError(t, err)
	require.Equal(t, "Test Plug", nameAfter)
}

func TestInsertVehicle(t *testing.T) {
	db := setupDB(t)

	err := InsertUser(db, "test-user", "test@example.com", "")
	require.NoError(t, err)

	err = InsertVehicle(db, "rm1", "test-user", "rm1", "Maeving RM1", 20, 80)
	require.NoError(t, err)

	var id, modelID string
	var currentPct, targetPct float64
	err = db.QueryRow("SELECT id, model_id, current_percent, target_percent FROM vehicles WHERE id = ?", "rm1").Scan(&id, &modelID, &currentPct, &targetPct)
	require.NoError(t, err)
	require.Equal(t, "rm1", id)
	require.Equal(t, "rm1", modelID)
	require.Equal(t, 20.0, currentPct)
	require.Equal(t, 80.0, targetPct)

	// Idempotent
	err = InsertVehicle(db, "rm1", "test-user", "rm1s", "Different", 30, 90)
	require.NoError(t, err)

	var modelIDAft string
	err = db.QueryRow("SELECT model_id FROM vehicles WHERE id = 'rm1'").Scan(&modelIDAft)
	require.NoError(t, err)
	require.Equal(t, "rm1", modelIDAft)
}

func TestInsertVehicleWithModel(t *testing.T) {
	db := setupDB(t)

	err := InsertUser(db, "test-user", "test@example.com", "")
	require.NoError(t, err)

	err = InsertVehicleWithModel(db, "custom", "test-user", "Custom Model", 10.0, 1200, 0.9, 50, 100)
	require.NoError(t, err)

	// Check vehicle_model
	var modelID, modelName string
	var capacity, chargerW, eff, rangeMin, rangeMax float64
	err = db.QueryRow("SELECT id, name, capacity_kwh, charger_output_w, charging_efficiency, range_min_mi, range_max_mi FROM vehicle_models WHERE id = ?", "custom").Scan(&modelID, &modelName, &capacity, &chargerW, &eff, &rangeMin, &rangeMax)
	require.NoError(t, err)
	require.Equal(t, "custom", modelID)
	require.Equal(t, "Custom Model", modelName)
	require.Equal(t, 10.0, capacity)
	require.Equal(t, 1200.0, chargerW)
	require.Equal(t, 0.9, eff)
	require.Equal(t, 50.0, rangeMin)
	require.Equal(t, 100.0, rangeMax)

	// Check vehicle instance
	var vid, vModelID string
	err = db.QueryRow("SELECT id, model_id FROM vehicles WHERE id = ?", "custom").Scan(&vid, &vModelID)
	require.NoError(t, err)
	require.Equal(t, "custom", vid)
	require.Equal(t, "custom", vModelID)
}

func TestInsertChargeSession_Pending(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-pending",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "pending",
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	var status string
	err = db.QueryRow("SELECT status FROM charge_sessions WHERE id = ?", "cs-pending").Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "pending", status)
}

func TestInsertChargeSession_Completed(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	now := time.Now()
	endedAt := now.Add(time.Hour)
	endKwh := 1.6
	endPct := 80.0
	batteryKwh := 1.2
	wallKwh := 1.5
	co2Grams := 300.0

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:         "cs-completed",
		VehicleID:  DefaultVehicleID,
		UserID:     DefaultUserID,
		PlugID:     DefaultPlugID,
		Status:     "completed",
		CreatedAt:  now,
		StartedAt:  &now,
		EndedAt:    &endedAt,
		StartKwh:   0.4,
		EndKwh:     &endKwh,
		TargetKwh:  1.6,
		StartPct:   20,
		EndPct:     &endPct,
		TargetPct:  80,
		BatteryKwh: &batteryKwh,
		WallKwh:    &wallKwh,
		Co2Grams:   &co2Grams,
	})
	require.NoError(t, err)

	var id, status string
	var startKwh, tKwh, startPct, targetPct float64
	var endKwhDB, endPctDB, batKwh, wallKwhDB, co2DB sql.NullFloat64
	var createdAt, startedAt, endedAtDB string
	err = db.QueryRow(`SELECT id, status, start_kwh, target_kwh, start_percent, target_percent,
		end_kwh, end_percent, battery_kwh, wall_kwh, co2_grams, created_at, started_at, ended_at
		FROM charge_sessions WHERE id = ?`, "cs-completed").Scan(
		&id, &status, &startKwh, &tKwh, &startPct, &targetPct,
		&endKwhDB, &endPctDB, &batKwh, &wallKwhDB, &co2DB,
		&createdAt, &startedAt, &endedAtDB,
	)
	require.NoError(t, err)
	require.Equal(t, "cs-completed", id)
	require.Equal(t, "completed", status)
	require.Equal(t, 0.4, startKwh)
	require.Equal(t, 1.6, tKwh)
	require.Equal(t, 20.0, startPct)
	require.Equal(t, 80.0, targetPct)
	require.True(t, endKwhDB.Valid)
	require.Equal(t, 1.6, endKwhDB.Float64)
	require.True(t, endPctDB.Valid)
	require.Equal(t, 80.0, endPctDB.Float64)
	require.True(t, batKwh.Valid)
	require.Equal(t, 1.2, batKwh.Float64)
	require.True(t, wallKwhDB.Valid)
	require.Equal(t, 1.5, wallKwhDB.Float64)
	require.True(t, co2DB.Valid)
	require.Equal(t, 300.0, co2DB.Float64)
}

func TestInsertChargeSession_Active(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	now := time.Now()
	startedAt := now.Add(-time.Minute)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-active",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "active",
		CreatedAt: now,
		StartedAt: &startedAt,
		StartKwh:  0.5,
		TargetKwh: 1.6,
		StartPct:  25,
		TargetPct: 80,
	})
	require.NoError(t, err)

	var status, startedAtStr string
	err = db.QueryRow("SELECT status, started_at FROM charge_sessions WHERE id = ?", "cs-active").Scan(&status, &startedAtStr)
	require.NoError(t, err)
	require.Equal(t, "active", status)
	require.NotEmpty(t, startedAtStr)
}

func TestInsertPowerReading(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	sessionID := "cs-power-test"
	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        sessionID,
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "active",
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	now := time.Now()
	opts := &PowerReadingOpts{
		SessionID:       sessionID,
		Timestamp:       now,
		Voltage:         230,
		Current:         32.5,
		Power:           7475,
		EnergyKwh:       1.234,
		CarbonIntensity: ptrFloat(450.0),
	}

	err = InsertPowerReading(db, opts)
	require.NoError(t, err)

	var sID string
	var ts time.Time
	var voltage, current, power, energy float64
	err = db.QueryRow("SELECT session_id, timestamp, voltage, current, power, energy_kwh FROM power_readings WHERE session_id = ?", sessionID).Scan(&sID, &ts, &voltage, &current, &power, &energy)
	require.NoError(t, err)
	require.Equal(t, sessionID, sID)
	require.Equal(t, 230.0, voltage)
	require.Equal(t, 32.5, current)
	require.Equal(t, 7475.0, power)
	require.Equal(t, 1.234, energy)
}

func TestInsertSOCSnapshot(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	sessionID := "cs-soc-test"
	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        sessionID,
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "active",
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	now := time.Now()
	opts := &SOCSnapshotOpts{
		SessionID:  sessionID,
		Timestamp:  now,
		SocPercent: 45.5,
	}

	err = InsertSOCSnapshot(db, opts)
	require.NoError(t, err)

	var sID string
	var soc float64
	err = db.QueryRow("SELECT session_id, soc_percent FROM soc_snapshots WHERE session_id = ?", sessionID).Scan(&sID, &soc)
	require.NoError(t, err)
	require.Equal(t, sessionID, sID)
	require.Equal(t, 45.5, soc)
}

func TestInsertSchedule(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	err := InsertSchedule(db, &ScheduleOpts{
		ID:     "sched-1",
		PlugID: DefaultPlugID,
		UserID: DefaultUserID,
		Time:   "06:00",
		Enabled: true,
	})
	require.NoError(t, err)

	var id string
	var schedTime string
	var enabled int
	err = db.QueryRow("SELECT id, time, enabled FROM schedules WHERE id = ?", "sched-1").Scan(&id, &schedTime, &enabled)
	require.NoError(t, err)
	require.Equal(t, "sched-1", id)
	require.Equal(t, "06:00", schedTime)
	require.Equal(t, 1, enabled)
}

func TestInsertRefreshToken(t *testing.T) {
	db := setupDB(t)
	SeedDefaultUser(t, db)

	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour)

	opts := &RefreshTokenOpts{
		UserID:    DefaultUserID,
		TokenHash: "hash123",
		ExpiresAt: expiresAt,
	}

	err := InsertRefreshToken(db, opts)
	require.NoError(t, err)

	var userID, hash string
	var exp time.Time
	err = db.QueryRow("SELECT user_id, token_hash, expires_at FROM refresh_tokens WHERE token_hash = ?", "hash123").Scan(&userID, &hash, &exp)
	require.NoError(t, err)
	require.Equal(t, DefaultUserID, userID)
	require.Equal(t, "hash123", hash)
	require.WithinDuration(t, expiresAt, exp, time.Second)
}

func TestActivateSession(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-1",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "pending",
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	err = ActivateSession(db, "cs-1")
	require.NoError(t, err)

	var status string
	err = db.QueryRow("SELECT status FROM charge_sessions WHERE id = ?", "cs-1").Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "active", status)
}

func TestActivateSession_NotPending(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-1",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "completed",
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	err = ActivateSession(db, "cs-1")
	require.Error(t, err)
	require.Equal(t, ErrSessionWrongState, err)
}

func TestCompleteSession(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	now := time.Now()
	endedAt := now.Add(time.Hour)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-1",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "active",
		CreatedAt: now,
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	err = CompleteSession(db, "cs-1", endedAt, 1.6, 80, 1.2, 1.5, 300, ptrFloat(200))
	require.NoError(t, err)

	var status string
	var endKwh, endPct, batKwh, wallKwh, co2 float64
	err = db.QueryRow("SELECT status, end_kwh, end_percent, battery_kwh, wall_kwh, co2_grams FROM charge_sessions WHERE id = ?", "cs-1").Scan(&status, &endKwh, &endPct, &batKwh, &wallKwh, &co2)
	require.NoError(t, err)
	require.Equal(t, "completed", status)
	require.Equal(t, 1.6, endKwh)
	require.Equal(t, 80.0, endPct)
	require.Equal(t, 1.2, batKwh)
	require.Equal(t, 1.5, wallKwh)
	require.Equal(t, 300.0, co2)
}

func TestCancelSession(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	now := time.Now()
	endedAt := now.Add(30 * time.Minute)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-1",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "active",
		CreatedAt: now,
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	err = CancelSession(db, "cs-1", endedAt)
	require.NoError(t, err)

	var status string
	err = db.QueryRow("SELECT status FROM charge_sessions WHERE id = ?", "cs-1").Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "cancelled", status)
}

func TestCancelPendingSession(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	now := time.Now()
	endedAt := now.Add(30 * time.Minute)

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-1",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "pending",
		CreatedAt: now,
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	err = CancelPendingSession(db, "cs-1", endedAt)
	require.NoError(t, err)

	var status string
	err = db.QueryRow("SELECT status FROM charge_sessions WHERE id = ?", "cs-1").Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "cancelled", status)
}

func TestBackdateSession(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	now := time.Now()

	err := InsertChargeSession(db, &ChargeSessionOpts{
		ID:        "cs-1",
		VehicleID: DefaultVehicleID,
		UserID:    DefaultUserID,
		PlugID:    DefaultPlugID,
		Status:    "completed",
		CreatedAt: now,
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	})
	require.NoError(t, err)

	past := now.Add(-2 * time.Hour)
	err = BackdateSession(db, "cs-1", past)
	require.NoError(t, err)

	var createdAt string
	err = db.QueryRow("SELECT created_at FROM charge_sessions WHERE id = ?", "cs-1").Scan(&createdAt)
	require.NoError(t, err)
	require.Contains(t, createdAt, past.Format("2006-01-02"))
}

func TestSetVehicleEfficiency(t *testing.T) {
	db := setupDB(t)

	err := SetVehicleEfficiency(db, "rm1", 0.85)
	require.NoError(t, err)

	var eff float64
	err = db.QueryRow("SELECT charging_efficiency FROM vehicle_models WHERE id = ?", "rm1").Scan(&eff)
	require.NoError(t, err)
	require.Equal(t, 0.85, eff)
}

func TestSetVehicleCapacity(t *testing.T) {
	db := setupDB(t)

	err := SetVehicleCapacity(db, "rm1", 5.0)
	require.NoError(t, err)

	var cap float64
	err = db.QueryRow("SELECT capacity_kwh FROM vehicle_models WHERE id = ?", "rm1").Scan(&cap)
	require.NoError(t, err)
	require.Equal(t, 5.0, cap)
}

func TestSetChargerOutput(t *testing.T) {
	db := setupDB(t)

	err := SetChargerOutput(db, "rm1", 600)
	require.NoError(t, err)

	var output float64
	err = db.QueryRow("SELECT charger_output_w FROM vehicle_models WHERE id = ?", "rm1").Scan(&output)
	require.NoError(t, err)
	require.Equal(t, 600.0, output)
}

func TestSeedDefaultUser(t *testing.T) {
	db := setupDB(t)
	SeedDefaultUser(t, db)

	var id, email string
	err := db.QueryRow("SELECT id, email FROM users WHERE id = ?", DefaultUserID).Scan(&id, &email)
	require.NoError(t, err)
	require.Equal(t, DefaultUserID, id)
	require.Equal(t, "test@example.com", email)
}

func TestSeedDefaultPlug(t *testing.T) {
	db := setupDB(t)
	SeedDefaultUser(t, db)
	SeedDefaultPlug(t, db)

	var id, userID, name string
	err := db.QueryRow("SELECT id, user_id, name FROM plugs WHERE id = ?", DefaultPlugID).Scan(&id, &userID, &name)
	require.NoError(t, err)
	require.Equal(t, DefaultPlugID, id)
	require.Equal(t, DefaultUserID, userID)
	require.Equal(t, "Test", name)
}

func TestSeedDefaultVehicles(t *testing.T) {
	db := setupDB(t)
	SeedDefaultUser(t, db)
	SeedDefaultVehicles(t, db)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM vehicles WHERE user_id = ?", DefaultUserID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	// Verify rm1 exists
	var modelID string
	err = db.QueryRow("SELECT model_id FROM vehicles WHERE id = ?", DefaultVehicleID).Scan(&modelID)
	require.NoError(t, err)
	require.Equal(t, "rm1", modelID)
}

func TestSeedFullTestDB(t *testing.T) {
	db := setupDB(t)
	SeedFullTestDB(t, db)

	var userCount, plugCount, vehicleCount int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, userCount, 1)

	err = db.QueryRow("SELECT COUNT(*) FROM plugs").Scan(&plugCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, plugCount, 1)

	err = db.QueryRow("SELECT COUNT(*) FROM vehicles").Scan(&vehicleCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, vehicleCount, 3)
}

func TestSeedMultiUser(t *testing.T) {
	db := setupDB(t)
	SeedMultiUser(t, db)

	var userCount int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, userCount, 3)
}

func ptrFloat(f float64) *float64 {
	return &f
}
