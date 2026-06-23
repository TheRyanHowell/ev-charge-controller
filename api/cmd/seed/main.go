package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	autopaho "github.com/eclipse/paho.golang/autopaho"
	pahopkg "github.com/eclipse/paho.golang/paho"

	"ev-charge-controller/api/database"
	mqttpkg "ev-charge-controller/api/mqtt"
	"ev-charge-controller/api/services"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
)

var db *sql.DB

type vehicleSpec struct {
	capacityKwh    float64
	chargerOutputW float64
	efficiency     float64
	time0to100Min  int
}

var specs = map[string]vehicleSpec{
	"rm1":      {2.026, 600, 0.8, 250},
	"rm1_dual": {4.052, 600, 0.8, 250},
	"rm1s":     {5.46, 1200, 0.8, 360},
	"rm2":      {5.46, 1200, 0.8, 360},
}

type seedSession struct {
	vehicleID   string
	plugID      string
	date        time.Time
	hour        int
	minute      int
	startPct    float64
	endPct      float64
	status      string
	hasTotalKwh bool
}

var rng = rand.New(rand.NewSource(42))

// Seed data identifiers - printed so the user can log in.
const (
	seedEmail    = "test@example.com"
	seedPassword = "password123"
)

func main() {
	clearData := flag.Bool("clear", true, "Clear existing sessions before seeding")
	flag.Parse()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./ev-charge.db"
	}
	var err error
	db, err = database.Init(dbPath)
	if err != nil {
		slog.Error("Failed to initialize database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if *clearData {
		// Delete in FK order
		mustExec(db, "DELETE FROM power_readings")
		mustExec(db, "DELETE FROM soc_snapshots")
		mustExec(db, "DELETE FROM charge_sessions")
		mustExec(db, "DELETE FROM schedules")
		mustExec(db, "DELETE FROM plugs")
		mustExec(db, "DELETE FROM vehicles")
		mustExec(db, "DELETE FROM users")
		slog.Info("Cleared existing seed data")
	}

	seedUser()
	plugIDs, vehicleIDs := seedPlugsAndVehicles()
	userID := queryUserID()
	seedSchedule(plugIDs[0], userID)
	configureMockTasmota(plugIDs)
	vidToPlugID := map[string]string{
		vehicleIDs[0]: plugIDs[0],
		vehicleIDs[1]: plugIDs[1],
		vehicleIDs[2]: plugIDs[0],
	}
	sessions := generateSessions(vehicleIDs, vidToPlugID)
	insertSessions(sessions, vehicleIDs, userID)
	printSummary()
}

func seedUser() {
	userID := uuid.New().String()
	hash, err := argon2id.CreateHash(seedPassword, argon2id.DefaultParams)
	if err != nil {
		slog.Error("Failed to hash password", "err", err)
		os.Exit(1)
	}
	mustExec(db,
		"INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)",
		userID, seedEmail, hash,
	)
	slog.Info("Seeded user", "email", seedEmail, "id", userID)
}

