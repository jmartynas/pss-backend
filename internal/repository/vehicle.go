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

type vehicleRepository struct{ db *sql.DB }

func NewVehicleRepository(db *sql.DB) domain.VehicleRepository {
	return &vehicleRepository{db: db}
}

func (r *vehicleRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Vehicle, error) {
	rows, err := sq.Select("id", "user_id", "COALESCE(make,'')", "model", "plate_number", "seats", "created_at").
		From("vehicles").
		Where(sq.Eq{"user_id": userID.String()}).
		OrderBy("created_at ASC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("vehicle list: %w", err)
	}
	defer rows.Close()

	var out []domain.Vehicle
	for rows.Next() {
		var v domain.Vehicle
		var idStr, userIDStr string
		if err := rows.Scan(&idStr, &userIDStr, &v.Make, &v.Model, &v.PlateNumber, &v.Seats, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("vehicle scan: %w", err)
		}
		v.ID, _ = uuid.Parse(idStr)
		v.UserID, _ = uuid.Parse(userIDStr)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vehicle list: %w", err)
	}
	if out == nil {
		out = []domain.Vehicle{}
	}
	return out, nil
}

func (r *vehicleRepository) Create(ctx context.Context, userID uuid.UUID, in domain.CreateVehicleInput) (uuid.UUID, error) {
	id := uuid.New()
	_, err := sq.Insert("vehicles").
		Columns("id", "user_id", "make", "model", "plate_number", "seats").
		Values(id.String(), userID.String(), nullableStr(in.Make), in.Model, in.PlateNumber, in.Seats).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("vehicle create: %w", err)
	}
	return id, nil
}

func (r *vehicleRepository) Update(ctx context.Context, id, userID uuid.UUID, in domain.UpdateVehicleInput) error {
	var ownerStr string
	err := sq.Select("user_id").From("vehicles").
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).QueryRowContext(ctx).Scan(&ownerStr)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("vehicle update: fetch owner: %w", err)
	}
	if ownerStr != userID.String() {
		return errs.ErrForbidden
	}
	_, err = sq.Update("vehicles").
		Set("make", nullableStr(in.Make)).
		Set("model", in.Model).
		Set("plate_number", in.PlateNumber).
		Set("seats", in.Seats).
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("vehicle update: %w", err)
	}
	return nil
}

func (r *vehicleRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	var ownerStr string
	err := sq.Select("user_id").From("vehicles").
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).QueryRowContext(ctx).Scan(&ownerStr)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("vehicle delete: fetch owner: %w", err)
	}
	if ownerStr != userID.String() {
		return errs.ErrForbidden
	}
	_, err = sq.Delete("vehicles").
		Where(sq.Eq{"id": id.String()}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("vehicle delete: %w", err)
	}
	return nil
}
