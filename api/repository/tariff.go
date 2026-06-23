package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ev-charge-controller/api/models"

	"github.com/google/uuid"
)

// TariffRepository persists per-user electricity tariffs (base rate + off-peak windows).
type TariffRepository struct {
	db *sql.DB
}

func NewTariffRepository(db *sql.DB) *TariffRepository {
	return &TariffRepository{db: db}
}

// GetByUserID returns the user's tariff, or nil when none has been configured.
func (r *TariffRepository) GetByUserID(ctx context.Context, userID string) (*models.TariffSettings, error) {
	var t models.TariffSettings
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT base_rate_pence, updated_at FROM tariff_settings WHERE user_id = ?`, userID,
	).Scan(&t.BaseRatePence, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.UpdatedAt = &updatedAt

	rows, err := r.db.QueryContext(ctx,
		`SELECT start_hhmm, end_hhmm, rate_pence FROM tariff_off_peak_windows
		 WHERE user_id = ? ORDER BY position`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	t.OffPeakWindows = []models.OffPeakWindow{}
	for rows.Next() {
		var w models.OffPeakWindow
		if err := rows.Scan(&w.Start, &w.End, &w.RatePence); err != nil {
			return nil, err
		}
		t.OffPeakWindows = append(t.OffPeakWindows, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &t, nil
}

// Upsert replaces the user's tariff: the base rate is upserted and the off-peak
// windows are replaced wholesale, all within a single transaction.
func (r *TariffRepository) Upsert(ctx context.Context, userID string, settings *models.TariffSettings) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO tariff_settings (id, user_id, base_rate_pence, created_at, updated_at)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET
			base_rate_pence = excluded.base_rate_pence,
			updated_at = CURRENT_TIMESTAMP`,
		uuid.New().String(), userID, settings.BaseRatePence,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM tariff_off_peak_windows WHERE user_id = ?`, userID,
	); err != nil {
		return err
	}

	for i, w := range settings.OffPeakWindows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tariff_off_peak_windows (id, user_id, position, start_hhmm, end_hhmm, rate_pence)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), userID, i, w.Start, w.End, w.RatePence,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
