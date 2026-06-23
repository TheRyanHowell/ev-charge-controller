-- Vehicles table
CREATE TABLE IF NOT EXISTS vehicles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  capacity_kwh REAL NOT NULL,
  charger_output_w REAL NOT NULL,
  charging_efficiency REAL DEFAULT 0.8 NOT NULL,
  time_0_100_min INTEGER,
  time_0_80_min INTEGER,
  time_20_80_min INTEGER,
  time_20_100_min INTEGER,
  range_min_mi REAL DEFAULT 0.0,
  range_max_mi REAL DEFAULT 0.0,
  current_percent REAL DEFAULT 20.0,
  target_percent REAL DEFAULT 80.0,
  selected INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Charge sessions
CREATE TABLE IF NOT EXISTS charge_sessions (
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

-- Power readings
CREATE TABLE IF NOT EXISTS power_readings (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  timestamp DATETIME NOT NULL,
  voltage REAL,
  current REAL,
  power REAL,
  energy_kwh REAL,
  FOREIGN KEY (session_id) REFERENCES charge_sessions(id) ON DELETE CASCADE
);

-- SOC snapshots
CREATE TABLE IF NOT EXISTS soc_snapshots (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  timestamp DATETIME NOT NULL,
  soc_percent REAL,
  FOREIGN KEY (session_id) REFERENCES charge_sessions(id) ON DELETE CASCADE
);

-- Schedules table (singleton - one plug, one schedule)
CREATE TABLE IF NOT EXISTS schedules (
  id TEXT PRIMARY KEY,
  time TEXT NOT NULL,
  enabled INTEGER DEFAULT 0
);

-- Push subscriptions (Web Push / VAPID)
CREATE TABLE IF NOT EXISTS push_subscriptions (
  id TEXT PRIMARY KEY,
  endpoint TEXT NOT NULL,
  p256dh_key TEXT NOT NULL,
  auth_key TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(endpoint, p256dh_key)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_charge_sessions_status ON charge_sessions(status);
CREATE INDEX IF NOT EXISTS idx_charge_sessions_status_created ON charge_sessions(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_charge_sessions_vehicle_id ON charge_sessions(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_charge_sessions_vehicle_created ON charge_sessions(vehicle_id, created_at);
CREATE INDEX IF NOT EXISTS idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_charge_sessions_created_at ON charge_sessions(created_at);
CREATE INDEX IF NOT EXISTS idx_power_readings_session_id ON power_readings(session_id);
CREATE INDEX IF NOT EXISTS idx_power_readings_session_timestamp ON power_readings(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_soc_snapshots_session_id ON soc_snapshots(session_id);
CREATE INDEX IF NOT EXISTS idx_soc_snapshots_session_timestamp ON soc_snapshots(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_vehicles_selected ON vehicles(selected);