// seedPlugsAndVehicles creates 2 plugs and 3 vehicles.
// Vehicle 1 (RM1) -> Plug 1, Vehicle 2 (RM1S) -> Plug 2, Vehicle 3 (RM2) -> no plug.
func seedPlugsAndVehicles() (plugIDs []string, vehicleIDs []string) {
	userID := queryUserID()

	// Create 3 vehicles from catalog models
	modelIDs := []string{"rm1", "rm1s", "rm2"}
	vehicleNames := []string{"My RM1", "My RM1S", "My RM2"}
	vehicleIDs = make([]string, len(modelIDs))
	for i, mid := range modelIDs {
		vID := uuid.New().String()
		vehicleIDs[i] = vID
		mustExec(db,
			"INSERT INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, 20.0, 80.0, CURRENT_TIMESTAMP)",
			vID, userID, mid, vehicleNames[i],
		)
	}

	// Create 2 plugs, assign to first 2 vehicles.
	// Reuse the same generators the API uses so seed plugs match production format.
	plugNames := []string{"Garage Plug", "Driveway Plug"}
	plugIDs = make([]string, 2)
	for i := range plugNames {
		pID := uuid.New().String()
		plugIDs[i] = pID
		namespace, err := services.GenerateNamespace()
		if err != nil {
			slog.Error("Failed to generate namespace", "err", err)
			os.Exit(1)
		}
		topic, err := services.GenerateTopic()
		if err != nil {
			slog.Error("Failed to generate topic", "err", err)
			os.Exit(1)
		}
		mustExec(db,
			"INSERT INTO plugs (id, user_id, name, namespace, mqtt_topic, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
			pID, userID, plugNames[i], namespace, topic,
		)
		mustExec(db,
			"UPDATE plugs SET vehicle_id = ? WHERE id = ?",
			vehicleIDs[i], pID,
		)
	}

	slog.Info("Seeded vehicles and plugs", "vehicles", len(vehicleIDs), "plugs", len(plugIDs))
	return plugIDs, vehicleIDs
}

func seedSchedule(plugID, userID string) {
	scheduleID := uuid.New().String()
	mustExec(db,
		"INSERT INTO schedules (id, plug_id, user_id, time, enabled) VALUES (?, ?, ?, '06:00', 1)",
		scheduleID, plugID, userID,
	)
	slog.Info("Seeded schedule", "plugID", plugID)
}

func queryUserID() string {
	var userID string
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", seedEmail).Scan(&userID)
	if err != nil {
		slog.Error("Failed to find seed user", "err", err)
		os.Exit(1)
	}
	return userID
}

func generateSessions(vehicleIDs []string, vidToPlugID map[string]string) []seedSession {
	var sessions []seedSession
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	addCompleted := func(vehicleID string, date time.Time, count int) {
		usedSlots := make(map[int]bool)
		for range count {
			hour := rng.Intn(22)
			minute := rng.Intn(60)
			slot := hour*60 + minute
			for usedSlots[slot] {
				hour = (hour + 1) % 23
				minute = rng.Intn(60)
				slot = hour*60 + minute
			}
			usedSlots[slot] = true

			startPct := 5.0 + rng.Float64()*30.0
			endPct := 60.0 + rng.Float64()*36.0
			if endPct <= startPct {
				endPct = startPct + 40
			}
			if endPct > 98 {
				endPct = 98
			}

			sessions = append(sessions, seedSession{
				vehicleID:   vehicleID,
				plugID:      vidToPlugID[vehicleID],
				date:        date,
				hour:        hour,
				minute:      minute,
				startPct:    startPct,
				endPct:      endPct,
				status:      "completed",
				hasTotalKwh: true,
			})
		}
	}

	// Generate ~200 sessions per vehicle over 180 days (only vehicles with plugs)
	for daysAgo := 0; daysAgo < 180; daysAgo++ {
		date := today.AddDate(0, 0, -daysAgo)

		sessionsPerVehicle := rng.Intn(3) // 0-2 per vehicle per day, avg ~1.1 → ~180 total

		for _, vid := range vehicleIDs[:2] {
			addCompleted(vid, date, sessionsPerVehicle)
		}
	}

	// Add some cancelled sessions (small delta: charged a bit then stopped)
	for _, vid := range vehicleIDs[:2] {
		startPct := 15.0 + rng.Float64()*15.0
		delta := 5.0 + rng.Float64()*20.0
		sessions = append(sessions, seedSession{
			vehicleID:   vid,
			plugID:      vidToPlugID[vid],
			date:        today.AddDate(0, 0, -rng.Intn(5)),
			hour:        rng.Intn(22),
			minute:      rng.Intn(60),
			startPct:    startPct,
			endPct:      startPct + delta,
			status:      "cancelled",
			hasTotalKwh: true,
		})
	}

	// Some sessions with NULL start_total_kwh (simulate old data)
	for i := range sessions {
		if sessions[i].status == "completed" && rng.Intn(10) == 0 {
			sessions[i].hasTotalKwh = false
		}
	}

	return sessions
}

