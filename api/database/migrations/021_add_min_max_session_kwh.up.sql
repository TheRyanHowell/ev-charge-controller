ALTER TABLE vehicles ADD COLUMN min_session_battery_kwh REAL NOT NULL DEFAULT 0;
ALTER TABLE vehicles ADD COLUMN max_session_battery_kwh REAL NOT NULL DEFAULT 0;

-- Backfill from completed sessions
UPDATE vehicles SET
    min_session_battery_kwh = COALESCE(
        (SELECT MIN(battery_kwh) FROM charge_sessions
         WHERE vehicle_id = vehicles.id AND status = 'completed' AND battery_kwh IS NOT NULL),
        0
    ),
    max_session_battery_kwh = COALESCE(
        (SELECT MAX(battery_kwh) FROM charge_sessions
         WHERE vehicle_id = vehicles.id AND status = 'completed' AND battery_kwh IS NOT NULL),
        0
    );
