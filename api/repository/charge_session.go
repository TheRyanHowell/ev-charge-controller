package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ev-charge-controller/api/models"

	"github.com/google/uuid"
)

var ErrSessionWrongState = errors.New("session is not in the expected state for this operation")

type ChargeSessionRepository struct {
	db *sql.DB
}

func NewChargeSessionRepository(db *sql.DB) *ChargeSessionRepository {
	return &ChargeSessionRepository{db: db}
}

func generateID() string {
	return uuid.New().String()
}

const chargeSessionColumns = "id, vehicle_id, created_at, ended_at, start_kwh, end_kwh, target_kwh, start_percent, end_percent, target_percent, status, start_total_kwh, started_at, last_blended_kwh, plug_id, battery_kwh, wall_kwh, avg_carbon_intensity, co2_grams, user_id, cost_pence, off_peak_kwh, hold_percent, ready_by_time"

func (r *ChargeSessionRepository) Create(ctx context.Context, session *models.ChargeSession) error {
	session.ID = generateID()

	query := `INSERT INTO charge_sessions (id, vehicle_id, created_at, start_kwh, target_kwh, start_percent, target_percent, status, start_total_kwh, user_id, plug_id, hold_percent, ready_by_time)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, session.ID, session.VehicleID, session.CreatedAt,
		session.StartKwh, session.TargetKwh, session.StartPercent, session.TargetPercent, session.Status, session.StartTotalKwh,
		toNullString(session.UserID), toNullString(session.PlugID),
		toNullFloat(session.HoldPercent), toNullString(session.ReadyByTime))

	return err
}

func (r *ChargeSessionRepository) FindByID(ctx context.Context, id string) (*models.ChargeSession, error) {
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE id = ?`
	return r.queryOneSession(ctx, query, id)
}

func (r *ChargeSessionRepository) GetActive(ctx context.Context) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE status IN (` + buildPlaceholders(len(models.ActiveSessionStatuses)) + `)` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append(toAnySlice(models.ActiveSessionStatuses), ua...)...)
}

func (r *ChargeSessionRepository) GetActiveByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE vehicle_id = ? AND status IN (` + buildPlaceholders(len(models.ActiveSessionStatuses)) + `)` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append(append([]any{vehicleID}, toAnySlice(models.ActiveSessionStatuses)...), ua...)...)
}

func (r *ChargeSessionRepository) GetPending(ctx context.Context) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE status = ?` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append([]any{models.SessionStatusPending}, ua...)...)
}

func (r *ChargeSessionRepository) GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE plug_id = ? AND status IN (` + buildPlaceholders(len(models.ActiveSessionStatuses)) + `)` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append(append([]any{plugID}, toAnySlice(models.ActiveSessionStatuses)...), ua...)...)
}

func (r *ChargeSessionRepository) GetPendingByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE plug_id = ? AND status = ?` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append([]any{plugID, models.SessionStatusPending}, ua...)...)
}

func (r *ChargeSessionRepository) GetLastCompletedByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE vehicle_id = ? AND status = ?` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append([]any{vehicleID, models.SessionStatusCompleted}, ua...)...)
}

func (r *ChargeSessionRepository) GetLastCompleted(ctx context.Context) (*models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions WHERE status = ?` + uc + ` ORDER BY created_at DESC LIMIT 1`
	return r.queryOneSession(ctx, query, append([]any{models.SessionStatusCompleted}, ua...)...)
}

func (r *ChargeSessionRepository) GetAll(ctx context.Context) ([]models.ChargeSession, error) {
	wc, wa := whereUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions` + wc + ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, wa...)
	if err != nil {
		return nil, err
	}
	return scanChargeSessionRows(rows)
}

func (r *ChargeSessionRepository) GetAllByVehicle(ctx context.Context, vehicleID string) ([]models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + `
  	              FROM charge_sessions WHERE vehicle_id = ?` + uc + ` ORDER BY created_at DESC`
	args := append([]any{vehicleID}, ua...)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanChargeSessionRows(rows)
}