func insertSessions(sessions []seedSession, vehicleIDs []string, userID string) {
	// Build vehicleID -> modelID mapping for specs lookup
	vidToModel := map[string]string{
		vehicleIDs[0]: "rm1",
		vehicleIDs[1]: "rm1s",
		vehicleIDs[2]: "rm2",
	}

	completedIDs := make([]string, 0)

	// Track per-vehicle aggregates for updating vehicles table
	type vehicleAgg struct {
		totalSessions        int
		totalBatteryKwh      float64
		totalWallKwh         float64
		totalCo2Grams        float64
		lastSessionAt        *time.Time
		minSessionBatteryKwh float64
		maxSessionBatteryKwh float64
	}
	aggs := make(map[string]*vehicleAgg)
	for _, vid := range vehicleIDs {
		aggs[vid] = &vehicleAgg{}
	}

	for i, s := range sessions {
		modelID := vidToModel[s.vehicleID]
		spec := specs[modelID]
		id := fmt.Sprintf("seed-%04d", i)

		startKwh := spec.capacityKwh * s.startPct / 100

		var targetPct float64
		switch s.status {
		case "completed":
			targetPct = s.endPct
		case "cancelled":
			targetPct = s.endPct + 30 + rng.Float64()*20
			if targetPct > 98 {
				targetPct = 98
			}
		case "active":
			targetPct = 80 + rng.Float64()*15
		}
		targetKwh := spec.capacityKwh * targetPct / 100

		startTime := time.Date(s.date.Year(), s.date.Month(), s.date.Day(), s.hour, s.minute, 0, 0, time.UTC)

		var endTime *time.Time
		var endKwh *float64
		var endPct *float64
		var totalKwh *float64

		// Session stats (only for completed sessions)
		var batteryKwh, wallKwh, avgCarbonIntensity, co2Grams *float64

		if s.status != "active" {
			durationMin := (s.endPct - s.startPct) / 100 * float64(spec.time0to100Min)
			et := startTime.Add(time.Duration(durationMin * float64(time.Minute)))
			endTime = &et

			ek := spec.capacityKwh * s.endPct / 100
			endKwh = &ek

			ep := s.endPct
			endPct = &ep
		}

		if s.hasTotalKwh {
			tk := startKwh
			totalKwh = &tk
		}

		// Compute stats for completed sessions at insert time
		if s.status == "completed" && endKwh != nil {
			bkwh := *endKwh - startKwh // battery-side energy
			if bkwh > 0 {
				wkwh := bkwh / spec.efficiency // wall-side energy
				// Carbon intensity varies by hour: lower overnight, higher at peak
				hour := float64(s.hour)
				baseCarbon := 200.0 + 150.0*(1.0-math.Cos((hour-14)*math.Pi/12.0))
				ci := baseCarbon + rFloat(-30, 30)
				if ci < 100 {
					ci = 100
				}
				co2 := wkwh * ci

				batteryKwh = &bkwh
				wallKwh = &wkwh
				avgCarbonIntensity = &ci
				co2Grams = &co2

				// Accumulate vehicle aggregates
				a := aggs[s.vehicleID]
				a.totalSessions++
				a.totalBatteryKwh += bkwh
				a.totalWallKwh += wkwh
				a.totalCo2Grams += co2
				if a.lastSessionAt == nil || endTime.After(*a.lastSessionAt) {
					a.lastSessionAt = endTime
				}
				if a.totalSessions == 1 {
					a.minSessionBatteryKwh = bkwh
					a.maxSessionBatteryKwh = bkwh
				} else {
					if bkwh < a.minSessionBatteryKwh {
						a.minSessionBatteryKwh = bkwh
					}
					if bkwh > a.maxSessionBatteryKwh {
						a.maxSessionBatteryKwh = bkwh
					}
				}
			}
		}

		_, err := db.Exec(`
			INSERT INTO charge_sessions (
				id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
				start_percent, end_percent, target_kwh, target_percent,
				status, start_total_kwh, user_id, plug_id,
				battery_kwh, wall_kwh, avg_carbon_intensity, co2_grams
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, s.vehicleID, startTime,
			endTime, startKwh, endKwh,
			s.startPct, endPct, targetKwh, targetPct,
			s.status, totalKwh, userID, s.plugID,
			batteryKwh, wallKwh, avgCarbonIntensity, co2Grams,
		)
		if err != nil {
			slog.Warn("failed to insert session", "id", id, "err", err)
			continue
		}

		if s.status == "completed" {
			completedIDs = append(completedIDs, id)
		}
	}

	// Insert power readings & SOC snapshots for completed sessions
	for _, id := range completedIDs {
		insertPowerReadings(id, vidToModel)
		insertSOCSnapshots(id)
	}

	// Update vehicle lifetime aggregates
	for vid, a := range aggs {
		var lastSessionAt any
		if a.lastSessionAt != nil {
			lastSessionAt = a.lastSessionAt.Format(time.RFC3339)
		}
		_, err := db.Exec(`
			UPDATE vehicles SET
				total_sessions = ?,
				total_battery_kwh = ?,
				total_wall_kwh = ?,
				total_co2_grams = ?,
				last_session_at = ?,
				min_session_battery_kwh = ?,
				max_session_battery_kwh = ?
			WHERE id = ?`,
			a.totalSessions, a.totalBatteryKwh, a.totalWallKwh,
			a.totalCo2Grams, lastSessionAt,
			a.minSessionBatteryKwh, a.maxSessionBatteryKwh, vid,
		)
		if err != nil {
			slog.Warn("failed to update vehicle aggregates", "vehicleID", vid, "err", err)
		}
	}
}

func insertPowerReadings(sessionID string, vidToModel map[string]string) {
	var (
		vid       string
		st        time.Time
		et        sql.NullTime
		skwh      float64
		ekwh      sql.NullFloat64
		stp       float64
		etp       sql.NullFloat64
		stotalKwh sql.NullFloat64
	)
	err := db.QueryRow(`
		SELECT vehicle_id, created_at, ended_at, start_kwh, end_kwh,
		       start_percent, end_percent, start_total_kwh
	FROM charge_sessions WHERE id = ?`, sessionID,
	).Scan(&vid, &st, &et, &skwh, &ekwh, &stp, &etp, &stotalKwh)
	if err != nil {
		return
	}
	if !et.Valid || !ekwh.Valid {
		return
	}

	modelID := vidToModel[vid]
	spec := specs[modelID]
	startTime := st
	endTime := et.Time

	numReadings := 5
	for j := 0; j <= numReadings; j++ {
		fraction := float64(j) / float64(numReadings)
		ts := startTime.Add(time.Duration(float64(endTime.Sub(startTime)) * fraction))

		voltage := rFloat(228, 234)
		batteryPower := spec.chargerOutputW*spec.efficiency + (rng.Float64()*20-10)*spec.efficiency
		current := batteryPower / voltage
		power := batteryPower

		var energyKwh float64
		if stotalKwh.Valid {
			batteryDelta := ekwh.Float64 - skwh
			energyKwh = stotalKwh.Float64 + batteryDelta*fraction
		} else {
			energyKwh = skwh + (ekwh.Float64 - skwh)*fraction
		}

		prID := fmt.Sprintf("pr-%s-%d", sessionID, j)
		// Carbon intensity varies by hour: lower overnight (renewables), higher at peak
		hour := float64(ts.Hour())
		baseCarbon := 200.0 + 150.0*(1.0-math.Cos((hour-14)*math.Pi/12.0))
		carbonIntensity := baseCarbon + rFloat(-30, 30)
		if carbonIntensity < 100 {
			carbonIntensity = 100
		}
		_, err := db.Exec(`
			INSERT INTO power_readings (id, session_id, timestamp, voltage, current, power, energy_kwh, carbon_intensity_g_co2_per_kwh)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			prID, sessionID, ts,
			voltage, current, power, energyKwh, carbonIntensity,
		)
		if err != nil {
			slog.Warn("failed to insert power reading", "prID", prID, "err", err)
		}
	}
}

