-- Make user_id NOT NULL on charge_sessions (rebuild table)
CREATE TABLE charge_sessions_new (
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
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plug_id TEXT NOT NULL REFERENCES plugs(id),
  FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE CASCADE
);
INSERT INTO charge_sessions_new SELECT * FROM charge_sessions WHERE user_id IS NOT NULL AND plug_id IS NOT NULL;
DROP TABLE charge_sessions;
ALTER TABLE charge_sessions_new RENAME TO charge_sessions;
CREATE INDEX idx_charge_sessions_status ON charge_sessions(status);
CREATE INDEX idx_charge_sessions_status_created ON charge_sessions(status, created_at DESC);
CREATE INDEX idx_charge_sessions_vehicle_id ON charge_sessions(vehicle_id);
CREATE INDEX idx_charge_sessions_vehicle_created ON charge_sessions(vehicle_id, created_at);
CREATE INDEX idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at DESC);
CREATE INDEX idx_charge_sessions_created_at ON charge_sessions(created_at);
CREATE INDEX idx_charge_sessions_user_id ON charge_sessions(user_id);
CREATE INDEX idx_charge_sessions_plug_id ON charge_sessions(plug_id);

-- Make session_id NOT NULL on power_readings (rebuild table)
CREATE TABLE power_readings_new (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES charge_sessions(id) ON DELETE CASCADE,
  timestamp DATETIME NOT NULL,
  voltage REAL,
  current REAL,
  power REAL,
  energy_kwh REAL,
  carbon_intensity_g_co2_per_kwh REAL
);
INSERT INTO power_readings_new SELECT * FROM power_readings WHERE session_id IS NOT NULL;
DROP TABLE power_readings;
ALTER TABLE power_readings_new RENAME TO power_readings;
CREATE INDEX idx_power_readings_session_id ON power_readings(session_id);
CREATE INDEX idx_power_readings_session_timestamp ON power_readings(session_id, timestamp);

-- Make session_id and soc_percent NOT NULL on soc_snapshots (rebuild table)
CREATE TABLE soc_snapshots_new (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES charge_sessions(id) ON DELETE CASCADE,
  timestamp DATETIME NOT NULL,
  soc_percent REAL NOT NULL
);
INSERT INTO soc_snapshots_new SELECT * FROM soc_snapshots WHERE session_id IS NOT NULL AND soc_percent IS NOT NULL;
DROP TABLE soc_snapshots;
ALTER TABLE soc_snapshots_new RENAME TO soc_snapshots;
CREATE INDEX idx_soc_snapshots_session_id ON soc_snapshots(session_id);
CREATE INDEX idx_soc_snapshots_session_timestamp ON soc_snapshots(session_id, timestamp);

-- Make user_id NOT NULL on vehicles (rebuild table)
CREATE TABLE vehicles_new (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL REFERENCES vehicle_models(id),
  name TEXT NOT NULL,
  current_percent REAL NOT NULL DEFAULT 20.0,
  target_percent REAL NOT NULL DEFAULT 80.0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO vehicles_new SELECT * FROM vehicles WHERE user_id IS NOT NULL;
DROP TABLE vehicles;
ALTER TABLE vehicles_new RENAME TO vehicles;
CREATE INDEX idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX idx_vehicles_model_id ON vehicles(model_id);
CREATE UNIQUE INDEX idx_vehicles_user_name ON vehicles(user_id, name);

-- Make created_at and user_id NOT NULL on push_subscriptions (rebuild table)
CREATE TABLE push_subscriptions_new (
  id TEXT PRIMARY KEY,
  endpoint TEXT NOT NULL,
  p256dh_key TEXT NOT NULL,
  auth_key TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(endpoint, p256dh_key)
);
INSERT INTO push_subscriptions_new SELECT * FROM push_subscriptions WHERE user_id IS NOT NULL;
DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions_new RENAME TO push_subscriptions;
CREATE INDEX idx_push_subscriptions_user_id ON push_subscriptions(user_id);

-- Make user_id and plug_id NOT NULL on schedules (rebuild table)
CREATE TABLE schedules_new (
  id TEXT PRIMARY KEY,
  plug_id TEXT NOT NULL UNIQUE REFERENCES plugs(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  time TEXT NOT NULL DEFAULT '00:00',
  enabled INTEGER NOT NULL DEFAULT 0
);
INSERT INTO schedules_new SELECT * FROM schedules WHERE user_id IS NOT NULL AND plug_id IS NOT NULL;
DROP TABLE schedules;
ALTER TABLE schedules_new RENAME TO schedules;
CREATE INDEX idx_schedules_plug_id ON schedules(plug_id);
