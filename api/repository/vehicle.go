package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ev-charge-controller/api/models"

	"github.com/google/uuid"
)

type VehicleRepository struct {
	db *sql.DB
}

func NewVehicleRepository(db *sql.DB) *VehicleRepository {
	return &VehicleRepository{db: db}
}

// instanceColumns selects instance columns joined with catalog config from vehicle_models.
const instanceColumns = `v.id, v.model_id, v.user_id, v.name, v.current_percent, v.target_percent,
  v.created_at,
  m.name, m.capacity_kwh, m.charger_output_w, m.charging_efficiency,
  m.time_0_100_min, m.time_0_80_min, m.time_20_80_min, m.time_20_100_min,
  m.pack_voltage_max_v, m.pack_cutoff_current_ma, m.range_min_mi, m.range_max_mi,
  v.total_sessions, v.total_battery_kwh, v.total_wall_kwh, v.total_co2_grams, v.total_cost_pence,
  v.min_session_battery_kwh, v.max_session_battery_kwh, v.last_session_at,
  v.notify_charge_started, v.notify_charge_complete, v.notify_charger_offline, v.notify_maintenance_offline`

const instanceJoin = ` FROM vehicles v JOIN vehicle_models m ON m.id = v.model_id`

// CreateInstance inserts a new per-user vehicle instance.
// The ID is generated automatically; CreatedAt is set to now when zero.
func (r *VehicleRepository) CreateInstance(ctx context.Context, vehicle *models.Vehicle) error {
	if vehicle.ID == "" {
		vehicle.ID = uuid.New().String()
	}
	if vehicle.CreatedAt.IsZero() {
		vehicle.CreatedAt = time.Now()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		vehicle.ID, toNullString(vehicle.UserID), vehicle.ModelID, vehicle.Name,
		vehicle.CurrentPercent, vehicle.TargetPercent, vehicle.CreatedAt,
	)
	return err
}