func insertSOCSnapshots(sessionID string) {
	var (
		vid   string
		st    time.Time
		et    sql.NullTime
		skwh  float64
		ekwh  sql.NullFloat64
		stp   float64
		etp   sql.NullFloat64
	)
	err := db.QueryRow(`
		SELECT vehicle_id, created_at, ended_at, start_kwh, end_kwh,
		       start_percent, end_percent
	FROM charge_sessions WHERE id = ?`, sessionID,
	).Scan(&vid, &st, &et, &skwh, &ekwh, &stp, &etp)
	if err != nil {
		return
	}
	if !et.Valid || !etp.Valid {
		return
	}

	startTime := st
	endTime := et.Time

	numSnapshots := 5
	for j := 0; j <= numSnapshots; j++ {
		fraction := float64(j) / float64(numSnapshots)
		ts := startTime.Add(time.Duration(float64(endTime.Sub(startTime)) * fraction))
		soc := stp + (etp.Float64-stp)*fraction

		snID := fmt.Sprintf("soc-%s-%d", sessionID, j)
		_, err := db.Exec(`
			INSERT INTO soc_snapshots (id, session_id, timestamp, soc_percent)
			VALUES (?, ?, ?, ?)`,
			snID, sessionID, ts, soc,
		)
		if err != nil {
			slog.Warn("failed to insert soc snapshot", "snID", snID, "err", err)
		}
	}
}

