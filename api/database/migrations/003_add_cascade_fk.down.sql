-- Remove ON DELETE CASCADE from foreign keys (reverse of 003).

-- Charge sessions: remove cascade
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
  FOREIGN KEY (vehicle_id) REFERENCES vehicles(id)
);
INSERT INTO charge_sessions_new SELECT * FROM charge_sessions;
DROP TABLE charge_sessions;
ALTER TABLE charge_sessions_new RENAME TO charge_sessions;
CREATE INDEX idx_charge_sessions_status ON charge_sessions(status);
CREATE INDEX idx_charge_sessions_status_created ON charge_sessions(status, created_at DESC);
CREATE INDEX idx_charge_sessions_vehicle_id ON charge_sessions(vehicle_id);
CREATE INDEX idx_charge_sessions_vehicle_created ON charge_sessions(vehicle_id, created_at);
CREATE INDEX idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at DESC);
CREATE INDEX idx_charge_sessions_created_at ON charge_sessions(created_at);

-- Power readings: remove cascade
CREATE TABLE power_readings_new (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  timestamp DATETIME NOT NULL,
  voltage REAL,
  current REAL,
  power REAL,
  energy_kwh REAL,
  FOREIGN KEY (session_id) REFERENCES charge_sessions(id)
);
INSERT INTO power_readings_new SELECT * FROM power_readings;
DROP TABLE power_readings;
ALTER TABLE power_readings_new RENAME TO power_readings;
CREATE INDEX idx_power_readings_session_id ON power_readings(session_id);
CREATE INDEX idx_power_readings_session_timestamp ON power_readings(session_id, timestamp);

-- SOC snapshots: remove cascade
CREATE TABLE soc_snapshots_new (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  timestamp DATETIME NOT NULL,
  soc_percent REAL,
  FOREIGN KEY (session_id) REFERENCES charge_sessions(id)
);
INSERT INTO soc_snapshots_new SELECT * FROM soc_snapshots;
DROP TABLE soc_snapshots;
ALTER TABLE soc_snapshots_new RENAME TO soc_snapshots;
CREATE INDEX idx_soc_snapshots_session_id ON soc_snapshots(session_id);
CREATE INDEX idx_soc_snapshots_session_timestamp ON soc_snapshots(session_id, timestamp);
