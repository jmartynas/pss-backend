package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
)

type sessionRepository struct{ db *sql.DB }

// NewSessionRepository returns a domain.SessionRepository backed by MySQL.
func NewSessionRepository(db *sql.DB) domain.SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) Create(ctx context.Context, userID uuid.UUID, maxAge time.Duration) (uuid.UUID, error) {
	if maxAge <= 0 {
		maxAge = domain.DefaultSessionMaxAge
	}
	id := uuid.New()
	expiresAt := time.Now().Add(maxAge)
	_, err := sq.Insert("sessions").
		Columns("token", "user_id", "expires_at").
		Values(id.String(), userID.String(), expiresAt).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("session create: %w", err)
	}
	return id, nil
}

func (r *sessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	var s domain.Session
	var userIDStr string
	err := sq.Select("user_id", "expires_at").
		From("sessions").
		Where(sq.And{
			sq.Eq{"token": token},
			sq.Expr("expires_at > NOW()"),
		}).
		RunWith(r.db).QueryRowContext(ctx).
		Scan(&userIDStr, &s.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("session get: %w", err)
	}
	s.UserID, _ = uuid.Parse(userIDStr)
	return &s, nil
}

func (r *sessionRepository) ExtendExpiry(ctx context.Context, token string, maxAge time.Duration) error {
	if maxAge <= 0 {
		maxAge = domain.DefaultSessionMaxAge
	}
	_, err := sq.Update("sessions").
		Set("expires_at", time.Now().Add(maxAge)).
		Where(sq.And{
			sq.Eq{"token": token},
			sq.Expr("expires_at > NOW()"),
		}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("session extend: %w", err)
	}
	return nil
}

func (r *sessionRepository) DeleteByToken(ctx context.Context, token string) error {
	_, err := sq.Delete("sessions").
		Where(sq.Eq{"token": token}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}
