-- SQLite can't drop columns; reset type to 'daily' for all rows.
UPDATE schedules SET type = 'daily', window_start = NULL, window_end = NULL;
