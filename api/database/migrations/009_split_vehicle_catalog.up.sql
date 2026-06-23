-- Create the shared vehicle catalog (global, seeded, no user ownership).
CREATE TABLE vehicle_models (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  capacity_kwh REAL NOT NULL,
  charger_output_w REAL NOT NULL,
  charging_efficiency REAL NOT NULL DEFAULT 0.8,
  time_0_100_min INTEGER,
  time_0_80_min INTEGER,
  time_20_80_min INTEGER,
  time_20_100_min INTEGER,
  pack_voltage_max_v REAL,
  pack_cutoff_current_ma REAL,
  range_min_mi REAL NOT NULL DEFAULT 0.0,
  range_max_mi REAL NOT NULL DEFAULT 0.0
);

-- Populate vehicle_models from the distinct catalog rows in vehicles.
-- The three Maeving ids (rm1, rm1s, rm2) map 1:1 to vehicle_model ids.
INSERT INTO vehicle_models (id, name, capacity_kwh, charger_output_w, charging_efficiency,
  time_0_100_min, time_0_80_min, time_20_80_min, time_20_100_min,
  pack_voltage_max_v, pack_cutoff_current_ma, range_min_mi, range_max_mi)
SELECT id, name, capacity_kwh, charger_output_w, charging_efficiency,
  time_0_100_min, time_0_80_min, time_20_80_min, time_20_100_min,
  pack_voltage_max_v, pack_cutoff_current_ma, range_min_mi, range_max_mi
FROM vehicles;

-- Rebuild vehicles as per-user instances referencing vehicle_models.
-- Existing rows keep their id, user_id, current_percent, target_percent, selected, created_at.
-- model_id defaults to the row's own id (1:1 mapping for the seeded Maevings).
-- name becomes a user-visible nickname (defaults to the catalog model name until renamed).
CREATE TABLE vehicles_new (
  id TEXT PRIMARY KEY,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL REFERENCES vehicle_models(id),
  name TEXT NOT NULL,
  current_percent REAL NOT NULL DEFAULT 20.0,
  target_percent REAL NOT NULL DEFAULT 80.0,
  selected INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO vehicles_new (id, user_id, model_id, name, current_percent, target_percent, selected, created_at)
SELECT id, user_id, id AS model_id, name, current_percent, target_percent, selected, created_at
FROM vehicles;

DROP TABLE vehicles;
ALTER TABLE vehicles_new RENAME TO vehicles;

CREATE INDEX idx_vehicles_selected ON vehicles(selected);
CREATE INDEX idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX idx_vehicles_model_id ON vehicles(model_id);
