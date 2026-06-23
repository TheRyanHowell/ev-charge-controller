-- Rebuild vehicles as the original flat table (re-merge catalog columns).
CREATE TABLE vehicles_restored (
  id TEXT PRIMARY KEY,
  user_id TEXT REFERENCES users(id),
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
  range_max_mi REAL NOT NULL DEFAULT 0.0,
  current_percent REAL NOT NULL DEFAULT 20.0,
  target_percent REAL NOT NULL DEFAULT 80.0,
  selected INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO vehicles_restored (id, user_id, name, capacity_kwh, charger_output_w, charging_efficiency,
  time_0_100_min, time_0_80_min, time_20_80_min, time_20_100_min,
  pack_voltage_max_v, pack_cutoff_current_ma, range_min_mi, range_max_mi,
  current_percent, target_percent, selected, created_at)
SELECT v.id, v.user_id, v.name,
  m.capacity_kwh, m.charger_output_w, m.charging_efficiency,
  m.time_0_100_min, m.time_0_80_min, m.time_20_80_min, m.time_20_100_min,
  m.pack_voltage_max_v, m.pack_cutoff_current_ma, m.range_min_mi, m.range_max_mi,
  v.current_percent, v.target_percent, v.selected, v.created_at
FROM vehicles v
JOIN vehicle_models m ON m.id = v.model_id;

DROP TABLE vehicles;
ALTER TABLE vehicles_restored RENAME TO vehicles;
DROP TABLE vehicle_models;

CREATE INDEX idx_vehicles_selected ON vehicles(selected);
CREATE INDEX idx_vehicles_user_id ON vehicles(user_id);
