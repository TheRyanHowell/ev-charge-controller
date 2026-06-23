package main

import (
	"database/sql"
	"testing"
	"time"

	"ev-charge-controller/api/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	testDB, err := database.SetupTestDB(true)
	require.NoError(t, err)
	db = testDB
	_, err = testDB.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES ('seed-user', 'seed@test.com', '')`)
	require.NoError(t, err)
	for _, modelID := range []string{"rm1", "rm1s", "rm2", "rm1_dual"} {
		_, _ = testDB.Exec(
			`INSERT OR IGNORE INTO vehicle_models (id, name, capacity_kwh, charger_output_w, charging_efficiency, range_min_mi, range_max_mi) VALUES (?, ?, 2.0, 600, 0.8, 0, 0)`,
			modelID, modelID,
		)
		_, err = testDB.Exec(
			`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, 'seed-user', ?, ?, 20, 80, CURRENT_TIMESTAMP)`,
			modelID, modelID, modelID,
		)
		require.NoError(t, err)
	}
	_, err = testDB.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic, created_at) VALUES ('plug-1', 'seed-user', 'Plug 1', 'ns1', 'ns1', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = testDB.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic, created_at) VALUES ('plug-2', 'seed-user', 'Plug 2', 'ns2', 'ns2', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return testDB
}

func TestGenerateSessions_TotalCount(t *testing.T) {
	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)
	assert.Greater(t, len(sessions), 100, "expected ~200 sessions per vehicle across 180 days")
}

func TestGenerateSessions_StatusDistribution(t *testing.T) {
	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)

	completed := 0
	cancelled := 0
	for _, s := range sessions {
		switch s.status {
		case "completed":
			completed++
		case "cancelled":
			cancelled++
		}
	}
	assert.Greater(t, completed, 0, "should have completed sessions")
	assert.Equal(t, 2, cancelled, "should have 2 cancelled sessions (one per vehicle with plugs)")
}

func TestGenerateSessions_VehicleDistribution(t *testing.T) {
	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)

	counts := map[string]int{}
	for _, s := range sessions {
		counts[s.vehicleID]++
	}
	// Only vehicles with plugs should have sessions
	assert.Greater(t, counts["rm1"], 0, "rm1 should have sessions")
	assert.Greater(t, counts["rm1s"], 0, "rm1s should have sessions")
	assert.Equal(t, 0, counts["rm2"], "rm2 (no plug) should have no sessions")
}

func TestGenerateSessions_CompletedHaveValidRange(t *testing.T) {
	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)

	for _, s := range sessions {
		if s.status != "completed" {
			continue
		}
		spec := specs[s.vehicleID]
		startKwh := spec.capacityKwh * s.startPct / 100
		endKwh := spec.capacityKwh * s.endPct / 100

		assert.Positive(t, startKwh)
		assert.Positive(t, endKwh)
		assert.Greater(t, endKwh, startKwh, "end_kwh should be greater than start_kwh for completed session")
	}
}

func TestGenerateSessions_CancelledHavePartialRange(t *testing.T) {
	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)

	for _, s := range sessions {
		if s.status != "cancelled" {
			continue
		}
		delta := s.endPct - s.startPct
		assert.GreaterOrEqual(t, delta, 5.0, "cancelled session should have at least 5% delta")
		assert.LessOrEqual(t, delta, 25.0, "cancelled session should have at most 25% delta")
	}
}

func TestGenerateSessions_HasNullTotalKwh(t *testing.T) {
	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)

	hasNull := false
	for _, s := range sessions {
		if !s.hasTotalKwh {
			hasNull = true
			break
		}
	}
	assert.True(t, hasNull, "some sessions should have NULL start_total_kwh")
}

func TestInsertChargeSession_DataCorrectness(t *testing.T) {
	db := setupTestDB(t)

	vehicleIDs := []string{"rm1", "rm1s", "rm2"}
	vidToPlugID := map[string]string{"rm1": "plug-1", "rm1s": "plug-2", "rm2": "plug-1"}
	sessions := generateSessions(vehicleIDs, vidToPlugID)
	s := sessions[0]
	spec := specs[s.vehicleID]

	id := "test-session"
	startKwh := spec.capacityKwh * s.startPct / 100
	endKwh := spec.capacityKwh * s.endPct / 100
	targetKwh := spec.capacityKwh * s.endPct / 100
	startTime := time.Date(s.date.Year(), s.date.Month(), s.date.Day(), s.hour, s.minute, 0, 0, time.UTC)
	durationMin := (s.endPct - s.startPct) / 100 * float64(spec.time0to100Min)
	endTime := startTime.Add(time.Duration(durationMin * float64(time.Minute)))

	_, err := db.Exec(`
		INSERT INTO charge_sessions (
			id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
			start_percent, end_percent, target_kwh, target_percent,
			status, start_total_kwh, user_id, plug_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, s.vehicleID, startTime,
		&endTime, startKwh, &endKwh,
		s.startPct, &s.endPct, targetKwh, s.endPct,
		"completed", &startKwh, "seed-user", "plug-1",
	)
	require.NoError(t, err)

	var vid, status string
	var skwh, ekwh, stkwh sql.NullFloat64
	err = db.QueryRow(`SELECT vehicle_id, start_kwh, end_kwh, status, start_total_kwh FROM charge_sessions WHERE id = ?`, id).
		Scan(&vid, &skwh, &ekwh, &status, &stkwh)
	require.NoError(t, err)

	assert.Equal(t, s.vehicleID, vid)
	assert.True(t, skwh.Valid)
	assert.InDelta(t, startKwh, skwh.Float64, 0.001)
	assert.True(t, ekwh.Valid)
	assert.InDelta(t, endKwh, ekwh.Float64, 0.001)
	assert.Equal(t, "completed", status)
	assert.True(t, stkwh.Valid)
	assert.InDelta(t, startKwh, stkwh.Float64, 0.001)
}