// DeleteInstance removes a vehicle instance owned by userID.
func (r *VehicleRepository) DeleteInstance(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM vehicles WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func (r *VehicleRepository) FindByID(ctx context.Context, id string) (*models.Vehicle, error) {
	var v models.Vehicle
	err := scanVehicle(&v, r.db.QueryRowContext(ctx,
		`SELECT `+instanceColumns+instanceJoin+` WHERE v.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VehicleRepository) FindByIDs(ctx context.Context, ids []string) (map[string]*models.Vehicle, error) {
	result := make(map[string]*models.Vehicle)
	if len(ids) == 0 {
		return result, nil
	}

	args := toAnySlice(ids)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+instanceColumns+instanceJoin+
			` WHERE v.id IN (`+buildPlaceholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var v models.Vehicle
		if err := scanVehicle(&v, rows); err != nil {
			return nil, err
		}
		result[v.ID] = &v
	}
	return result, rows.Err()
}

func (r *VehicleRepository) UpdatePercents(ctx context.Context, id string, currentPercent, targetPercent float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE vehicles SET current_percent = ?, target_percent = ? WHERE id = ?`,
		currentPercent, targetPercent, id)
	return err
}

func (r *VehicleRepository) List(ctx context.Context) ([]models.Vehicle, error) {
	wc, wa := whereUserClause(ctx)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+instanceColumns+instanceJoin+wc+` ORDER BY v.created_at`, wa...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vehicles []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		if err := scanVehicle(&v, rows); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

// UpdateName updates the user-visible nickname of an instance.
func (r *VehicleRepository) UpdateName(ctx context.Context, id, name, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE vehicles SET name = ? WHERE id = ? AND user_id = ?`, name, id, userID)
	return err
}

// IncrementLifetimeStats atomically increments the vehicle's lifetime stats.
func (r *VehicleRepository) IncrementLifetimeStats(ctx context.Context, id string, batteryKwh, wallKwh, co2Grams, costPence float64, sessionAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE vehicles SET
			total_sessions = total_sessions + 1,
			total_battery_kwh = total_battery_kwh + ?,
			total_wall_kwh = total_wall_kwh + ?,
			total_co2_grams = total_co2_grams + ?,
			total_cost_pence = total_cost_pence + ?,
			last_session_at = ?,
			min_session_battery_kwh = CASE WHEN total_sessions = 0 THEN ? ELSE MIN(min_session_battery_kwh, ?) END,
			max_session_battery_kwh = CASE WHEN total_sessions = 0 THEN ? ELSE MAX(max_session_battery_kwh, ?) END
		WHERE id = ?`,
		batteryKwh, wallKwh, co2Grams, costPence, sessionAt.Format(time.RFC3339),
		batteryKwh, batteryKwh,
		batteryKwh, batteryKwh,
		id)
	return err
}

// DecrementLifetimeStats atomically decrements the vehicle's lifetime stats.
// Min/max session kWh are reset to 0 when the last session is removed; otherwise
// they remain unchanged (may be slightly stale, but decrement is a rare rollback path).
func (r *VehicleRepository) DecrementLifetimeStats(ctx context.Context, id string, batteryKwh, wallKwh, co2Grams, costPence float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE vehicles SET
			total_sessions = MAX(total_sessions - 1, 0),
			total_battery_kwh = MAX(total_battery_kwh - ?, 0),
			total_wall_kwh = MAX(total_wall_kwh - ?, 0),
			total_co2_grams = MAX(total_co2_grams - ?, 0),
			total_cost_pence = MAX(total_cost_pence - ?, 0),
			min_session_battery_kwh = CASE WHEN total_sessions <= 1 THEN 0 ELSE min_session_battery_kwh END,
			max_session_battery_kwh = CASE WHEN total_sessions <= 1 THEN 0 ELSE max_session_battery_kwh END
		WHERE id = ?`,
		batteryKwh, wallKwh, co2Grams, costPence, id)
	return err
}

func scanVehicle(v *models.Vehicle, scanner sqlScanner) error {
	var userID sql.NullString
	var notifyChargeStarted, notifyChargeComplete, notifyChargerOffline, notifyMaintenanceOffline int
	err := scanner.Scan(
		&v.ID, &v.ModelID, &userID, &v.Name,
		&v.CurrentPercent, &v.TargetPercent, &v.CreatedAt,
		&v.ModelName, &v.CapacityKwh, &v.ChargerOutputW, &v.ChargingEfficiency,
		newNullInt(&v.Time0to100Min), newNullInt(&v.Time0to80Min),
		newNullInt(&v.Time20to80Min), newNullInt(&v.Time20to100Min),
		newNullFloat(&v.PackVoltageMaxV), newNullFloat(&v.PackCutoffCurrentMa),
		&v.RangeMinMi, &v.RangeMaxMi,
		&v.TotalSessions, &v.TotalBatteryKwh, &v.TotalWallKwh, &v.TotalCo2Grams, &v.TotalCostPence,
		&v.MinSessionBatteryKwh, &v.MaxSessionBatteryKwh,
		newNullTime(&v.LastSessionAt),
		&notifyChargeStarted, &notifyChargeComplete, &notifyChargerOffline, &notifyMaintenanceOffline,
	)
	if err != nil {
		return err
	}
	if userID.Valid {
		v.UserID = &userID.String
	}
	v.NotifyChargeStarted = notifyChargeStarted != 0
	v.NotifyChargeComplete = notifyChargeComplete != 0
	v.NotifyChargerOffline = notifyChargerOffline != 0
	v.NotifyMaintenanceOffline = notifyMaintenanceOffline != 0
	return nil
}

// UpdateNotificationPrefs updates the per-vehicle notification preference toggles.
func (r *VehicleRepository) UpdateNotificationPrefs(ctx context.Context, id, userID string, notifyChargeStarted, notifyChargeComplete, notifyChargerOffline, notifyMaintenanceOffline bool) error {
	ncs := 0
	if notifyChargeStarted {
		ncs = 1
	}
	ncc := 0
	if notifyChargeComplete {
		ncc = 1
	}
	nco := 0
	if notifyChargerOffline {
		nco = 1
	}
	nmo := 0
	if notifyMaintenanceOffline {
		nmo = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE vehicles SET notify_charge_started = ?, notify_charge_complete = ?, notify_charger_offline = ?, notify_maintenance_offline = ?
		 WHERE id = ? AND user_id = ?`,
		ncs, ncc, nco, nmo, id, userID,
	)
	return err
}
