package repository

import (
	"context"
	"database/sql"
	"errors"

	"ev-charge-controller/api/models"
)

type ScheduleRepository struct {
	db *sql.DB
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Get(ctx context.Context) (*models.Schedule, error) {
	wc, wa := whereUserClause(ctx)
	query := `SELECT id, plug_id, user_id, type, time, window_start, window_end, ready_by, enabled FROM schedules` + wc + ` LIMIT 1`
	var s models.Schedule
	var schedType string
	err := r.db.QueryRowContext(ctx, query, wa...).Scan(
		&s.ID, newNullString(&s.PlugID), newNullString(&s.UserID),
		&schedType, &s.Time, newNullString(&s.WindowStart), newNullString(&s.WindowEnd),
		newNullString(&s.ReadyBy), &s.Enabled,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Type = models.ScheduleType(schedType)
	return &s, nil
}

func (r *ScheduleRepository) Upsert(ctx context.Context, schedule *models.Schedule) error {
	const upsertSQL = `INSERT INTO schedules (id, plug_id, user_id, type, time, window_start, window_end, ready_by, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			plug_id = excluded.plug_id,
			user_id = excluded.user_id,
			type = excluded.type,
			time = excluded.time,
			window_start = excluded.window_start,
			window_end = excluded.window_end,
			ready_by = excluded.ready_by,
			enabled = excluded.enabled`
	_, err := r.db.ExecContext(ctx, upsertSQL,
		schedule.ID, toNullString(schedule.PlugID), toNullString(schedule.UserID),
		string(schedule.Type), schedule.Time,
		toNullString(schedule.WindowStart), toNullString(schedule.WindowEnd),
		toNullString(schedule.ReadyBy),
		schedule.Enabled)
	return err
}

func (r *ScheduleRepository) GetByPlugID(ctx context.Context, plugID string) (*models.Schedule, error) {
	var s models.Schedule
	var schedType string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, plug_id, user_id, type, time, window_start, window_end, ready_by, enabled FROM schedules WHERE plug_id = ?`, plugID,
	).Scan(&s.ID, newNullString(&s.PlugID), newNullString(&s.UserID),
		&schedType, &s.Time, newNullString(&s.WindowStart), newNullString(&s.WindowEnd),
		newNullString(&s.ReadyBy), &s.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Type = models.ScheduleType(schedType)
	return &s, nil
}

func (r *ScheduleRepository) UpsertByPlugID(ctx context.Context, schedule *models.Schedule) error {
	const q = `INSERT INTO schedules (id, plug_id, user_id, type, time, window_start, window_end, ready_by, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plug_id) DO UPDATE SET
			type = excluded.type,
			time = excluded.time,
			window_start = excluded.window_start,
			window_end = excluded.window_end,
			ready_by = excluded.ready_by,
			enabled = excluded.enabled`
	_, err := r.db.ExecContext(ctx, q,
		schedule.ID, toNullString(schedule.PlugID), toNullString(schedule.UserID),
		string(schedule.Type), schedule.Time,
		toNullString(schedule.WindowStart), toNullString(schedule.WindowEnd),
		toNullString(schedule.ReadyBy),
		schedule.Enabled)
	return err
}

// ListAll returns all schedules with their associated plug IDs (for schedule activator).
func (r *ScheduleRepository) ListAll(ctx context.Context) ([]models.Schedule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, plug_id, user_id, type, time, window_start, window_end, ready_by, enabled FROM schedules WHERE plug_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var schedType string
		if err := rows.Scan(&s.ID, newNullString(&s.PlugID), newNullString(&s.UserID),
			&schedType, &s.Time, newNullString(&s.WindowStart), newNullString(&s.WindowEnd),
			newNullString(&s.ReadyBy), &s.Enabled); err != nil {
			return nil, err
		}
		s.Type = models.ScheduleType(schedType)
		out = append(out, s)
	}
	return out, rows.Err()
}