func printSummary() {
	fmt.Printf("\nSeed complete!\n")
	fmt.Printf("Login: %s / %s\n\n", seedEmail, seedPassword)

	var sessionCount, prCount, socCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM charge_sessions").Scan(&sessionCount); err != nil {
		slog.Warn("failed to count sessions", "err", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM power_readings").Scan(&prCount); err != nil {
		slog.Warn("failed to count power readings", "err", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM soc_snapshots").Scan(&socCount); err != nil {
		slog.Warn("failed to count soc snapshots", "err", err)
	}

	fmt.Printf("Charge sessions: %d\n", sessionCount)
	fmt.Printf("Power readings:  %d\n", prCount)
	fmt.Printf("SOC snapshots:   %d\n", socCount)

	// By status
	fmt.Println("\nSessions by status:")
	rows, err := db.Query("SELECT status, COUNT(*) FROM charge_sessions GROUP BY status ORDER BY status")
	if err != nil {
		slog.Warn("failed to query by status", "err", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var status string
			var cnt int
			if err := rows.Scan(&status, &cnt); err != nil {
				continue
			}
			fmt.Printf("  %-12s %d\n", status, cnt)
		}
	}

	// By vehicle
	fmt.Println("\nSessions by vehicle:")
	rows2, err2 := db.Query(`
		SELECT v.name, COUNT(*) FROM charge_sessions cs
		JOIN vehicles v ON v.id = cs.vehicle_id
		GROUP BY v.id ORDER BY v.name`)
	if err2 != nil {
		slog.Warn("failed to query by vehicle", "err", err2)
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var name string
			var cnt int
			if err := rows2.Scan(&name, &cnt); err != nil {
				continue
			}
			fmt.Printf("  %-12s %d\n", name, cnt)
		}
	}
}

