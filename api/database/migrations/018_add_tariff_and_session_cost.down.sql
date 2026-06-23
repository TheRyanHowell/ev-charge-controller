DROP INDEX IF EXISTS idx_tariff_off_peak_windows_user_id;
DROP INDEX IF EXISTS idx_tariff_settings_user_id;
DROP TABLE IF EXISTS tariff_off_peak_windows;
DROP TABLE IF EXISTS tariff_settings;

-- SQLite can't reliably drop columns across versions; clear the frozen cost instead.
UPDATE charge_sessions SET cost_pence = NULL, off_peak_kwh = NULL;
UPDATE vehicles SET total_cost_pence = 0;
