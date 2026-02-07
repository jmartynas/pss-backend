package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RouteFilter string

const (
	RouteFilterActive RouteFilter = "active"
	RouteFilterPast   RouteFilter = "past"
)

// Stop is an intermediate waypoint on a confirmed route.
// ParticipantID is nil for driver-owned stops, non-nil for passenger stops.
type Stop struct {
	ID               uuid.UUID `json:"id"`
	Position         uint      `json:"position"`
	Lat              float64   `json:"lat"`
	Lng              float64   `json:"lng"`
	PlaceID          *string   `json:"place_id"`
	FormattedAddress *string   `json:"formatted_address"`
	ParticipantID    *string   `json:"participant_id,omitempty"`
}

// Participant is a confirmed passenger or driver on a route.
type Participant struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
}

// Route is the full route entity returned to clients.
type Route struct {
	ID                    uuid.UUID     `json:"id"`
	CreatorID             uuid.UUID     `json:"creator_id"`
	CreatorName           string        `json:"creator_name"`
	VehicleID             *uuid.UUID    `json:"vehicle_id,omitempty"`
	Description           *string       `json:"description"`
	StartLat              float64       `json:"start_lat"`
	StartLng              float64       `json:"start_lng"`
	StartPlaceID          *string       `json:"start_place_id"`
	StartFormattedAddress *string       `json:"start_formatted_address"`
	EndLat                float64       `json:"end_lat"`
	EndLng                float64       `json:"end_lng"`
	EndPlaceID            *string       `json:"end_place_id"`
	EndFormattedAddress   *string       `json:"end_formatted_address"`
	MaxPassengers         uint          `json:"max_passengers"`
	MaxDeviation          float64       `json:"max_deviation"`
	AvailablePassengers   uint          `json:"available_passengers"`
	Price                 *float64      `json:"price,omitempty"`
	LeavingAt             *time.Time    `json:"leaving_at"`
	Stops                 []Stop        `json:"stops"`
	Participants          []Participant `json:"participants"`
	// CreatorRating is the driver's average rating (nil when review count < 5).
	CreatorRating       *float64 `json:"creator_rating,omitempty"`
	CreatorReviewCount  int      `json:"creator_review_count"`
}

// StopInput is a waypoint provided when creating a route.
type StopInput struct {
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	PlaceID          *string `json:"place_id"`
	FormattedAddress *string `json:"formatted_address"`
}

// CreateRouteInput carries the data for a new route.
type CreateRouteInput struct {
	VehicleID             *uuid.UUID  `json:"vehicle_id"`
	Description           *string     `json:"description"`
	StartLat              float64     `json:"start_lat"`
	StartLng              float64     `json:"start_lng"`
	StartPlaceID          *string     `json:"start_place_id"`
	StartFormattedAddress *string     `json:"start_formatted_address"`
	EndLat                float64     `json:"end_lat"`
	EndLng                float64     `json:"end_lng"`
	EndPlaceID            *string     `json:"end_place_id"`
	EndFormattedAddress   *string     `json:"end_formatted_address"`
	MaxPassengers         uint        `json:"max_passengers"`
	MaxDeviation          float64     `json:"max_deviation"`
	Price                 *float64    `json:"price"`
	LeavingAt             *time.Time  `json:"leaving_at"`
	Stops                 []StopInput `json:"stops"`
}

// UpdateRouteInput carries the fields a creator may change (all optional).
type UpdateRouteInput struct {
	Description   *string    `json:"description"`
	MaxPassengers *uint      `json:"max_passengers"`
	MaxDeviation  *float64   `json:"max_deviation"`
	Price         *float64   `json:"price"`
	LeavingAt     *time.Time `json:"leaving_at"`

	// Route geometry — all four start/end fields should be provided together.
	StartLat              *float64 `json:"start_lat"`
	StartLng              *float64 `json:"start_lng"`
	StartPlaceID          *string  `json:"start_place_id"`
	StartFormattedAddress *string  `json:"start_formatted_address"`
	EndLat                *float64 `json:"end_lat"`
	EndLng                *float64 `json:"end_lng"`
	EndPlaceID            *string  `json:"end_place_id"`
	EndFormattedAddress   *string  `json:"end_formatted_address"`

	// Stops replaces all intermediate waypoints when non-nil.
	Stops *[]StopInput `json:"stops"`
}

// SearchStop is a passenger pickup/dropoff point in a search query.
type SearchStop struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// SearchRouteInput is the body for POST /routes/search.
type SearchRouteInput struct {
	StartLat float64      `json:"start_lat"`
	StartLng float64      `json:"start_lng"`
	EndLat   float64      `json:"end_lat"`
	EndLng   float64      `json:"end_lng"`
	Stops    []SearchStop `json:"stops"`
}

// RouteRepository is the persistence contract for routes.
type RouteRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Route, error)
	Create(ctx context.Context, creatorID uuid.UUID, in CreateRouteInput) (uuid.UUID, error)
	Update(ctx context.Context, id, creatorID uuid.UUID, in UpdateRouteInput) error
	Delete(ctx context.Context, id, creatorID uuid.UUID) error
	ListByCreator(ctx context.Context, creatorID uuid.UUID, filter RouteFilter) ([]Route, error)
	ListByParticipant(ctx context.Context, userID uuid.UUID, filter RouteFilter) ([]Route, error)
	// ListSearchable returns all routes that still have available seats.
	ListSearchable(ctx context.Context) ([]Route, error)
}