func mustExec(db *sql.DB, query string, args ...any) {
	_, err := db.Exec(query, args...)
	if err != nil {
		slog.Error("Exec failed", "query", query[:min(60, len(query))], "err", err)
		os.Exit(1)
	}
}

func rFloat(min, max float64) float64 {
	return min + rng.Float64()*(max-min)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// configureMockTasmota provisions MQTT credentials via dynsec and pushes
// configuration to mock-tasmota devices directly, without calling the API.
func configureMockTasmota(plugIDs []string) {
	tasmotaURLs := []string{
		getEnvOr("MOCK_TASMOTA_1_URL", "http://mock-tasmota:8081"),
		getEnvOr("MOCK_TASMOTA_2_URL", "http://mock-tasmota-2:8082"),
	}

	// Connect to mosquitto for dynsec provisioning.
	mqttBroker := getEnvOr("MQTT_BROKER_URL", "mqtt://mosquitto:1883")
	mqttUser := getEnvOr("MQTT_USERNAME", "")
	mqttPass := getEnvOr("MQTT_PASSWORD", "")
	dynsec, cm, err := connectDynsec(mqttBroker, mqttUser, mqttPass)
	if err != nil {
		slog.Warn("Cannot connect to MQTT broker, skipping dynsec provisioning (plugs will work without MQTT)", "err", err)
		dynsec = nil
		cm = nil
	}

	// Derive external broker host/port from MQTT_BROKER_URL (same source the API uses).
	bu, _ := url.Parse(mqttBroker)
	mqttExtHost := bu.Hostname()
	mqttExtPort := bu.Port()
	if mqttExtPort == "" {
		switch bu.Scheme {
		case "mqtt", "tcp":
			mqttExtPort = "1883"
		case "mqtts", "ssl", "wss":
			mqttExtPort = "8883"
		}
	}

	for i, plugID := range plugIDs {
		if i >= len(tasmotaURLs) {
			break
		}

		tasmotaURL := tasmotaURLs[i]
		tasmotaHost := strings.TrimPrefix(tasmotaURL, "http://")
		tasmotaHost = strings.TrimSuffix(tasmotaHost, "/")

		// Read plug namespace and topic from DB.
		var namespace, mqttTopic string
		if err := db.QueryRow("SELECT namespace, mqtt_topic FROM plugs WHERE id = ?", plugID).Scan(&namespace, &mqttTopic); err != nil {
			slog.Warn("Failed to read plug from DB", "plugID", plugID, "err", err)
			continue
		}

		slog.Info("Configuring mock-tasmota", "plugID", plugID, "url", tasmotaURL, "namespace", namespace)

		// Generate password.
		rawPw, err := services.GeneratePassword()
		if err != nil {
			slog.Warn("Failed to generate password", "plugID", plugID, "err", err)
			continue
		}

		// Provision dynsec.
		if dynsec != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if err := dynsec.ProvisionPlug(ctx, namespace, rawPw); err != nil {
				cancel()
				slog.Warn("Failed to provision dynsec", "plugID", plugID, "err", err)
			} else {
				cancel()
				slog.Info("dynsec: plug provisioned", "plugID", plugID)
			}
		}

		// Build and push all commands to mock-tasmota, including Restart 1.
		consoleCmd := services.BuildConsoleCommands(mqttExtHost, mqttExtPort, namespace, mqttTopic, rawPw)
		cmds := services.ParseBacklogCommands(consoleCmd)

		for i := 0; i < len(cmds); i++ {
			if err := pushTasmotaCommand(tasmotaHost, cmds[i]); err != nil {
				slog.Warn("Failed to push command to mock-tasmota", "plugID", plugID, "cmd", cmds[i], "err", err)
				break
			}
			time.Sleep(200 * time.Millisecond)
		}

		// Wait for mock-tasmota to connect after restart.
		if cm != nil {
			lwtTopic := fmt.Sprintf("evcc/%s/tele/%s/LWT", namespace, mqttTopic)
			if ok := waitForMQTTOnline(context.Background(), bu, mqttUser, []byte(mqttPass), lwtTopic, 30*time.Second); ok {
				slog.Info("mock-tasmota: online", "plugID", plugID, "lwt", lwtTopic)
			} else {
				slog.Warn("mock-tasmota: did not come online", "plugID", plugID)
			}
		}

		slog.Info("Configured mock-tasmota", "plugID", plugID)
	}
}

