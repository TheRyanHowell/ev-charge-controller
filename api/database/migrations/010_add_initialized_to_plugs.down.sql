-- SQLite does not support DROP COLUMN before 3.35.0; recreate table without initialized.
CREATE TABLE plugs_backup AS SELECT id, user_id, name, namespace, mqtt_topic, vehicle_id, online, last_offline_notified_at, created_at FROM plugs;
DROP TABLE plugs;
ALTER TABLE plugs_backup RENAME TO plugs;
