package testdb

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrSessionWrongState = errors.New("session is not in the expected state for this operation")

// DefaultIDs are the canonical test identifiers used across all test packages.
const (
	DefaultUserID    = "test-user"
	DefaultPlugID    = "test-plug"
	DefaultVehicleID = "rm1"
)

// ChargeSessionOpts configures a test charge session insert.
// VehicleID, UserID, and PlugID are required.
// Zero values use sensible defaults: status defaults to "pending",
// CreatedAt defaults to now, optional fields are omitted when nil.
type ChargeSessionOpts struct {
	ID             string
	VehicleID      string
	UserID         string
	PlugID         string
	Status         string
	CreatedAt      time.Time
	StartedAt      *time.Time
	EndedAt        *time.Time
	StartKwh       float64
	EndKwh         *float64
	TargetKwh      float64
	StartPct       float64
	EndPct         *float64
	TargetPct      float64
	StartTotalKwh  *float64
	LastBlendedKwh *float64
	BatteryKwh     *float64
	WallKwh        *float64
	Co2Grams       *float64
}

// PowerReadingOpts configures a test power reading insert.
type PowerReadingOpts struct {
	ID                string
	SessionID         string
	Timestamp         time.Time
	Voltage           float64
	Current           float64
	Power             float64
	EnergyKwh         float64
	CarbonIntensity   *float64
}

// SOCSnapshotOpts configures a test SOC snapshot insert.
type SOCSnapshotOpts struct {
	ID         string
	SessionID  string
	Timestamp  time.Time
	SocPercent float64
}

// ScheduleOpts configures a test schedule insert.
// PlugID and UserID are required.
type ScheduleOpts struct {
	ID       string
	PlugID   string
	UserID   string
	Time     string
	ReadyBy  string
	TwoStage bool
	Enabled  bool
}

// RefreshTokenOpts configures a test refresh token insert.
type RefreshTokenOpts struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// InsertUser inserts a test user (idempotent via INSERT OR IGNORE).
func InsertUser(db *sql.DB, id, email, passwordHash string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		id, email, passwordHash)
	return err
}

// InsertPlug inserts a test plug (idempotent via INSERT OR IGNORE).
func InsertPlug(db *sql.DB, id, userID, name, namespace, mqttTopic string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO plugs (id, user_id, name, namespace, mqtt_topic, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		id, userID, name, namespace, mqttTopic)
	return err
}

// InsertVehicle inserts a test vehicle instance (idempotent via INSERT OR IGNORE).
// The modelID must already exist in vehicle_models (seeded by SetupTestDB(true)).
func InsertVehicle(db *sql.DB, id, userID, modelID, name string, currentPct, targetPct float64) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		id, userID, modelID, name, currentPct, targetPct)
	return err
}

// InsertVehicleWithModel creates a custom vehicle_model + vehicle instance pair.
// Both inserts are idempotent via INSERT OR IGNORE.
func InsertVehicleWithModel(db *sql.DB, id, userID, name string, capacityKwh, chargerOutputW, efficiency, rangeMinMi, rangeMaxMi float64) error {
	if _, err := db.Exec(`INSERT OR IGNORE INTO vehicle_models (id, name, capacity_kwh, charger_output_w, charging_efficiency, range_min_mi, range_max_mi) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, capacityKwh, chargerOutputW, efficiency, rangeMinMi, rangeMaxMi); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES (?, ?, ?, ?, 20, 80, CURRENT_TIMESTAMP)`,
		id, userID, id, name); err != nil {
		return err
	}
	return nil
}

// InsertChargeSession inserts a test charge session with the given options.
// Uses INSERT OR IGNORE for idempotency.
// VehicleID, UserID, and PlugID are required and must exist in the database.
func InsertChargeSession(db *sql.DB, opts *ChargeSessionOpts) error {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}
	if opts.Status == "" {
		opts.Status = "pending"
	}
	if opts.CreatedAt.IsZero() {
		opts.CreatedAt = time.Now()
	}

	// Build column list and values dynamically based on what's set.
	cols := []string{"id", "vehicle_id", "user_id", "plug_id", "created_at", "start_kwh", "target_kwh", "start_percent", "target_percent", "status"}
	vals := []any{opts.ID, opts.VehicleID, opts.UserID, opts.PlugID, opts.CreatedAt.Format(time.RFC3339), opts.StartKwh, opts.TargetKwh, opts.StartPct, opts.TargetPct, opts.Status}

	appendNullable := func(col string, val any) {
		if val != nil {
			cols = append(cols, col)
			vals = append(vals, val)
		}
	}

	appendNullable("started_at", formatTime(opts.StartedAt))
	appendNullable("ended_at", formatTime(opts.EndedAt))
	appendNullable("start_total_kwh", opts.StartTotalKwh)
	appendNullable("last_blended_kwh", opts.LastBlendedKwh)
	appendNullable("end_kwh", opts.EndKwh)
	appendNullable("end_percent", opts.EndPct)
	appendNullable("battery_kwh", opts.BatteryKwh)
	appendNullable("wall_kwh", opts.WallKwh)
	appendNullable("co2_grams", opts.Co2Grams)

	query := `INSERT OR IGNORE INTO charge_sessions (` + joinCols(cols) + `) VALUES (` + placeholders(len(cols)) + `)`
	_, err := db.Exec(query, vals...)
	return err
}

