package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ApplicationStop is a passenger's proposed pickup/dropoff stop.
type ApplicationStop struct {
	ID               uuid.UUID `json:"id"`
	Position         uint      `json:"position"`
	Lat              float64   `json:"lat"`
	Lng              float64   `json:"lng"`
	PlaceID          *string   `json:"place_id,omitempty"`
	FormattedAddress *string   `json:"formatted_address,omitempty"`
	RouteStopID      *string   `json:"route_stop_id,omitempty"`
}

// Application is a passenger's request to join a route.
type Application struct {
	ID                 uuid.UUID         `json:"id"`
	UserID             uuid.UUID         `json:"user_id"`
	UserName           string            `json:"user_name"`
	RouteID            uuid.UUID         `json:"route_id"`
	Status             string            `json:"status"`
	Comment            *string           `json:"comment,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	Stops              []ApplicationStop `json:"stops"`
	PendingStopChange  bool              `json:"pending_stop_change"`

	// Route summary fields — populated only in ListByUser responses.
	RouteLeavingAt    *time.Time `json:"route_leaving_at,omitempty"`
	RouteStartAddress *string    `json:"route_start_address,omitempty"`
	RouteEndAddress   *string    `json:"route_end_address,omitempty"`
}

// ApplicationStopInput is a stop submitted with an application.
type ApplicationStopInput struct {
	Position         uint    `json:"position"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	PlaceID          *string `json:"place_id"`
	FormattedAddress *string `json:"formatted_address"`
	RouteStopID      *string `json:"route_stop_id"`
}

// ApplyInput carries the data for a new application.
type ApplyInput struct {
	Comment *string                `json:"comment"`
	Stops   []ApplicationStopInput `json:"stops"`
}

// ApplicationRepository is the persistence contract for applications.
type ApplicationRepository interface {
	// Create persists a new application with its stops (no business-rule checks).
	Create(ctx context.Context, userID, routeID uuid.UUID, in ApplyInput) (uuid.UUID, error)
	// GetByID loads a single application with its stops.
	GetByID(ctx context.Context, id uuid.UUID) (*Application, error)
	// GetByUserAndRoute returns the active application a user has for a route, or nil.
	GetByUserAndRoute(ctx context.Context, userID, routeID uuid.UUID) (*Application, error)
	// ListByRoute returns all active applications for a route.
	ListByRoute(ctx context.Context, routeID uuid.UUID) ([]Application, error)
	// ListByUser returns all active applications submitted by a user.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Application, error)
	// ReviewUpdate changes status to approved/rejected and handles the downstream DB
	// work (stops update, participant insertion) inside a transaction.
	ReviewUpdate(ctx context.Context, id uuid.UUID, status string, appUserID, routeID uuid.UUID) error
	// UpdateStops replaces the request_stops and optionally updates the comment for a pending application.
	UpdateStops(ctx context.Context, id uuid.UUID, stops []ApplicationStopInput, comment *string) error
	// RequestStopChange stores new proposed stops (and optional comment) and flags pending_stop_change on an approved application.
	RequestStopChange(ctx context.Context, id uuid.UUID, stops []ApplicationStopInput, comment *string) error
	// ReviewStopChange approves or rejects a pending stop-change request.
	// On approve: copies new request_stops into route_stops and clears the flag.
	// On reject: discards the proposed request_stops and clears the flag.
	ReviewStopChange(ctx context.Context, id uuid.UUID, routeID uuid.UUID, approve bool) error
	// CancelStopChange lets the applicant withdraw a pending stop-change request.
	CancelStopChange(ctx context.Context, id uuid.UUID) error
	// SoftDelete marks an application deleted and optionally removes the participant record.
	SoftDelete(ctx context.Context, id uuid.UUID, wasApproved bool) error
}
