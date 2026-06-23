-- Add user_id to vehicles (nullable; back-fill assigns bootstrap user)
ALTER TABLE vehicles ADD COLUMN user_id TEXT REFERENCES users(id);

-- Add user_id to push_subscriptions
ALTER TABLE push_subscriptions ADD COLUMN user_id TEXT REFERENCES users(id);

-- Add user_id and plug_id to charge_sessions
ALTER TABLE charge_sessions ADD COLUMN user_id TEXT REFERENCES users(id);
ALTER TABLE charge_sessions ADD COLUMN plug_id TEXT REFERENCES plugs(id);

-- Rebuild schedules for per-plug structure.
-- Existing singleton row migrated with NULL plug_id/user_id; back-fill assigns them.
CREATE TABLE schedules_new (
  id TEXT PRIMARY KEY,
  plug_id TEXT UNIQUE REFERENCES plugs(id) ON DELETE CASCADE,
  user_id TEXT REFERENCES users(id),
  time TEXT NOT NULL DEFAULT '00:00',
  enabled INTEGER NOT NULL DEFAULT 0
);
INSERT INTO schedules_new (id, time, enabled)
  SELECT id, time, enabled FROM schedules;
DROP TABLE schedules;
ALTER TABLE schedules_new RENAME TO schedules;

CREATE INDEX idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX idx_push_subscriptions_user_id ON push_subscriptions(user_id);
CREATE INDEX idx_charge_sessions_user_id ON charge_sessions(user_id);
CREATE INDEX idx_charge_sessions_plug_id ON charge_sessions(plug_id);
CREATE INDEX idx_schedules_plug_id ON schedules(plug_id);
