DROP INDEX IF EXISTS idx_schedules_plug_id;
DROP INDEX IF EXISTS idx_charge_sessions_plug_id;
DROP INDEX IF EXISTS idx_charge_sessions_user_id;
DROP INDEX IF EXISTS idx_push_subscriptions_user_id;
DROP INDEX IF EXISTS idx_vehicles_user_id;

-- Restore schedules to singleton structure
CREATE TABLE schedules_v1 (
  id TEXT PRIMARY KEY,
  time TEXT NOT NULL DEFAULT '00:00',
  enabled INTEGER NOT NULL DEFAULT 0
);
INSERT INTO schedules_v1 (id, time, enabled)
  SELECT id, time, enabled FROM schedules;
DROP TABLE schedules;
ALTER TABLE schedules_v1 RENAME TO schedules;

-- Rebuild charge_sessions without user_id/plug_id
CREATE TABLE charge_sessions_v1 (
  id TEXT PRIMARY KEY,
  vehicle_id TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at DATETIME,
  start_kwh REAL NOT NULL,
  end_kwh REAL,
  target_kwh REAL NOT NULL,
  start_percent REAL NOT NULL,
  end_percent REAL,
  target_percent REAL NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  start_total_kwh REAL,
  started_at DATETIME,
  last_blended_kwh REAL,
  FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE CASCADE
);
INSERT INTO charge_sessions_v1 SELECT
  id, vehicle_id, created_at, ended_at, start_kwh, end_kwh,
  target_kwh, start_percent, end_percent, target_percent,
  status, start_total_kwh, started_at, last_blended_kwh
FROM charge_sessions;
DROP TABLE charge_sessions;
ALTER TABLE charge_sessions_v1 RENAME TO charge_sessions;
CREATE INDEX idx_charge_sessions_status ON charge_sessions(status);
CREATE INDEX idx_charge_sessions_status_created ON charge_sessions(status, created_at DESC);
CREATE INDEX idx_charge_sessions_vehicle_id ON charge_sessions(vehicle_id);
CREATE INDEX idx_charge_sessions_vehicle_created ON charge_sessions(vehicle_id, created_at);
CREATE INDEX idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at DESC);
CREATE INDEX idx_charge_sessions_created_at ON charge_sessions(created_at);

-- Rebuild push_subscriptions without user_id
CREATE TABLE push_subscriptions_v1 (
  id TEXT PRIMARY KEY,
  endpoint TEXT NOT NULL,
  p256dh_key TEXT NOT NULL,
  auth_key TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(endpoint, p256dh_key)
);
INSERT INTO push_subscriptions_v1 SELECT id, endpoint, p256dh_key, auth_key, created_at FROM push_subscriptions;
DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions_v1 RENAME TO push_subscriptions;

-- Rebuild vehicles without user_id
CREATE TABLE vehicles_v1 (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  capacity_kwh REAL NOT NULL,
  charger_output_w REAL NOT NULL,
  charging_efficiency REAL DEFAULT 0.8 NOT NULL,
  time_0_100_min INTEGER,
  time_0_80_min INTEGER,
  time_20_80_min INTEGER,
  time_20_100_min INTEGER,
  pack_voltage_max_v REAL,
  pack_cutoff_current_ma REAL,
  range_min_mi REAL DEFAULT 0.0,
  range_max_mi REAL DEFAULT 0.0,
  current_percent REAL DEFAULT 20.0,
  target_percent REAL DEFAULT 80.0,
  selected INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO vehicles_v1 SELECT
  id, name, capacity_kwh, charger_output_w, charging_efficiency,
  time_0_100_min, time_0_80_min, time_20_80_min, time_20_100_min,
  pack_voltage_max_v, pack_cutoff_current_ma,
  range_min_mi, range_max_mi, current_percent, target_percent,
  selected, created_at
FROM vehicles;
DROP TABLE vehicles;
ALTER TABLE vehicles_v1 RENAME TO vehicles;
CREATE INDEX idx_vehicles_selected ON vehicles(selected);
