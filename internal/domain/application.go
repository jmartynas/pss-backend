package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ApplicationStop struct {
	ID               uuid.UUID `json:"id"`
	Position         uint      `json:"position"`
	Lat              float64   `json:"lat"`
	Lng              float64   `json:"lng"`
	PlaceID          *string   `json:"place_id,omitempty"`
	FormattedAddress *string   `json:"formatted_address,omitempty"`
	RouteStopID      *string   `json:"route_stop_id,omitempty"`
}

type Application struct {
	ID                uuid.UUID         `json:"id"`
	UserID            uuid.UUID         `json:"user_id"`
	UserName          string            `json:"user_name"`
	RouteID           uuid.UUID         `json:"route_id"`
	Status            string            `json:"status"`
	Comment           *string           `json:"comment,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	Stops             []ApplicationStop `json:"stops"`
	PendingStopChange bool              `json:"pending_stop_change"`
	RouteLeavingAt    *time.Time        `json:"route_leaving_at,omitempty"`
	RouteStartAddress *string           `json:"route_start_address,omitempty"`
	RouteEndAddress   *string           `json:"route_end_address,omitempty"`
}

type ApplicationStopInput struct {
	Position         uint    `json:"position"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	PlaceID          *string `json:"place_id"`
	FormattedAddress *string `json:"formatted_address"`
	RouteStopID      *string `json:"route_stop_id"`
}

type ApplyInput struct {
	Comment *string                `json:"comment"`
	Stops   []ApplicationStopInput `json:"stops"`
}

type ApplicationRepository interface {
	Create(ctx context.Context, userID, routeID uuid.UUID, in ApplyInput) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Application, error)
	GetByUserAndRoute(ctx context.Context, userID, routeID uuid.UUID) (*Application, error)
	ListByRoute(ctx context.Context, routeID uuid.UUID) ([]Application, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Application, error)
	ReviewUpdate(ctx context.Context, id uuid.UUID, status string, appUserID, routeID uuid.UUID) error
	UpdateStops(ctx context.Context, id uuid.UUID, stops []ApplicationStopInput, comment *string) error
	RequestStopChange(ctx context.Context, id uuid.UUID, stops []ApplicationStopInput, comment *string) error
	ReviewStopChange(ctx context.Context, id uuid.UUID, routeID uuid.UUID, approve bool) error
	CancelStopChange(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID, wasApproved bool) error
}