// InsertPowerReading inserts a test power reading.
func InsertPowerReading(db *sql.DB, opts *PowerReadingOpts) error {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}
	_, err := db.Exec(`INSERT INTO power_readings (id, session_id, timestamp, voltage, current, power, energy_kwh, carbon_intensity_g_co2_per_kwh) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		opts.ID, opts.SessionID, opts.Timestamp.Format(time.RFC3339), opts.Voltage, opts.Current, opts.Power, opts.EnergyKwh, opts.CarbonIntensity)
	return err
}

// InsertSOCSnapshot inserts a test SOC snapshot.
func InsertSOCSnapshot(db *sql.DB, opts *SOCSnapshotOpts) error {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}
	_, err := db.Exec(`INSERT INTO soc_snapshots (id, session_id, timestamp, soc_percent) VALUES (?, ?, ?, ?)`,
		opts.ID, opts.SessionID, opts.Timestamp.Format(time.RFC3339), opts.SocPercent)
	return err
}

// InsertSchedule inserts a test schedule.
// Requires: user and plug to already exist in the database.
func InsertSchedule(db *sql.DB, opts *ScheduleOpts) error {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}
	enabled := 0
	if opts.Enabled {
		enabled = 1
	}
	twoStage := 0
	if opts.TwoStage {
		twoStage = 1
	}
	var readyBy any
	if opts.ReadyBy != "" {
		readyBy = opts.ReadyBy
	}
	_, err := db.Exec(`INSERT INTO schedules (id, plug_id, user_id, time, ready_by, two_stage, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		opts.ID, opts.PlugID, opts.UserID, opts.Time, readyBy, twoStage, enabled)
	return err
}

// InsertRefreshToken inserts a test refresh token.
func InsertRefreshToken(db *sql.DB, opts *RefreshTokenOpts) error {
	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}
	if opts.CreatedAt.IsZero() {
		opts.CreatedAt = time.Now()
	}
	_, err := db.Exec(`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		opts.ID, opts.UserID, opts.TokenHash, opts.ExpiresAt, opts.CreatedAt)
	return err
}

// ActivateSession transitions a pending session to active.
// Returns ErrSessionWrongState if no rows were affected (session not pending).
func ActivateSession(db *sql.DB, sessionID string) error {
	result, err := db.Exec(`UPDATE charge_sessions SET status = 'active', started_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'pending'`, sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSessionWrongState
	}
	return nil
}

// CompleteSession marks an active session as completed with final stats.
func CompleteSession(db *sql.DB, sessionID string, endedAt time.Time, endKwh, endPct, batteryKwh, wallKwh, co2Grams float64, avgCarbon *float64) error {
	_, err := db.Exec(`UPDATE charge_sessions SET ended_at = ?, end_kwh = ?, end_percent = ?, status = 'completed', battery_kwh = ?, wall_kwh = ?, avg_carbon_intensity = ?, co2_grams = ? WHERE id = ?`,
		endedAt.Format(time.RFC3339), endKwh, endPct, batteryKwh, wallKwh, avgCarbon, co2Grams, sessionID)
	return err
}

// CancelSession marks an active session as cancelled.
func CancelSession(db *sql.DB, sessionID string, endedAt time.Time) error {
	_, err := db.Exec(`UPDATE charge_sessions SET status = 'cancelled', ended_at = ? WHERE id = ?`,
		endedAt.Format(time.RFC3339), sessionID)
	return err
}

// CancelPendingSession atomically transitions a pending session to cancelled.
// Returns ErrSessionWrongState if no rows were affected (session not pending).
func CancelPendingSession(db *sql.DB, sessionID string, endedAt time.Time) error {
	result, err := db.Exec(`UPDATE charge_sessions SET status = 'cancelled', ended_at = ? WHERE id = ? AND status = 'pending'`,
		endedAt.Format(time.RFC3339), sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSessionWrongState
	}
	return nil
}

// BackdateSession sets the created_at of a session to a specific time.
func BackdateSession(db *sql.DB, sessionID string, createdAt time.Time) error {
	_, err := db.Exec(`UPDATE charge_sessions SET created_at = ? WHERE id = ?`,
		createdAt.Format(time.RFC3339), sessionID)
	return err
}

// SetVehicleEfficiency updates the charging_efficiency of a vehicle model.
func SetVehicleEfficiency(db *sql.DB, modelID string, efficiency float64) error {
	_, err := db.Exec(`UPDATE vehicle_models SET charging_efficiency = ? WHERE id = ?`, efficiency, modelID)
	return err
}

// SetVehicleCapacity updates the capacity_kwh of a vehicle model.
func SetVehicleCapacity(db *sql.DB, modelID string, capacityKwh float64) error {
	_, err := db.Exec(`UPDATE vehicle_models SET capacity_kwh = ? WHERE id = ?`, capacityKwh, modelID)
	return err
}

// SetChargerOutput updates the charger_output_w of a vehicle model.
func SetChargerOutput(db *sql.DB, modelID string, watts float64) error {
	_, err := db.Exec(`UPDATE vehicle_models SET charger_output_w = ? WHERE id = ?`, watts, modelID)
	return err
}

func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func joinCols(cols []string) string {
	result := ""
	for i, c := range cols {
		if i > 0 {
			result += ", "
		}
		result += c
	}
	return result
}

func placeholders(n int) string {
	result := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			result += ", "
		}
		result += "?"
	}
	return result
}
