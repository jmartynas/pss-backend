package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

type userRepository struct{ db *sql.DB }

// NewUserRepository returns a domain.UserRepository backed by MySQL.
func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Upsert(ctx context.Context, email, name, provider, providerSub string) (uuid.UUID, error) {
	if email == "" {
		return uuid.Nil, errors.New("user upsert: email is required")
	}
	id := uuid.New()
	_, err := sq.Insert("users").
		Columns("id", "email", "name", "provider", "provider_sub").
		Values(id.String(), email, nullableStr(name), provider, providerSub).
		Suffix("ON DUPLICATE KEY UPDATE email = email, name = IF(name IS NULL AND VALUES(name) IS NOT NULL, VALUES(name), name)").
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user upsert: %w", err)
	}

	var idStr string
	err = sq.Select("id").
		From("users").
		Where(sq.Eq{"provider_sub": providerSub, "provider": provider}).
		RunWith(r.db).QueryRowContext(ctx).Scan(&idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user upsert get id: %w", err)
	}
	parsed, _ := uuid.Parse(idStr)
	return parsed, nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	var idStr string
	err := sq.Select(
		"id", "email", "COALESCE(name, '')", "provider", "provider_sub",
		"COALESCE(status, '')", "created_at", "updated_at",
	).
		From("users").
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).QueryRowContext(ctx).
		Scan(&idStr, &u.Email, &u.Name, &u.Provider, &u.ProviderSub, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("user get by id: %w", err)
	}
	u.ID, _ = uuid.Parse(idStr)
	return &u, nil
}

func (r *userRepository) UpdateName(ctx context.Context, id uuid.UUID, name *string) error {
	if name == nil {
		return nil
	}
	_, err := sq.Update("users").
		Set("name", nullableStr(*name)).
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("user update name: %w", err)
	}
	return nil
}

func (r *userRepository) Disable(ctx context.Context, id uuid.UUID) error {
	// Anonymise the two unique-keyed columns (email, provider+provider_sub) so
	// that a future login with the same OAuth credentials creates a fresh row
	// rather than re-activating this one.
	_, err := sq.Update("users").
		Set("status", "inactive").
		Set("provider_sub", "disabled_"+id.String()).
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("user disable: %w", err)
	}
	return nil
}

func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
