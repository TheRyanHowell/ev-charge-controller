-- Make namespace nullable so plugs can be created before MQTT provisioning.
-- SQLite requires table recreation to change column constraints.
-- Use partial unique index: only enforce uniqueness when namespace is non-empty.
CREATE TABLE plugs_new (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  namespace TEXT,
  mqtt_topic TEXT NOT NULL DEFAULT '',
  tls INTEGER NOT NULL DEFAULT 0,
  online INTEGER NOT NULL DEFAULT 0,
  last_seen DATETIME,
  last_offline_notified_at DATETIME,
  vehicle_id TEXT,
  initialized INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, name),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE SET NULL
);
INSERT INTO plugs_new SELECT id, user_id, name, CASE WHEN namespace = '' THEN NULL ELSE namespace END, mqtt_topic, tls, online, last_seen, last_offline_notified_at, vehicle_id, initialized, created_at FROM plugs;
DROP TABLE plugs;
ALTER TABLE plugs_new RENAME TO plugs;
CREATE UNIQUE INDEX idx_plugs_namespace ON plugs(namespace) WHERE namespace != '';
CREATE INDEX idx_plugs_user_id ON plugs(user_id);
