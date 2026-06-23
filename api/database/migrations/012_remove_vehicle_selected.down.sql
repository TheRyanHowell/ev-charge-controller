-- Restore the selected column for rollback.
DROP INDEX IF EXISTS idx_vehicles_user_name;
ALTER TABLE vehicles ADD COLUMN selected INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_vehicles_selected ON vehicles(selected);
