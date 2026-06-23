package repository

import (
	"context"
	"database/sql"
	"errors"

	"ev-charge-controller/api/models"
)

// VehicleModelRepository provides read access to the global vehicle catalog.
type VehicleModelRepository struct {
	db *sql.DB
}

func NewVehicleModelRepository(db *sql.DB) *VehicleModelRepository {
	return &VehicleModelRepository{db: db}
}

const vehicleModelColumns = `id, name, capacity_kwh, charger_output_w, charging_efficiency,
  time_0_100_min, time_0_80_min, time_20_80_min, time_20_100_min,
  pack_voltage_max_v, pack_cutoff_current_ma, range_min_mi, range_max_mi`

func (r *VehicleModelRepository) List(ctx context.Context) ([]models.VehicleModel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+vehicleModelColumns+` FROM vehicle_models ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.VehicleModel
	for rows.Next() {
		var m models.VehicleModel
		if err := scanVehicleModel(&m, rows); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *VehicleModelRepository) FindByID(ctx context.Context, id string) (*models.VehicleModel, error) {
	var m models.VehicleModel
	err := scanVehicleModel(&m, r.db.QueryRowContext(ctx,
		`SELECT `+vehicleModelColumns+` FROM vehicle_models WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func scanVehicleModel(m *models.VehicleModel, scanner sqlScanner) error {
	return scanner.Scan(
		&m.ID, &m.Name, &m.CapacityKwh, &m.ChargerOutputW, &m.ChargingEfficiency,
		newNullInt(&m.Time0to100Min), newNullInt(&m.Time0to80Min),
		newNullInt(&m.Time20to80Min), newNullInt(&m.Time20to100Min),
		newNullFloat(&m.PackVoltageMaxV), newNullFloat(&m.PackCutoffCurrentMa),
		&m.RangeMinMi, &m.RangeMaxMi,
	)
}
