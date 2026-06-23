-- Add ON DELETE CASCADE to charge_sessions.plug_id

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
  plug_id TEXT NOT NULL REFERENCES plugs(id) ON DELETE CASCADE,
  FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE CASCADE
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
CREATE INDEX idx_charge_sessions_user_id ON charge_sessions(user_id);
CREATE INDEX idx_charge_sessions_plug_id ON charge_sessions(plug_id);
