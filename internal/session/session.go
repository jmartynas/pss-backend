package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const DefaultMaxAge = 7 * 24 * time.Hour

func Create(ctx context.Context, db *sql.DB, userID uuid.UUID, maxAge time.Duration) (sessionID uuid.UUID, err error) {
	if maxAge <= 0 {
		maxAge = DefaultMaxAge
	}
	sessionID = uuid.New()
	expiresAt := time.Now().Add(maxAge)
	_, err = db.ExecContext(ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		sessionID.String(), userID.String(), expiresAt)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert session: %w", err)
	}
	return sessionID, nil
}

type Row struct {
	UserID   uuid.UUID
	ExpiresAt time.Time
}

func GetByToken(ctx context.Context, db *sql.DB, token string) (*Row, error) {
	var r Row
	var userIDStr string
	err := db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token = ? AND expires_at > NOW()`,
		token,
	).Scan(&userIDStr, &r.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("get session by token: %w", err)
	}
	r.UserID, _ = uuid.Parse(userIDStr)
	return &r, nil
}

func ExtendExpiry(ctx context.Context, db *sql.DB, token string, maxAge time.Duration) error {
	if maxAge <= 0 {
		maxAge = DefaultMaxAge
	}
	newExpires := time.Now().Add(maxAge)
	_, err := db.ExecContext(ctx, `UPDATE sessions SET expires_at = ? WHERE token = ? AND expires_at > NOW()`,
		newExpires, token)
	if err != nil {
		return fmt.Errorf("extend session expiry: %w", err)
	}
	return nil
}

func DeleteByToken(ctx context.Context, db *sql.DB, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("delete session by token: %w", err)
	}
	return nil
}
