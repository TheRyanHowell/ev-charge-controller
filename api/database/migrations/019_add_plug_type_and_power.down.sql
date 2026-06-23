-- SQLite does not support DROP COLUMN in older versions; recreate the table.
CREATE TABLE plugs_backup AS SELECT id, user_id, name, namespace, mqtt_topic, tls, online, initialized, last_seen, last_offline_notified_at, vehicle_id, created_at FROM plugs;
DROP TABLE plugs;
ALTER TABLE plugs_backup RENAME TO plugs;
