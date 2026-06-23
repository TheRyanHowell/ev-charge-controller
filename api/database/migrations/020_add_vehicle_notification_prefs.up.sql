ALTER TABLE vehicles ADD COLUMN notify_charge_complete INTEGER NOT NULL DEFAULT 1;
ALTER TABLE vehicles ADD COLUMN notify_charger_offline INTEGER NOT NULL DEFAULT 1;
ALTER TABLE vehicles ADD COLUMN notify_maintenance_offline INTEGER NOT NULL DEFAULT 1;