func (r *ChargeSessionRepository) GetLatest(ctx context.Context, limit, offset int) ([]models.ChargeSession, error) {
	wc, wa := whereUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + ` FROM charge_sessions` + wc + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args := append(wa, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanChargeSessionRows(rows)
}

func (r *ChargeSessionRepository) GetLatestByVehicle(ctx context.Context, vehicleID string, limit, offset int) ([]models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + `
  		          FROM charge_sessions WHERE vehicle_id = ?` + uc + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args := append(append([]any{vehicleID}, ua...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanChargeSessionRows(rows)
}

func (r *ChargeSessionRepository) GetByDate(ctx context.Context, date string, limit, offset int) ([]models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + `
  		          FROM charge_sessions WHERE DATE(created_at) = ?` + uc + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args := append(append([]any{date}, ua...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanChargeSessionRows(rows)
}

func (r *ChargeSessionRepository) GetByVehicleAndDate(ctx context.Context, vehicleID, date string, limit, offset int) ([]models.ChargeSession, error) {
	uc, ua := andUserClause(ctx)
	query := `SELECT ` + chargeSessionColumns + `
  		          FROM charge_sessions WHERE vehicle_id = ? AND DATE(created_at) = ?` + uc + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args := append(append([]any{vehicleID, date}, ua...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanChargeSessionRows(rows)
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanChargeSession(s *models.ChargeSession, scanner sqlScanner) error {
	return scanner.Scan(&s.ID, &s.VehicleID, &s.CreatedAt,
		newNullTime(&s.EndedAt),
		&s.StartKwh, newNullFloat(&s.EndKwh), &s.TargetKwh, &s.StartPercent, newNullFloat(&s.EndPercent), &s.TargetPercent, &s.Status, newNullFloat(&s.StartTotalKwh), newNullTime(&s.StartedAt), newNullFloat(&s.LastBlendedKwh),
		newNullString(&s.PlugID),
		newNullFloat(&s.BatteryKwh), newNullFloat(&s.WallKwh), newNullFloat(&s.AvgCarbonIntensity), newNullFloat(&s.Co2Grams),
		newNullString(&s.UserID),
		newNullFloat(&s.CostPence), newNullFloat(&s.OffPeakKwh),
		newNullFloat(&s.HoldPercent), newNullString(&s.ReadyByTime))
}

// queryOneSession executes a single-row query and returns the session.
// Returns (nil, nil) when no row matches.
func (r *ChargeSessionRepository) queryOneSession(ctx context.Context, query string, args ...any) (*models.ChargeSession, error) {
	var s models.ChargeSession
	err := scanChargeSession(&s, r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanChargeSessionRows(rows *sql.Rows) ([]models.ChargeSession, error) {
	defer rows.Close()

	var sessions []models.ChargeSession
	for rows.Next() {
		var s models.ChargeSession
		err := scanChargeSession(&s, rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

func (r *ChargeSessionRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE charge_sessions SET status = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, id)

	return err
}

// ActivatePending atomically transitions a pending session to active.
// Returns ErrSessionNotPending if no rows were affected (session not pending).
func (r *ChargeSessionRepository) ActivatePending(ctx context.Context, id string, startedAt time.Time) error {
	query := `UPDATE charge_sessions SET started_at = ?, status = ? WHERE id = ? AND status = ?`
	result, err := r.db.ExecContext(ctx, query, startedAt.Format(time.RFC3339), models.SessionStatusActive, id, models.SessionStatusPending)
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

// ResumeHolding atomically transitions a holding session back to active and
// clears hold_percent, so subsequent auto-stop checks compare progress against
// the real target instead of the intermediate hold point.
// Returns ErrSessionWrongState if no rows were affected (session not holding).
func (r *ChargeSessionRepository) ResumeHolding(ctx context.Context, id string) error {
	query := `UPDATE charge_sessions SET status = ?, hold_percent = NULL WHERE id = ? AND status = ?`
	result, err := r.db.ExecContext(ctx, query, models.SessionStatusActive, id, models.SessionStatusHolding)
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

func (r *ChargeSessionRepository) UpdateStartTotalKwh(ctx context.Context, id string, startTotalKwh float64) error {
	query := `UPDATE charge_sessions SET start_total_kwh = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, startTotalKwh, id)
	return err
}

func (r *ChargeSessionRepository) UpdateEndedAt(ctx context.Context, id string, endedAt time.Time) error {
	query := `UPDATE charge_sessions SET ended_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, endedAt.Format(time.DateTime), id)

	return err
}

func (r *ChargeSessionRepository) UpdateCancelData(ctx context.Context, id string, endedAt time.Time) error {
	query := `UPDATE charge_sessions SET status = ?, ended_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, models.SessionStatusCancelled, endedAt.Format(time.DateTime), id)

	return err
}

// CancelPending atomically transitions a pending session to cancelled.
// Returns ErrSessionNotPending if no rows were affected (session not pending).
func (r *ChargeSessionRepository) CancelPending(ctx context.Context, id string, endedAt time.Time) error {
	query := `UPDATE charge_sessions SET status = ?, ended_at = ? WHERE id = ? AND status = ?`
	result, err := r.db.ExecContext(ctx, query, models.SessionStatusCancelled, endedAt.Format(time.DateTime), id, models.SessionStatusPending)
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

func (r *ChargeSessionRepository) UpdateEndWithStats(ctx context.Context, id string, endedAt time.Time, endKwh, endPercent float64, batteryKwh, wallKwh, co2Grams float64, avgCarbonIntensity *float64, costPence, offPeakKwh float64) error {
	query := `UPDATE charge_sessions SET ended_at = ?, end_kwh = ?, end_percent = ?, status = ?, battery_kwh = ?, wall_kwh = ?, avg_carbon_intensity = ?, co2_grams = ?, cost_pence = ?, off_peak_kwh = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, endedAt, endKwh, endPercent, models.SessionStatusCompleted, batteryKwh, wallKwh, toNullFloat(avgCarbonIntensity), co2Grams, costPence, offPeakKwh, id)

	return err
}

func (r *ChargeSessionRepository) UpdateLastBlendedKwh(ctx context.Context, id string, lastBlendedKwh float64) error {
	query := `UPDATE charge_sessions SET last_blended_kwh = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, lastBlendedKwh, id)
	return err
}

func (r *ChargeSessionRepository) CreatePowerReading(ctx context.Context, reading *models.PowerReading) error {
	query := `INSERT INTO power_readings (id, session_id, timestamp, voltage, current, power, energy_kwh, carbon_intensity_g_co2_per_kwh)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, reading.ID, reading.SessionID, reading.Timestamp,
		reading.Voltage, reading.Current, reading.Power, reading.EnergyKwh, reading.CarbonIntensityGCo2PerKwh)

	return err
}

func (r *ChargeSessionRepository) UpdateTarget(ctx context.Context, id string, targetPercent float64) error {
	// Single atomic UPDATE that computes target_kwh from the joined vehicle capacity_kwh.
	// Uses WHERE-clause guard to only update active sessions.
	query := `UPDATE charge_sessions SET target_percent = ?, target_kwh = (
		SELECT vehicle_models.capacity_kwh * ? / 100
		FROM vehicles JOIN vehicle_models ON vehicle_models.id = vehicles.model_id
		WHERE vehicles.id = charge_sessions.vehicle_id
	)
	WHERE id = ? AND status = ?`
	result, err := r.db.ExecContext(ctx, query, targetPercent, targetPercent, id, models.SessionStatusActive)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSessionWrongState // reused: session not in expected state
	}
	return nil
}

func (r *ChargeSessionRepository) GetPowerReadings(ctx context.Context, sessionID string) ([]models.PowerReading, error) {
	query := `SELECT id, session_id, timestamp, voltage, current, power, energy_kwh, carbon_intensity_g_co2_per_kwh
	          FROM power_readings WHERE session_id = ? ORDER BY timestamp`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []models.PowerReading
	for rows.Next() {
		var pr models.PowerReading
		err := rows.Scan(&pr.ID, &pr.SessionID, &pr.Timestamp, &pr.Voltage, &pr.Current, &pr.Power, &pr.EnergyKwh, &pr.CarbonIntensityGCo2PerKwh)
		if err != nil {
			return nil, err
		}
		readings = append(readings, pr)
	}

	return readings, rows.Err()
}

// GetAvgCarbonIntensityForSessions returns the average carbon intensity per session
// for the given session IDs. Sessions with no readings return nil.
func (r *ChargeSessionRepository) GetAvgCarbonIntensityForSessions(ctx context.Context, sessionIDs []string) (map[string]*float64, error) {
	if len(sessionIDs) == 0 {
		return map[string]*float64{}, nil
	}

	args := toAnySlice(sessionIDs)

	query := `SELECT session_id, AVG(carbon_intensity_g_co2_per_kwh)
	          FROM power_readings
	          WHERE session_id IN (` + buildPlaceholders(len(sessionIDs)) + `)
	          AND carbon_intensity_g_co2_per_kwh IS NOT NULL
	          GROUP BY session_id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*float64, len(sessionIDs))
	for rows.Next() {
		var sessionID string
		var avg float64
		if err := rows.Scan(&sessionID, &avg); err != nil {
			return nil, err
		}
		v := avg
		result[sessionID] = &v
	}

	return result, rows.Err()
}

func (r *ChargeSessionRepository) CreateSOCSnapshot(ctx context.Context, snapshot *models.SOCSnapshot) error {
	query := `INSERT INTO soc_snapshots (id, session_id, timestamp, soc_percent)
	          VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, snapshot.ID, snapshot.SessionID, snapshot.Timestamp, snapshot.SocPercent)

	return err
}

func (r *ChargeSessionRepository) GetSOCSnapshots(ctx context.Context, sessionID string) ([]models.SOCSnapshot, error) {
	query := `SELECT id, session_id, timestamp, soc_percent
	          FROM soc_snapshots WHERE session_id = ? ORDER BY timestamp`
	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.SOCSnapshot
	for rows.Next() {
		var s models.SOCSnapshot
		err := rows.Scan(&s.ID, &s.SessionID, &s.Timestamp, &s.SocPercent)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}

	return snapshots, rows.Err()
}

func (r *ChargeSessionRepository) GetLastSOCSnapshot(ctx context.Context, sessionID string) (*models.SOCSnapshot, error) {
	query := `SELECT id, session_id, timestamp, soc_percent
	          FROM soc_snapshots WHERE session_id = ? ORDER BY timestamp DESC LIMIT 1`
	var s models.SOCSnapshot
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(&s.ID, &s.SessionID, &s.Timestamp, &s.SocPercent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ChargeSessionRepository) GetLastPowerReading(ctx context.Context, sessionID string) (*models.PowerReading, error) {
	query := `SELECT id, session_id, timestamp, voltage, current, power, energy_kwh, carbon_intensity_g_co2_per_kwh
	          FROM power_readings WHERE session_id = ? ORDER BY timestamp DESC LIMIT 1`
	var pr models.PowerReading
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(
		&pr.ID, &pr.SessionID, &pr.Timestamp, &pr.Voltage, &pr.Current, &pr.Power, &pr.EnergyKwh, &pr.CarbonIntensityGCo2PerKwh,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

func (r *ChargeSessionRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := r.deleteInTx(ctx, tx, id); err != nil {
		tx.Rollback() //nolint:errcheck
		return err
	}
	return tx.Commit()
}

func (r *ChargeSessionRepository) deleteInTx(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM power_readings WHERE session_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM soc_snapshots WHERE session_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM charge_sessions WHERE id = ?`, id)
	return err
}

// GetVehicleChargingEfficiency returns the charging efficiency for a vehicle
// by joining vehicles -> vehicle_models.
func (r *ChargeSessionRepository) GetVehicleChargingEfficiency(ctx context.Context, vehicleID string) (float64, error) {
	var eff float64
	err := r.db.QueryRowContext(ctx,
		`SELECT m.charging_efficiency FROM vehicles v JOIN vehicle_models m ON m.id = v.model_id WHERE v.id = ?`,
		vehicleID,
	).Scan(&eff)
	if err != nil {
		return 0, err
	}
	return eff, nil
}

func (r *ChargeSessionRepository) ResolveChartSession(ctx context.Context, sessionID, vehicleID string) (*models.ChargeSession, error) {
	if sessionID != "" {
		return r.FindByID(ctx, sessionID)
	}

	if vehicleID != "" {
		session, err := r.GetActiveByVehicle(ctx, vehicleID)
		if err != nil {
			return nil, err
		}
		if session != nil {
			return session, nil
		}

		return r.GetLastCompletedByVehicle(ctx, vehicleID)
	}

	session, err := r.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	if session != nil {
		return session, nil
	}

	return r.GetLastCompleted(ctx)
}

// GetSessionAggregates returns aggregate statistics for completed sessions with
// pre-computed stats (battery_kwh IS NOT NULL) for a vehicle, optionally filtered
// by a time cutoff. When cutoff is zero, no time filter is applied (lifetime).
func (r *ChargeSessionRepository) GetSessionAggregates(ctx context.Context, vehicleID string, cutoff time.Time) (*models.SessionAggregates, error) {
	query := `SELECT COUNT(*), COALESCE(SUM(battery_kwh), 0), COALESCE(SUM(wall_kwh), 0),
		COALESCE(SUM(co2_grams), 0),
		CASE WHEN SUM(wall_kwh) > 0 THEN SUM(co2_grams) / SUM(wall_kwh) ELSE NULL END,
		COALESCE(MIN(battery_kwh), 0), COALESCE(MAX(battery_kwh), 0),
		COALESCE(SUM(cost_pence), 0)
		FROM charge_sessions
	 WHERE vehicle_id = ? AND status = ? AND battery_kwh IS NOT NULL`

	args := []any{vehicleID, models.SessionStatusCompleted}

	if !cutoff.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, cutoff.Format(time.RFC3339))
	}

	uc, ua := andUserClause(ctx)
	query += uc
	args = append(args, ua...)

	var agg models.SessionAggregates
	var avgCarbon sql.NullFloat64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&agg.TotalSessions, &agg.TotalBatteryKwh, &agg.TotalWallKwh, &agg.TotalCo2Grams, &avgCarbon,
		&agg.MinSessionBatteryKwh, &agg.MaxSessionBatteryKwh, &agg.TotalCostPence,
	)
	if err != nil {
		return nil, err
	}
	if avgCarbon.Valid {
		agg.AvgCarbonGCo2Kwh = &avgCarbon.Float64
	}
	return &agg, nil
}

// GetDailyEnergy returns daily energy data for completed sessions with pre-computed
// stats for a vehicle, optionally filtered by a time cutoff. Results are grouped
// by date and sorted ascending.
func (r *ChargeSessionRepository) GetDailyEnergy(ctx context.Context, vehicleID string, cutoff time.Time) ([]models.DailyEnergy, error) {
	query := `SELECT DATE(created_at),
		COALESCE(SUM(battery_kwh), 0),
		COUNT(*),
		COALESCE(SUM(co2_grams), 0),
		CASE WHEN SUM(wall_kwh) > 0 THEN SUM(co2_grams) / SUM(wall_kwh) ELSE NULL END
	 FROM charge_sessions
	 WHERE vehicle_id = ? AND status = ? AND battery_kwh IS NOT NULL`

	args := []any{vehicleID, models.SessionStatusCompleted}

	if !cutoff.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, cutoff.Format(time.RFC3339))
	}

	uc, ua := andUserClause(ctx)
	query += uc
	args = append(args, ua...)

	query += ` GROUP BY DATE(created_at) ORDER BY DATE(created_at)`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var daily []models.DailyEnergy
	for rows.Next() {
		var de models.DailyEnergy
		var avgCarbon sql.NullFloat64
		if err := rows.Scan(&de.Date, &de.BatteryKwh, &de.SessionCount, &de.Co2Grams, &avgCarbon); err != nil {
			return nil, err
		}
		if avgCarbon.Valid {
			de.AvgCarbonIntensityGCo2PerKwh = &avgCarbon.Float64
		}
		daily = append(daily, de)
	}
	return daily, rows.Err()
}
