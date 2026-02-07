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
	query := `INSERT INTO users (id, email, name, provider, provider_sub) VALUES (?, ?, ?, ?, ?) AS u
		ON DUPLICATE KEY UPDATE
		  name = COALESCE(u.name, users.name),
		  provider = u.provider,
		  provider_sub = u.provider_sub;
		SELECT id FROM users WHERE provider_sub = ?, provider = ?`
	args := []interface{}{idStr, email, nullStr(name), provider, providerSub, providerSub, provider}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user upsert: %w", err)
	}
	defer rows.Close()

	if !rows.NextResultSet() {
		return id, nil
	}
	var selected string
	if rows.Next() {
		rows.Scan(&selected)
		if selected != "" {
			id, _ = uuid.Parse(selected)
		}
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, fmt.Errorf("user upsert result: %w", err)
	}
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
