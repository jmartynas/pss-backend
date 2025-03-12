package db

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/bxcodec/dbresolver/v2"
	"github.com/jmartynas/pss-backend/structs"
)

var ErrNotFound = sql.ErrNoRows

func SelectPassword(
	ctx context.Context,
	dbc dbresolver.DB,
	email string,
) ([]byte, error) {
	var hashedPassword string

	if err := squirrel.Select("users.password_hash").
		From("users").
		Where(squirrel.Eq{"LOWER(users.email)": email}).
		RunWith(dbc).
		QueryRowContext(ctx).
		Scan(&hashedPassword); err != nil {
		return []byte{}, err
	}

	return []byte(hashedPassword), nil
}

func SelectUser(ctx context.Context, dbc dbresolver.DB, email string) (*structs.User, error) {
	var user *structs.User
	err := squirrel.Select("users.email, users.google_id, users.name").
		From("users").
		Where(squirrel.Eq{"users.email": email}).
		RunWith(dbc).
		QueryRowContext(ctx).
		Scan(&user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func InsertUser(ctx context.Context, dbc dbresolver.DB, user *structs.User) error {
	_, err := squirrel.Insert("users").
		SetMap(map[string]any{
			"email":         user.Email,
			"google_id":     user.GoogleID,
			"name":          user.Name,
			"password_hash": user.Password,
		}).
		RunWith(dbc).
		ExecContext(ctx)

	return err
}
