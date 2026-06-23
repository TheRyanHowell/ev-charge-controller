package repository

import (
	"context"
	"database/sql"
	"errors"

	"ev-charge-controller/api/models"

	"github.com/google/uuid"
)

type RefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	token.ID = uuid.New().String()
	query := `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
	          VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`
	_, err := r.db.ExecContext(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt)
	return err
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
	          FROM refresh_tokens WHERE token_hash = ?`
	var tok models.RefreshToken
	err := r.db.QueryRowContext(ctx, query, hash).Scan(
		&tok.ID, &tok.UserID, &tok.TokenHash, &tok.ExpiresAt,
		newNullTime(&tok.RevokedAt), &tok.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}
