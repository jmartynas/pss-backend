package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const DefaultSessionMaxAge = 7 * 24 * time.Hour

// Session holds the data stored in the sessions table.
type Session struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// SessionRepository is the persistence contract for sessions.
type SessionRepository interface {
	Create(ctx context.Context, userID uuid.UUID, maxAge time.Duration) (uuid.UUID, error)
	GetByToken(ctx context.Context, token string) (*Session, error)
	ExtendExpiry(ctx context.Context, token string, maxAge time.Duration) error
	DeleteByToken(ctx context.Context, token string) error
}