func TestInsertPowerReadings_EnergyMatchesSession(t *testing.T) {
	db := setupTestDB(t)

	startTime := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(2*time.Hour + 30*time.Minute)
	startKwh := 0.5
	endKwh := 1.5
	startPct := 25.0
	endPct := 75.0

	_, err := db.Exec(`
		INSERT INTO charge_sessions (
			id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
			start_percent, end_percent, target_kwh, target_percent,
			status, start_total_kwh, user_id, plug_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-pr", "rm1", startTime, endTime,
		startKwh, endKwh, startPct, endPct, endKwh, endPct,
		"completed", &startKwh, "seed-user", "plug-1",
	)
	require.NoError(t, err)

	vidToModel := map[string]string{"rm1": "rm1"}
	insertPowerReadings("test-pr", vidToModel)

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM power_readings WHERE session_id = ?`, "test-pr").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 6, count, "expected 6 power readings (j=0..5)")

	var firstEnergy float64
	err = db.QueryRow(`SELECT energy_kwh FROM power_readings WHERE session_id = ? ORDER BY timestamp LIMIT 1`, "test-pr").Scan(&firstEnergy)
	require.NoError(t, err)
	assert.InDelta(t, startKwh, firstEnergy, 0.01, "first reading energy should match start_kwh")

	var lastEnergy float64
	err = db.QueryRow(`SELECT energy_kwh FROM power_readings WHERE session_id = ? ORDER BY timestamp DESC LIMIT 1`, "test-pr").Scan(&lastEnergy)
	require.NoError(t, err)
	assert.InDelta(t, endKwh, lastEnergy, 0.01, "last reading energy should match end_kwh")

	rows, err := db.Query(`SELECT power FROM power_readings WHERE session_id = ?`, "test-pr")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var power float64
		require.NoError(t, rows.Scan(&power))
		assert.InDelta(t, 480.0, power, 10.0, "battery-side power should be ~480W (600*0.8)")
	}
}

func TestInsertSOCSnapshots_SocMatchesSession(t *testing.T) {
	db := setupTestDB(t)

	startTime := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(2*time.Hour + 30*time.Minute)
	startKwh := 0.5
	endKwh := 1.5
	startPct := 25.0
	endPct := 75.0

	_, err := db.Exec(`
		INSERT INTO charge_sessions (
			id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
			start_percent, end_percent, target_kwh, target_percent,
			status, start_total_kwh, user_id, plug_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-soc", "rm1", startTime, endTime,
		startKwh, endKwh, startPct, endPct, endKwh, endPct,
		"completed", &startKwh, "seed-user", "plug-1",
	)
	require.NoError(t, err)

	insertSOCSnapshots("test-soc")

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM soc_snapshots WHERE session_id = ?`, "test-soc").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 6, count, "expected 6 SOC snapshots (j=0..5)")

	var firstSoc float64
	err = db.QueryRow(`SELECT soc_percent FROM soc_snapshots WHERE session_id = ? ORDER BY timestamp LIMIT 1`, "test-soc").Scan(&firstSoc)
	require.NoError(t, err)
	assert.InDelta(t, startPct, firstSoc, 0.01, "first snapshot SOC should match start_percent")

	var lastSoc float64
	err = db.QueryRow(`SELECT soc_percent FROM soc_snapshots WHERE session_id = ? ORDER BY timestamp DESC LIMIT 1`, "test-soc").Scan(&lastSoc)
	require.NoError(t, err)
	assert.InDelta(t, endPct, lastSoc, 0.01, "last snapshot SOC should match end_percent")

	var prevSoc float64 = -1
	rows, err := db.Query(`SELECT soc_percent FROM soc_snapshots WHERE session_id = ? ORDER BY timestamp`, "test-soc")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var soc float64
		require.NoError(t, rows.Scan(&soc))
		assert.GreaterOrEqual(t, soc, prevSoc, "SOC should be monotonically increasing")
		prevSoc = soc
	}
}

