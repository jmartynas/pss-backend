package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Vehicle struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Make        string    `json:"make"`
	Model       string    `json:"model"`
	PlateNumber string    `json:"plate_number"`
	Seats       uint      `json:"seats"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateVehicleInput struct {
	Make        string `json:"make"`
	Model       string `json:"model"`
	PlateNumber string `json:"plate_number"`
	Seats       uint   `json:"seats"`
}

type UpdateVehicleInput struct {
	Make        string `json:"make"`
	Model       string `json:"model"`
	PlateNumber string `json:"plate_number"`
	Seats       uint   `json:"seats"`
}

type VehicleRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Vehicle, error)
	Create(ctx context.Context, userID uuid.UUID, in CreateVehicleInput) (uuid.UUID, error)
	Update(ctx context.Context, id, userID uuid.UUID, in UpdateVehicleInput) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}
