package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	Email       string
	Name        string
	Provider    string
	ProviderSub string
}

func Upsert(ctx context.Context, db *sql.DB, email, name, provider, providerSub string) (id uuid.UUID, err error) {
	if email == "" {
		return uuid.Nil, errors.New("user: email is required")
	}

	id = uuid.New()
	idStr := id.String()
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, email, name, provider, provider_sub) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  name = COALESCE(VALUES(name), name),
		  provider = VALUES(provider),
		  provider_sub = VALUES(provider_sub)`,
		idStr, email, nullStr(name), provider, providerSub)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user upsert: %w", err)
	}

	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE provider_sub = ? AND provider = ?`,
		providerSub, provider).Scan(&idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user upsert: %w", err)
	}
	id, _ = uuid.Parse(idStr)
	return id, nil
}

func GetByID(ctx context.Context, db *sql.DB, id uuid.UUID) (*User, error) {
	var u User
	var idStr string
	err := db.QueryRowContext(ctx,
		`SELECT id, email, COALESCE(name, ''), provider, provider_sub FROM users WHERE id = ?`,
		id.String(),
	).Scan(&idStr, &u.Email, &u.Name, &u.Provider, &u.ProviderSub)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	u.ID, _ = uuid.Parse(idStr)
	return &u, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
