-- SQLite does not support DROP COLUMN; recreate the table without the added columns.
CREATE TABLE vehicles_backup AS SELECT id, user_id, model_id, name, current_percent, target_percent, created_at, total_sessions, total_battery_kwh, total_wall_kwh, total_co2_grams, total_cost_pence, last_session_at FROM vehicles;
DROP TABLE vehicles;
ALTER TABLE vehicles_backup RENAME TO vehicles;
