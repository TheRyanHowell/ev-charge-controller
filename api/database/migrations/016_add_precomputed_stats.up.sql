-- Per-session stats on charge_sessions (NULL until session completes)
ALTER TABLE charge_sessions ADD COLUMN battery_kwh REAL;
ALTER TABLE charge_sessions ADD COLUMN wall_kwh REAL;
ALTER TABLE charge_sessions ADD COLUMN avg_carbon_intensity REAL;
ALTER TABLE charge_sessions ADD COLUMN co2_grams REAL;

-- Lifetime stats on vehicles
ALTER TABLE vehicles ADD COLUMN total_sessions INT NOT NULL DEFAULT 0;
ALTER TABLE vehicles ADD COLUMN total_battery_kwh REAL NOT NULL DEFAULT 0;
ALTER TABLE vehicles ADD COLUMN total_wall_kwh REAL NOT NULL DEFAULT 0;
ALTER TABLE vehicles ADD COLUMN total_co2_grams REAL NOT NULL DEFAULT 0;
ALTER TABLE vehicles ADD COLUMN last_session_at TEXT;

-- Index for stats queries: vehicle_id + status + created_at
-- Seeks directly to vehicle+completed subset, range-scans by date
CREATE INDEX IF NOT EXISTS idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at);
