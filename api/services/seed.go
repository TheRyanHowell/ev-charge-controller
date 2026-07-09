package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/eclipse/paho.golang/autopaho"
	pahopkg "github.com/eclipse/paho.golang/paho"
	"github.com/google/uuid"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/mqtt"
)

// SeedService handles database seeding and resetting.
// It creates a consistent state: 1 user, 2 plugs, 3 vehicles, 1 schedule,
// and ~500+ historical charge sessions with power readings and SOC snapshots.
type SeedService struct {
	mu              sync.Mutex
	db              *sql.DB
	tasmotaURLs     []string
	mqttProvisioned bool
}

// vehicleSpec holds model-specific charging parameters.
type vehicleSpec struct {
	capacityKwh    float64
	chargerOutputW float64
	efficiency     float64
	time0to100Min  int
}

var seedSpecs = map[string]vehicleSpec{
	"rm1":      {2.026, 600, 0.8, 250},
	"rm1_dual": {4.052, 600, 0.8, 250},
	"rm1s":     {5.46, 1200, 0.8, 360},
	"rm2":      {5.46, 1200, 0.8, 360},
}

// seedSession describes a charge session to be inserted.
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

const (
	seedEmail    = "test@example.com"
	seedPassword = "password123"
	// Deterministic IDs so data is reproducible across resets.
	// This is critical for E2E tests which rely on stable IDs.
	seedUserID     = "00000000-0000-0000-0000-000000000001"
	seedPlugID1    = "00000000-0000-0000-0000-000000000010" // Garage Plug
	seedPlugID2    = "00000000-0000-0000-0000-000000000011" // Driveway Plug
	seedPlugID3    = "00000000-0000-0000-0000-000000000012" // 12V Maintenance Plug (My RM1)
	seedVehicleID1 = "00000000-0000-0000-0000-000000000020" // My RM1
	seedVehicleID2 = "00000000-0000-0000-0000-000000000021" // My RM1S
	seedVehicleID3 = "00000000-0000-0000-0000-000000000022" // My RM2
	seedScheduleID  = "00000000-0000-0000-0000-000000000030" // Daily schedule (Garage Plug / My RM1)
	seedScheduleID2 = "00000000-0000-0000-0000-000000000031" // Carbon-aware schedule (Driveway Plug / My RM1S)
	// Deterministic MQTT namespaces and topics for plugs.
	seedPlugNamespace1 = "ev-garage"
	seedPlugTopic1     = "garage-plug"
	seedPlugNamespace2 = "ev-driveway"
	seedPlugTopic2     = "driveway-plug"
	seedPlugNamespace3 = "ev-12v-rm1"
	seedPlugTopic3     = "12v-rm1-plug"
	// Tariff: typical UK EV rate with an overnight off-peak window.
	seedTariffBaseRatePence = 24.83 // p/kWh peak rate
	seedTariffOffPeakRate   = 7.50  // p/kWh overnight cheap rate
	seedTariffOffPeakStart  = "00:30"
	seedTariffOffPeakEnd    = "05:30"
)

var seedRng = rand.New(rand.NewSource(42))

// plugOnlineTimeout bounds how long waitForPlugsOnline polls for mock-tasmota
// LWT confirmation. A package-level var (not a const) so tests can shrink it.
var plugOnlineTimeout = 30 * time.Second

// NewSeedService creates a new seed service.
// tasmotaURLs are the HTTP addresses of mock-tasmota instances (e.g., ["http://mock-tasmota:8081"]).
func NewSeedService(db *sql.DB, tasmotaURLs []string) *SeedService {
	return &SeedService{
		db:          db,
		tasmotaURLs: tasmotaURLs,
	}
}

