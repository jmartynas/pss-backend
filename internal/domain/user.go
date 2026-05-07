package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	Email       string
	Name        string
	Provider    string
	ProviderSub string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserRepository interface {
	Upsert(ctx context.Context, email, name, provider, providerSub string) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateName(ctx context.Context, id uuid.UUID, name *string) error
	Disable(ctx context.Context, id uuid.UUID) error
}
