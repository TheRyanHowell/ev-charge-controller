-- Remove the unused selected column and its index.
-- Vehicle selection is managed through plug assignment (plugs.vehicle_id),
-- not through a per-vehicle selected flag.
DROP INDEX IF EXISTS idx_vehicles_selected;
ALTER TABLE vehicles DROP COLUMN selected;

-- Enforce unique nickname per user: a user cannot have two vehicles with the same name.
CREATE UNIQUE INDEX idx_vehicles_user_name ON vehicles(user_id, name);