// connectDynsec creates an MQTT connection, waits for it to be ready, and
// returns a DynsecManager and the underlying ConnectionManager for verification.
func connectDynsec(brokerURL, username, password string) (*mqttpkg.DynsecManager, *autopaho.ConnectionManager, error) {
	bu, err := url.Parse(brokerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse broker URL: %w", err)
	}

	// Use a long-lived context for the connection manager.
	connCtx := context.Background()

	cfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{bu},
		KeepAlive:         30,
		ConnectRetryDelay: 2 * time.Second,
		ClientConfig: pahopkg.ClientConfig{
			ClientID: "seed-" + uuid.New().String()[:8],
		},
	}
	if username != "" {
		cfg.ConnectUsername = username
		cfg.ConnectPassword = []byte(password)
	}

	cm, err := autopaho.NewConnection(connCtx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("connect MQTT: %w", err)
	}

	// Only use timeout for awaiting the connection.
	awaitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cm.AwaitConnection(awaitCtx); err != nil {
		return nil, nil, fmt.Errorf("await MQTT connection: %w", err)
	}

	dynsec := mqttpkg.NewDynsecManager(&pahoPublisher{cm: cm})
	return dynsec, cm, nil
}

// pahoPublisher adapts autopaho.ConnectionManager to the pahoPublisher interface.
type pahoPublisher struct {
	cm *autopaho.ConnectionManager
}

func (p *pahoPublisher) Publish(ctx context.Context, pub *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
	return p.cm.Publish(ctx, pub)
}

// pushTasmotaCommand sends a single command to a Tasmota device's HTTP API.
func pushTasmotaCommand(host, cmd string) error {
	endpoint := fmt.Sprintf("http://%s/cm?cmnd=%s", host, url.QueryEscape(cmd))
	resp, err := http.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// waitForMQTTOnline creates a short-lived MQTT connection, subscribes to the
// LWT topic, and returns true if an "Online" message is received within the timeout.
func waitForMQTTOnline(ctx context.Context, broker *url.URL, username string, password []byte, lwtTopic string, timeout time.Duration) bool {
	done := make(chan struct{}, 1)

	cfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{broker},
		KeepAlive:         30,
		ConnectRetryDelay: 500 * time.Millisecond,
		ClientConfig: pahopkg.ClientConfig{
			ClientID: "seed-wait-" + uuid.New().String()[:8],
		},
		ConnectUsername: username,
		ConnectPassword: password,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *pahopkg.Connack) {
			cm.AddOnPublishReceived(func(pr autopaho.PublishReceived) (bool, error) {
				if strings.TrimSpace(string(pr.Packet.Payload)) == "Online" {
					select {
					case done <- struct{}{}:
					default:
					}
				}
				return true, nil
			})
			_, _ = cm.Subscribe(ctx, &pahopkg.Subscribe{
				Subscriptions: []pahopkg.SubscribeOptions{{Topic: lwtTopic, QoS: 1}},
			})
		},
	}

	watchCM, err := autopaho.NewConnection(ctx, cfg)
	if err != nil {
		slog.Warn("waitForMQTTOnline: failed to create connection", "err", err)
		return false
	}

	awaitCtx, cancel := context.WithTimeout(ctx, timeout/2)
	defer cancel()
	if err := watchCM.AwaitConnection(awaitCtx); err != nil {
		slog.Warn("waitForMQTTOnline: connection timed out", "err", err)
		return false
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