// Reset clears all data and re-seeds the database to a known consistent state.
// It also resets mock-tasmota instances if URLs are configured.
// Uses a mutex to prevent concurrent resets (which can cause UNIQUE constraint violations).
func (s *SeedService) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.clearData(); err != nil {
		return fmt.Errorf("clear data: %w", err)
	}

	if err := s.seedUser(); err != nil {
		return fmt.Errorf("seed user: %w", err)
	}

	if err := s.seedTariff(); err != nil {
		return fmt.Errorf("seed tariff: %w", err)
	}

	plugIDs, vehicleIDs, err := s.seedPlugsAndVehicles()
	if err != nil {
		return fmt.Errorf("seed plugs and vehicles: %w", err)
	}

	userID, err := s.queryUserID()
	if err != nil {
		return fmt.Errorf("query user: %w", err)
	}

	if err := s.seedSchedule(plugIDs[0], plugIDs[1], userID); err != nil {
		return fmt.Errorf("seed schedule: %w", err)
	}

	vidToPlugID := map[string]string{
		vehicleIDs[0]: plugIDs[0],
		vehicleIDs[1]: plugIDs[1],
		vehicleIDs[2]: plugIDs[0],
	}
	sessions := s.generateSessions(vehicleIDs, vidToPlugID)

	if err := s.insertSessions(sessions, vehicleIDs, userID); err != nil {
		return fmt.Errorf("insert sessions: %w", err)
	}

	// Reset mock-tasmota instances (best-effort, non-fatal)
	s.resetMockTasmota(plugIDs)

	slog.Info("Database reset complete", "vehicles", len(vehicleIDs), "plugs", len(plugIDs))
	return nil
}

// clearData deletes all domain rows in FK order.
//
// Users (and therefore refresh_tokens, which FK-cascade from users) are
// intentionally preserved so that auth tokens issued before a reset remain
// valid afterwards. This keeps E2E sessions authenticated across the per-test
// resets that drive the stateful suite. seedUser re-creates the deterministic
// user idempotently for a fresh database.
func (s *SeedService) clearData() error {
	queries := []string{
		"DELETE FROM power_readings",
		"DELETE FROM soc_snapshots",
		"DELETE FROM charge_sessions",
		"DELETE FROM schedules",
		"DELETE FROM plugs",
		"DELETE FROM vehicles",
		"DELETE FROM tariff_off_peak_windows",
		"DELETE FROM tariff_settings",
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q, err)
		}
	}
	slog.Info("Cleared database for reset")
	return nil
}

