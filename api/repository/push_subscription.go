package repository

import (
	"context"
	"database/sql"
	"errors"

	"ev-charge-controller/api/models"
)

var ErrPushSubscriptionNoUserID = errors.New("push subscription requires user ID")

type PushSubscriptionRepository struct {
	db *sql.DB
}

func NewPushSubscriptionRepository(db *sql.DB) *PushSubscriptionRepository {
	return &PushSubscriptionRepository{db: db}
}

func (r *PushSubscriptionRepository) Upsert(ctx context.Context, sub *models.PushSubscription) error {
	if sub.UserID == nil {
		return ErrPushSubscriptionNoUserID
	}
	query := `INSERT INTO push_subscriptions (id, endpoint, p256dh_key, auth_key, user_id)
	          VALUES (?, ?, ?, ?, ?)
	          ON CONFLICT(endpoint, p256dh_key) DO UPDATE SET
	            id=excluded.id, auth_key=excluded.auth_key, user_id=excluded.user_id`
	_, err := r.db.ExecContext(ctx, query, sub.ID, sub.Endpoint, sub.P256dhKey, sub.AuthKey, *sub.UserID)
	return err
}

func (r *PushSubscriptionRepository) RemoveByEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

func (r *PushSubscriptionRepository) GetAll(ctx context.Context) ([]models.PushSubscription, error) {
	wc, wa := whereUserClause(ctx)
	query := `SELECT id, endpoint, p256dh_key, auth_key, created_at FROM push_subscriptions` + wc
	rows, err := r.db.QueryContext(ctx, query, wa...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []models.PushSubscription
	for rows.Next() {
		var sub models.PushSubscription
		err := rows.Scan(&sub.ID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey, &sub.CreatedAt)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}

	return subs, rows.Err()
}
