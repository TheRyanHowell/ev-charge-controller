-- Revert charge_sessions: make user_id and plug_id nullable again
CREATE TABLE charge_sessions_old (
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
  user_id TEXT REFERENCES users(id),
  plug_id TEXT REFERENCES plugs(id),
  FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE CASCADE
);
INSERT INTO charge_sessions_old SELECT * FROM charge_sessions;
DROP TABLE charge_sessions;
ALTER TABLE charge_sessions_old RENAME TO charge_sessions;
CREATE INDEX idx_charge_sessions_status ON charge_sessions(status);
CREATE INDEX idx_charge_sessions_status_created ON charge_sessions(status, created_at DESC);
CREATE INDEX idx_charge_sessions_vehicle_id ON charge_sessions(vehicle_id);
CREATE INDEX idx_charge_sessions_vehicle_created ON charge_sessions(vehicle_id, created_at);
CREATE INDEX idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at DESC);
CREATE INDEX idx_charge_sessions_created_at ON charge_sessions(created_at);
CREATE INDEX idx_charge_sessions_user_id ON charge_sessions(user_id);
CREATE INDEX idx_charge_sessions_plug_id ON charge_sessions(plug_id);

-- Revert power_readings: make session_id nullable again
CREATE TABLE power_readings_old (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  timestamp DATETIME NOT NULL,
  voltage REAL,
  current REAL,
  power REAL,
  energy_kwh REAL,
  carbon_intensity_g_co2_per_kwh REAL,
  FOREIGN KEY (session_id) REFERENCES charge_sessions(id) ON DELETE CASCADE
);
INSERT INTO power_readings_old SELECT * FROM power_readings;
DROP TABLE power_readings;
ALTER TABLE power_readings_old RENAME TO power_readings;
CREATE INDEX idx_power_readings_session_id ON power_readings(session_id);
CREATE INDEX idx_power_readings_session_timestamp ON power_readings(session_id, timestamp);

-- Revert soc_snapshots: make session_id and soc_percent nullable again
CREATE TABLE soc_snapshots_old (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  timestamp DATETIME NOT NULL,
  soc_percent REAL,
  FOREIGN KEY (session_id) REFERENCES charge_sessions(id) ON DELETE CASCADE
);
INSERT INTO soc_snapshots_old SELECT * FROM soc_snapshots;
DROP TABLE soc_snapshots;
ALTER TABLE soc_snapshots_old RENAME TO soc_snapshots;
CREATE INDEX idx_soc_snapshots_session_id ON soc_snapshots(session_id);
CREATE INDEX idx_soc_snapshots_session_timestamp ON soc_snapshots(session_id, timestamp);

-- Revert vehicles: make user_id nullable again
CREATE TABLE vehicles_old (
  id TEXT PRIMARY KEY,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL REFERENCES vehicle_models(id),
  name TEXT NOT NULL,
  current_percent REAL NOT NULL DEFAULT 20.0,
  target_percent REAL NOT NULL DEFAULT 80.0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO vehicles_old SELECT * FROM vehicles;
DROP TABLE vehicles;
ALTER TABLE vehicles_old RENAME TO vehicles;
CREATE INDEX idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX idx_vehicles_model_id ON vehicles(model_id);
CREATE UNIQUE INDEX idx_vehicles_user_name ON vehicles(user_id, name);

-- Revert push_subscriptions: make created_at and user_id nullable again
CREATE TABLE push_subscriptions_old (
  id TEXT PRIMARY KEY,
  endpoint TEXT NOT NULL,
  p256dh_key TEXT NOT NULL,
  auth_key TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  user_id TEXT REFERENCES users(id),
  UNIQUE(endpoint, p256dh_key)
);
INSERT INTO push_subscriptions_old SELECT * FROM push_subscriptions;
DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions_old RENAME TO push_subscriptions;
CREATE INDEX idx_push_subscriptions_user_id ON push_subscriptions(user_id);

-- Revert schedules: make user_id and plug_id nullable again
CREATE TABLE schedules_old (
  id TEXT PRIMARY KEY,
  plug_id TEXT UNIQUE REFERENCES plugs(id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(id),
  time TEXT NOT NULL DEFAULT '00:00',
  enabled INTEGER NOT NULL DEFAULT 0
);
INSERT INTO schedules_old SELECT * FROM schedules;
DROP TABLE schedules;
ALTER TABLE schedules_old RENAME TO schedules;
CREATE INDEX idx_schedules_plug_id ON schedules(plug_id);