// seedUser creates the test user with a deterministic ID so JWT tokens
// remain valid across resets (important for E2E test isolation).
func (s *SeedService) seedUser() error {
	hash, err := argon2id.CreateHash(seedPassword, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = s.db.Exec(
		"INSERT OR IGNORE INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)",
		seedUserID, seedEmail, hash,
	)
	if err != nil {
		return err
	}
	slog.Info("Seeded user", "email", seedEmail, "id", seedUserID)
	return nil
}

// seedPlugsAndVehicles creates 2 plugs and 3 vehicles with deterministic IDs.
// Returns plugIDs and vehicleIDs.
func (s *SeedService) seedPlugsAndVehicles() ([]string, []string, error) {
	userID, err := s.queryUserID()
	if err != nil {
		return nil, nil, err
	}

	// Create 3 vehicles from catalog models with deterministic IDs
	modelIDs := []string{"rm1", "rm1s", "rm2"}
	vehicleNames := []string{"My RM1", "My RM1S", "My RM2"}
	vehicleIDs := []string{seedVehicleID1, seedVehicleID2, seedVehicleID3}
	for i, mid := range modelIDs {
		if _, err := s.db.Exec(
			"INSERT INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, 20.0, 80.0, CURRENT_TIMESTAMP)",
			vehicleIDs[i], userID, mid, vehicleNames[i],
		); err != nil {
			return nil, nil, fmt.Errorf("insert vehicle %d: %w", i, err)
		}
	}

	// Create plugs with deterministic IDs.
	// Plug 3 is a 12V maintenance charger on My RM1 (vehicleID1).
	type plugSeed struct {
		id        string
		name      string
		namespace string
		topic     string
		plugType  string
		vehicleID string
	}
	plugSeeds := []plugSeed{
		{seedPlugID1, "Garage Plug", seedPlugNamespace1, seedPlugTopic1, "charging", vehicleIDs[0]},
		{seedPlugID2, "Driveway Plug", seedPlugNamespace2, seedPlugTopic2, "charging", vehicleIDs[1]},
		{seedPlugID3, "My RM1 12V", seedPlugNamespace3, seedPlugTopic3, "maintenance", vehicleIDs[0]},
	}
	plugIDs := make([]string, len(plugSeeds))
	for i, p := range plugSeeds {
		plugIDs[i] = p.id
		if _, err := s.db.Exec(
			"INSERT INTO plugs (id, user_id, name, namespace, mqtt_topic, type, created_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
			p.id, userID, p.name, p.namespace, p.topic, p.plugType,
		); err != nil {
			return nil, nil, fmt.Errorf("insert plug %d: %w", i, err)
		}
		if _, err := s.db.Exec(
			"UPDATE plugs SET vehicle_id = ? WHERE id = ?",
			p.vehicleID, p.id,
		); err != nil {
			return nil, nil, fmt.Errorf("update plug vehicle %d: %w", i, err)
		}
	}

	slog.Info("Seeded vehicles and plugs", "vehicles", len(vehicleIDs), "plugs", len(plugIDs))
	return plugIDs, vehicleIDs, nil
}

// seedSchedule creates a daily schedule for the Garage Plug and a carbon-aware
// schedule for the Driveway Plug (My RM1S).
func (s *SeedService) seedSchedule(garagePlug, drivewayPlug, userID string) error {
	_, err := s.db.Exec(
		"INSERT INTO schedules (id, plug_id, user_id, time, enabled) VALUES (?, ?, ?, '06:00', 1)",
		seedScheduleID, garagePlug, userID,
	)
	if err != nil {
		return fmt.Errorf("insert daily schedule: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO schedules (id, plug_id, user_id, type, time, window_start, window_end, enabled)
		 VALUES (?, ?, ?, 'carbon_aware', '07:00', '22:00', '07:00', 1)`,
		seedScheduleID2, drivewayPlug, userID,
	)
	if err != nil {
		return fmt.Errorf("insert carbon-aware schedule: %w", err)
	}
	slog.Info("Seeded schedules", "garagePlug", garagePlug, "drivewayPlug", drivewayPlug)
	return nil
}

// queryUserID returns the user ID for the seed email.
func (s *SeedService) queryUserID() (string, error) {
	var userID string
	err := s.db.QueryRow("SELECT id FROM users WHERE email = ?", seedEmail).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("query user: %w", err)
	}
	return userID, nil
}

// seedTariff creates a realistic UK EV tariff for the seed user.
func (s *SeedService) seedTariff() error {
	userID, err := s.queryUserID()
	if err != nil {
		return err
	}
	tariffID := "00000000-0000-0000-0000-000000000040"
	windowID := "00000000-0000-0000-0000-000000000041"
	_, err = s.db.Exec(
		"INSERT INTO tariff_settings (id, user_id, base_rate_pence) VALUES (?, ?, ?)",
		tariffID, userID, seedTariffBaseRatePence,
	)
	if err != nil {
		return fmt.Errorf("insert tariff: %w", err)
	}
	_, err = s.db.Exec(
		"INSERT INTO tariff_off_peak_windows (id, user_id, position, start_hhmm, end_hhmm, rate_pence) VALUES (?, ?, 0, ?, ?, ?)",
		windowID, userID, seedTariffOffPeakStart, seedTariffOffPeakEnd, seedTariffOffPeakRate,
	)
	if err != nil {
		return fmt.Errorf("insert off-peak window: %w", err)
	}
	slog.Info("Seeded tariff", "baseRate", seedTariffBaseRatePence, "offPeakRate", seedTariffOffPeakRate)
	return nil
}

// generateSessions creates historical charge sessions.
func (s *SeedService) generateSessions(vehicleIDs []string, vidToPlugID map[string]string) []seedSession {
	var sessions []seedSession
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	addCompleted := func(vehicleID string, date time.Time, count int) {
		usedSlots := make(map[int]bool)
		for range count {
			hour := seedRng.Intn(22)
			minute := seedRng.Intn(60)
			slot := hour*60 + minute
			for usedSlots[slot] {
				hour = (hour + 1) % 23
				minute = seedRng.Intn(60)
				slot = hour*60 + minute
			}
			usedSlots[slot] = true

			startPct := 5.0 + seedRng.Float64()*30.0
			endPct := 60.0 + seedRng.Float64()*36.0
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

	// Always emit at least 2 completed sessions per vehicle on today's date so
	// the history page's default "today" filter is never empty regardless of
	// the random seed, eliminating the midnight-rollover class of flake.
	for _, vid := range vehicleIDs[:2] {
		addCompleted(vid, today, 2)
	}

	// Generate ~200 sessions per vehicle over 180 days (only vehicles with plugs).
	// Start at daysAgo=1 to avoid duplicating today's sessions above.
	for daysAgo := 1; daysAgo < 180; daysAgo++ {
		date := today.AddDate(0, 0, -daysAgo)
		sessionsPerVehicle := seedRng.Intn(3) // 0-2 per vehicle per day

		for _, vid := range vehicleIDs[:2] {
			addCompleted(vid, date, sessionsPerVehicle)
		}
	}

	// Add some cancelled sessions
	for _, vid := range vehicleIDs[:2] {
		startPct := 15.0 + seedRng.Float64()*15.0
		delta := 5.0 + seedRng.Float64()*20.0
		sessions = append(sessions, seedSession{
			vehicleID:   vid,
			plugID:      vidToPlugID[vid],
			date:        today.AddDate(0, 0, -seedRng.Intn(5)),
			hour:        seedRng.Intn(22),
			minute:      seedRng.Intn(60),
			startPct:    startPct,
			endPct:      startPct + delta,
			status:      "cancelled",
			hasTotalKwh: true,
		})
	}

	// Some sessions with NULL start_total_kwh (simulate old data)
	for i := range sessions {
		if sessions[i].status == "completed" && seedRng.Intn(10) == 0 {
			sessions[i].hasTotalKwh = false
		}
	}

	return sessions
}

// insertSessions inserts charge sessions with power readings and SOC snapshots.
func (s *SeedService) insertSessions(sessions []seedSession, vehicleIDs []string, userID string) error {
	vidToModel := map[string]string{
		vehicleIDs[0]: "rm1",
		vehicleIDs[1]: "rm1s",
		vehicleIDs[2]: "rm2",
	}

	completedIDs := make([]string, 0)

	// Track per-vehicle aggregates
	type vehicleAgg struct {
		totalSessions        int
		totalBatteryKwh      float64
		totalWallKwh         float64
		totalCo2Grams        float64
		totalCostPence       float64
		lastSessionAt        *time.Time
		minSessionBatteryKwh float64
		maxSessionBatteryKwh float64
	}
	aggs := make(map[string]*vehicleAgg)
	for _, vid := range vehicleIDs {
		aggs[vid] = &vehicleAgg{}
	}

	for i, sess := range sessions {
		modelID := vidToModel[sess.vehicleID]
		spec := seedSpecs[modelID]
		id := fmt.Sprintf("seed-%04d", i)

		startKwh := spec.capacityKwh * sess.startPct / 100

		var targetPct float64
		switch sess.status {
		case "completed":
			targetPct = sess.endPct
		case "cancelled":
			targetPct = sess.endPct + 30 + seedRng.Float64()*20
			if targetPct > 98 {
				targetPct = 98
			}
		case "active":
			targetPct = 80 + seedRng.Float64()*15
		}
		targetKwh := spec.capacityKwh * targetPct / 100

		startTime := time.Date(sess.date.Year(), sess.date.Month(), sess.date.Day(), sess.hour, sess.minute, 0, 0, time.UTC)

		var endTime *time.Time
		var endKwh *float64
		var endPct *float64
		var totalKwh *float64

		var batteryKwh, wallKwh, avgCarbonIntensity, co2Grams *float64
		var costPence, offPeakKwh *float64

		if sess.status != "active" {
			durationMin := (sess.endPct - sess.startPct) / 100 * float64(spec.time0to100Min)
			et := startTime.Add(time.Duration(durationMin * float64(time.Minute)))
			endTime = &et

			ek := spec.capacityKwh * sess.endPct / 100
			endKwh = &ek

			ep := sess.endPct
			endPct = &ep
		}

		if sess.hasTotalKwh {
			tk := startKwh
			totalKwh = &tk
		}

		// Compute stats for completed sessions at insert time
		if sess.status == "completed" && endKwh != nil {
			bkwh := *endKwh - startKwh // battery-side energy
			if bkwh > 0 {
				wkwh := bkwh / spec.efficiency // wall-side energy
				// Carbon intensity varies by hour: lower overnight, higher at peak
				hour := float64(sess.hour)
				baseCarbon := 200.0 + 150.0*(1.0-math.Cos((hour-14)*math.Pi/12.0))
				ci := baseCarbon + seedRngFloat(-30, 30)
				if ci < 100 {
					ci = 100
				}
				co2 := wkwh * ci

				batteryKwh = &bkwh
				wallKwh = &wkwh
				avgCarbonIntensity = &ci
				co2Grams = &co2

				// Cost: apply tariff rate based on session start time.
				seedTariff := models.TariffSettings{
					BaseRatePence: seedTariffBaseRatePence,
					OffPeakWindows: []models.OffPeakWindow{
						{Start: seedTariffOffPeakStart, End: seedTariffOffPeakEnd, RatePence: seedTariffOffPeakRate},
					},
				}
				rate, isOffPeak := applicableRatePence(seedTariff, startTime)
				cp := wkwh * rate
				costPence = &cp
				okwh := 0.0
				if isOffPeak {
					okwh = wkwh
				}
				offPeakKwh = &okwh

				// Accumulate vehicle aggregates
				a := aggs[sess.vehicleID]
				a.totalSessions++
				a.totalBatteryKwh += bkwh
				a.totalWallKwh += wkwh
				a.totalCo2Grams += co2
				a.totalCostPence += cp
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

		_, err := s.db.Exec(`
			INSERT INTO charge_sessions (
				id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
				start_percent, end_percent, target_kwh, target_percent,
				status, start_total_kwh, user_id, plug_id,
				battery_kwh, wall_kwh, avg_carbon_intensity, co2_grams,
				cost_pence, off_peak_kwh
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, sess.vehicleID, startTime,
			endTime, startKwh, endKwh,
			sess.startPct, endPct, targetKwh, targetPct,
			sess.status, totalKwh, userID, sess.plugID,
			batteryKwh, wallKwh, avgCarbonIntensity, co2Grams,
			costPence, offPeakKwh,
		)
		if err != nil {
			slog.Warn("failed to insert session", "id", id, "err", err)
			continue
		}

		if sess.status == "completed" {
			completedIDs = append(completedIDs, id)
		}
	}

	// Insert power readings & SOC snapshots for completed sessions
	for _, id := range completedIDs {
		s.insertPowerReadings(id, vidToModel)
		s.insertSOCSnapshots(id)
	}

	// Update vehicle lifetime aggregates
	for vid, a := range aggs {
		var lastSessionAt any
		if a.lastSessionAt != nil {
			lastSessionAt = a.lastSessionAt.Format(time.RFC3339)
		}
		_, err := s.db.Exec(`
			UPDATE vehicles SET
				total_sessions = ?,
				total_battery_kwh = ?,
				total_wall_kwh = ?,
				total_co2_grams = ?,
				total_cost_pence = ?,
				last_session_at = ?,
				min_session_battery_kwh = ?,
				max_session_battery_kwh = ?
			WHERE id = ?`,
			a.totalSessions, a.totalBatteryKwh, a.totalWallKwh,
			a.totalCo2Grams, a.totalCostPence, lastSessionAt,
			a.minSessionBatteryKwh, a.maxSessionBatteryKwh, vid,
		)
		if err != nil {
			slog.Warn("failed to update vehicle aggregates", "vehicleID", vid, "err", err)
		}
	}

	return nil
}

// insertPowerReadings generates power readings for a completed session.
func (s *SeedService) insertPowerReadings(sessionID string, vidToModel map[string]string) {
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
	err := s.db.QueryRow(`
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
	spec := seedSpecs[modelID]
	startTime := st
	endTime := et.Time

	numReadings := 5
	for j := 0; j <= numReadings; j++ {
		fraction := float64(j) / float64(numReadings)
		ts := startTime.Add(time.Duration(float64(endTime.Sub(startTime)) * fraction))

		voltage := seedRngFloat(228, 234)
		batteryPower := spec.chargerOutputW*spec.efficiency + (seedRng.Float64()*20-10)*spec.efficiency
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
		carbonIntensity := baseCarbon + seedRngFloat(-30, 30)
		if carbonIntensity < 100 {
			carbonIntensity = 100
		}
		_, err := s.db.Exec(`
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

// insertSOCSnapshots generates SOC snapshots for a completed session.
func (s *SeedService) insertSOCSnapshots(sessionID string) {
	var (
		vid   string
		st    time.Time
		et    sql.NullTime
		skwh  float64
		ekwh  sql.NullFloat64
		stp   float64
		etp   sql.NullFloat64
	)
	err := s.db.QueryRow(`
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
		_, err := s.db.Exec(`
			INSERT INTO soc_snapshots (id, session_id, timestamp, soc_percent)
			VALUES (?, ?, ?, ?)`,
			snID, sessionID, ts, soc,
		)
		if err != nil {
			slog.Warn("failed to insert soc snapshot", "snID", snID, "err", err)
		}
	}
}

// resetMockTasmota provisions each mock-tasmota instance's MQTT credentials
// on the first call only, then resets each instance's power/energy state on
// every call. Best-effort: failures are logged but not fatal.
//
// Namespace and topic are deterministic and never change across resets (see
// seedPlugsAndVehicles), so there is nothing for a repeat dynsec
// provision+reconnect cycle to actually fix - it only forces every plug's
// MQTT connection to drop and re-establish. Doing that before every single
// e2e test (resetAllState runs per-test, not per-file) kept 1-3 plugs
// mid-reconnect at any given moment; the always-on carbon-aware schedule
// fixture (seedScheduleID2) then intermittently raced that reconnect window
// and timed out waiting for power confirmation. Provisioning once per
// SeedService lifetime (i.e. once per API process, which is once per e2e
// worker) keeps connections stable for the rest of the run.
func (s *SeedService) resetMockTasmota(plugIDs []string) {
	if len(s.tasmotaURLs) == 0 {
		slog.Debug("No mock-tasmota URLs configured, skipping tasmota reset")
		return
	}

	if !s.mqttProvisioned {
		s.provisionMockTasmotaMQTT(plugIDs)
		s.mqttProvisioned = true
	}

	for i, plugID := range plugIDs {
		if i >= len(s.tasmotaURLs) {
			break
		}
		resetURL := fmt.Sprintf("%s/reset", s.tasmotaURLs[i])
		if _, err := http.Post(resetURL, "text/plain", nil); err != nil {
			slog.Warn("Failed to reset mock-tasmota energy state", "plugID", plugID, "err", err)
		} else {
			slog.Info("Reset mock-tasmota energy state", "plugID", plugID)
		}
	}
}

// provisionMockTasmotaMQTT provisions dynsec credentials for each plug and
// pushes MQTT config to each mock-tasmota instance, then waits for all
// plugs to come online. Called at most once per SeedService lifetime by
// resetMockTasmota.
func (s *SeedService) provisionMockTasmotaMQTT(plugIDs []string) {
	slog.Info("Provisioning mock-tasmota MQTT credentials", "plugCount", len(plugIDs))

	mqttBroker := getEnvOr("MQTT_BROKER_URL", "mqtt://mosquitto:1883")
	mqttUser := getEnvOr("MQTT_USERNAME", "")
	mqttPass := getEnvOr("MQTT_PASSWORD", "")

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

	// Connect to mosquitto for dynsec provisioning.
	dynsec, _, err := connectDynsec(mqttBroker, mqttUser, mqttPass)
	if err != nil {
		slog.Warn("Cannot connect to MQTT broker, skipping dynsec provisioning (plugs will work without MQTT)", "err", err)
		dynsec = nil
	} else {
		slog.Info("Connected to MQTT broker for dynsec provisioning")
	}

	// Read all plug namespaces from DB for cleanup
	type plugInfo struct {
		id        string
		namespace string
		topic     string
	}
	var plugs []plugInfo
	for _, plugID := range plugIDs {
		var namespace, mqttTopic string
		if err := s.db.QueryRow("SELECT namespace, mqtt_topic FROM plugs WHERE id = ?", plugID).Scan(&namespace, &mqttTopic); err != nil {
			slog.Warn("Failed to read plug for mock-tasmota reset", "plugID", plugID, "err", err)
			continue
		}
		plugs = append(plugs, plugInfo{id: plugID, namespace: namespace, topic: mqttTopic})
	}

	// Clean up any stale credentials from a previous process (e.g. a crashed
	// API container reusing the same mosquitto instance) before provisioning.
	if dynsec != nil {
		for _, p := range plugs {
			slog.Info("Cleaning up old dynsec credentials", "plugID", p.id, "namespace", p.namespace)
			if err := dynsec.RemovePlug(context.Background(), p.namespace); err != nil {
				slog.Warn("Failed to remove old dynsec credentials (may not exist yet)", "plugID", p.id, "namespace", p.namespace, "err", err)
			}
		}
		// Brief pause to let broker process deletions
		time.Sleep(500 * time.Millisecond)
	}

	for i, p := range plugs {
		if i >= len(s.tasmotaURLs) {
			slog.Warn("Skipping plug - no matching mock-tasmota URL", "plugID", p.id, "index", i, "tasmotaURLCount", len(s.tasmotaURLs))
			break
		}

		tasmotaURL := s.tasmotaURLs[i]
		tasmotaHost := strings.TrimPrefix(tasmotaURL, "http://")
		tasmotaHost = strings.TrimSuffix(tasmotaHost, "/")

		slog.Info("Provisioning mock-tasmota", "plugID", p.id, "url", tasmotaURL, "namespace", p.namespace, "topic", p.topic)

		// Generate password for this plug
		rawPw, err := GeneratePassword()
		if err != nil {
			slog.Warn("Failed to generate password for mock-tasmota reset", "plugID", p.id, "err", err)
			continue
		}

		// CRITICAL: Provision dynsec credentials BEFORE pushing config to mock-tasmota.
		// If we push config first, mock-tasmota will try to reconnect with the new password
		// but the broker won't have the credentials yet, causing connection failures.
		if dynsec != nil {
			slog.Info("Provisioning dynsec credentials", "plugID", p.id, "namespace", p.namespace)
			if err := dynsec.ProvisionPlug(context.Background(), p.namespace, rawPw); err != nil {
				slog.Warn("Failed to provision dynsec for plug", "plugID", p.id, "err", err)
			} else {
				slog.Info("Provisioned dynsec credentials", "plugID", p.id, "namespace", p.namespace)
			}
		}

		// Now push config to mock-tasmota (it will reconnect with the new credentials)
		consoleCmd := BuildConsoleCommands(mqttExtHost, mqttExtPort, p.namespace, p.topic, rawPw)
		cmds := ParseBacklogCommands(consoleCmd)
		slog.Info("Pushing MQTT config to mock-tasmota", "plugID", p.id, "commandCount", len(cmds))

		// Push commands to mock-tasmota
		for ci := 0; ci < len(cmds); ci++ {
			slog.Info("Pushing command", "plugID", p.id, "index", ci, "total", len(cmds), "cmd", cmds[ci])
			if err := pushTasmotaCommand(tasmotaHost, cmds[ci]); err != nil {
				slog.Warn("Failed to push command to mock-tasmota", "plugID", p.id, "cmd", cmds[ci], "err", err)
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Wait for all plugs to come online via MQTT LWT.
	// Mock-tasmota needs time to reconnect to MQTT after receiving new config.
	slog.Info("Waiting for plugs to come online", "plugIDs", plugIDs)
	s.waitForPlugsOnline(plugIDs)
	slog.Info("Mock-tasmota MQTT provisioning complete")
}

// waitForPlugsOnline polls the database until all plugs are online or timeout.
func (s *SeedService) waitForPlugsOnline(plugIDs []string) {
	const pollInterval = 500 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), plugOnlineTimeout)
	defer cancel()

	slog.Info("Waiting for plugs to come online", "plugIDs", plugIDs, "timeout", plugOnlineTimeout)

	for {
		select {
		case <-ctx.Done():
			slog.Warn("Timed out waiting for plugs to come online", "plugIDs", plugIDs)
			// Log current plug status for debugging
			for _, plugID := range plugIDs {
				var online bool
				_ = s.db.QueryRow(`SELECT online FROM plugs WHERE id = ?`, plugID).Scan(&online)
				slog.Info("Plug status at timeout", "plugID", plugID, "online", online)
			}
			return
		default:
		}

		allOnline := true
		for _, plugID := range plugIDs {
			var online bool
			err := s.db.QueryRowContext(ctx, `SELECT online FROM plugs WHERE id = ?`, plugID).Scan(&online)
			if err != nil {
				slog.Warn("Failed to query plug online status", "plugID", plugID, "err", err)
				allOnline = false
				break
			}
			if !online {
				slog.Debug("Plug not yet online", "plugID", plugID)
				allOnline = false
				break
			}
		}

		if allOnline {
			slog.Info("All plugs are online", "plugIDs", plugIDs)
			return
		}

		time.Sleep(pollInterval)
	}
}

// connectDynsec establishes an MQTT connection and returns a DynsecManager.
func connectDynsec(brokerURL, username, password string) (*mqtt.DynsecManager, *autopaho.ConnectionManager, error) {
	bu, err := url.Parse(brokerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse broker URL: %w", err)
	}

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

	awaitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cm.AwaitConnection(awaitCtx); err != nil {
		return nil, nil, fmt.Errorf("await MQTT connection: %w", err)
	}

	dynsec := mqtt.NewDynsecManager(&seedPahoPublisher{cm: cm})
	return dynsec, cm, nil
}

// seedPahoPublisher adapts autopaho.ConnectionManager to the pahoPublisher interface.
type seedPahoPublisher struct {
	cm *autopaho.ConnectionManager
}

func (p *seedPahoPublisher) Publish(ctx context.Context, pub *pahopkg.Publish) (*pahopkg.PublishResponse, error) {
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

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func seedRngFloat(min, max float64) float64 {
	return min + seedRng.Float64()*(max-min)
}
