ALTER TABLE schedules ADD COLUMN type TEXT NOT NULL DEFAULT 'daily';
ALTER TABLE schedules ADD COLUMN window_start TEXT;
ALTER TABLE schedules ADD COLUMN window_end TEXT;
