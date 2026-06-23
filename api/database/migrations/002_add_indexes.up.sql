-- Composite index: filters by status, returns rows sorted by created_at DESC.
-- Covers GetActive, GetPending, GetLastCompleted (hot path - polled every tick).
CREATE INDEX idx_charge_sessions_status_created ON charge_sessions(status, created_at DESC);

-- Composite index: filters by vehicle_id + status, returns rows sorted by created_at DESC.
-- Covers GetActiveByVehicle, GetLastCompletedByVehicle.
CREATE INDEX idx_charge_sessions_vehicle_status_created ON charge_sessions(vehicle_id, status, created_at DESC);

-- Composite index: filters by session_id, returns rows sorted by timestamp.
-- Covers GetPowerReadings (eliminates in-memory sort for session readings).
CREATE INDEX idx_power_readings_session_timestamp ON power_readings(session_id, timestamp);

-- Composite index: filters by session_id, returns rows sorted by timestamp.
-- Covers GetSOCSnapshots (eliminates in-memory sort for session snapshots).
CREATE INDEX idx_soc_snapshots_session_timestamp ON soc_snapshots(session_id, timestamp);

-- Index on selected column for GetSelected query (currently full table scan).
CREATE INDEX idx_vehicles_selected ON vehicles(selected);
