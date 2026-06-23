-- Per-user electricity tariff: one base rate plus zero or more off-peak windows.
CREATE TABLE tariff_settings (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL UNIQUE,
  base_rate_pence REAL NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Off-peak windows for a user's tariff. Replaced wholesale on every update.
-- start_hhmm/end_hhmm are local "HH:MM"; end < start means the window wraps midnight.
CREATE TABLE tariff_off_peak_windows (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  start_hhmm TEXT NOT NULL,
  end_hhmm TEXT NOT NULL,
  rate_pence REAL NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_tariff_settings_user_id ON tariff_settings(user_id);
CREATE INDEX idx_tariff_off_peak_windows_user_id ON tariff_off_peak_windows(user_id);

-- Frozen cost computed once at session completion (time-weighted across tariff
-- windows). off_peak_kwh is the wall-side energy billed at an off-peak rate;
-- on-peak energy = wall_kwh - off_peak_kwh.
ALTER TABLE charge_sessions ADD COLUMN cost_pence REAL;
ALTER TABLE charge_sessions ADD COLUMN off_peak_kwh REAL;

-- Backfill existing completed sessions at the legacy flat default rate (24.83p/kWh);
-- no off-peak windows existed historically, so all energy is on-peak.
UPDATE charge_sessions
SET cost_pence = wall_kwh * 24.83, off_peak_kwh = 0
WHERE status = 'completed' AND wall_kwh IS NOT NULL;

-- Precomputed lifetime cost per vehicle, mirroring the other lifetime stat columns.
ALTER TABLE vehicles ADD COLUMN total_cost_pence REAL NOT NULL DEFAULT 0;
UPDATE vehicles SET total_cost_pence = (
  SELECT COALESCE(SUM(cost_pence), 0) FROM charge_sessions
  WHERE charge_sessions.vehicle_id = vehicles.id
    AND charge_sessions.status = 'completed'
    AND charge_sessions.cost_pence IS NOT NULL
);