func TestInsertPowerReadings_SkipsActiveSessions(t *testing.T) {
	db := setupTestDB(t)

	startTime := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
		INSERT INTO charge_sessions (
			id, vehicle_id, created_at, start_kwh, start_percent,
			target_kwh, target_percent, status, user_id, plug_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"active-session", "rm1", startTime, 0.5, 25.0,
		1.9, 95.0, "active", "seed-user", "plug-1",
	)
	require.NoError(t, err)

	vidToModel := map[string]string{"rm1": "rm1"}
	insertPowerReadings("active-session", vidToModel)

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM power_readings WHERE session_id = ?`, "active-session").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "active sessions should not have power readings")
}

func TestInsertSOCSnapshots_SkipsActiveSessions(t *testing.T) {
	db := setupTestDB(t)

	startTime := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
		INSERT INTO charge_sessions (
			id, vehicle_id, created_at, start_kwh, start_percent,
			target_kwh, target_percent, status, user_id, plug_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"active-soc", "rm1", startTime, 0.5, 25.0,
		1.9, 95.0, "active", "seed-user", "plug-1",
	)
	require.NoError(t, err)

	insertSOCSnapshots("active-soc")

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM soc_snapshots WHERE session_id = ?`, "active-soc").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "active sessions should not have SOC snapshots")
}

func TestInsertPowerReadings_NoTotalKwhFallback(t *testing.T) {
	db := setupTestDB(t)

	startTime := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(2*time.Hour + 30*time.Minute)
	startKwh := 0.5
	endKwh := 1.5

	_, err := db.Exec(`
		INSERT INTO charge_sessions (
			id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
			start_percent, end_percent, target_kwh, target_percent,
			status, user_id, plug_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-null-kwh", "rm1", startTime, endTime,
		startKwh, endKwh, 25.0, 75.0, endKwh, 75.0,
		"completed", "seed-user", "plug-1",
	)
	require.NoError(t, err)

	vidToModel := map[string]string{"rm1": "rm1"}
	insertPowerReadings("test-null-kwh", vidToModel)

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM power_readings WHERE session_id = ?`, "test-null-kwh").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 6, count, "should still insert readings for sessions without start_total_kwh")

	var firstEnergy, lastEnergy float64
	err = db.QueryRow(`SELECT energy_kwh FROM power_readings WHERE session_id = ? ORDER BY timestamp LIMIT 1`, "test-null-kwh").Scan(&firstEnergy)
	require.NoError(t, err)
	err = db.QueryRow(`SELECT energy_kwh FROM power_readings WHERE session_id = ? ORDER BY timestamp DESC LIMIT 1`, "test-null-kwh").Scan(&lastEnergy)
	require.NoError(t, err)
	assert.InDelta(t, startKwh, firstEnergy, 0.01, "first reading should match start_kwh")
	assert.InDelta(t, endKwh, lastEnergy, 0.01, "last reading should match end_kwh")
}

func TestRFloat(t *testing.T) {
	for range 100 {
		v := rFloat(10.0, 20.0)
		assert.GreaterOrEqual(t, v, 10.0)
		assert.Less(t, v, 20.0)
	}
}

func TestMin(t *testing.T) {
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 5, min(7, 5))
	assert.Equal(t, 5, min(5, 5))
}

func TestSpecs(t *testing.T) {
	rm1 := specs["rm1"]
	assert.InDelta(t, 2.026, rm1.capacityKwh, 0.001)
	assert.InDelta(t, 600.0, rm1.chargerOutputW, 1.0)
	assert.InDelta(t, 0.8, rm1.efficiency, 0.01)
	assert.Equal(t, 250, rm1.time0to100Min)

	rm1s := specs["rm1s"]
	assert.InDelta(t, 5.46, rm1s.capacityKwh, 0.001)
	assert.InDelta(t, 1200.0, rm1s.chargerOutputW, 1.0)
	assert.Equal(t, 360, rm1s.time0to100Min)

	rm2 := specs["rm2"]
	assert.InDelta(t, 5.46, rm2.capacityKwh, 0.001)
	assert.InDelta(t, 1200.0, rm2.chargerOutputW, 1.0)
	assert.Equal(t, 360, rm2.time0to100Min)
}
