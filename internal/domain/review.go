package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Review is a rating left by one user about another after a shared ride.
type Review struct {
	ID         uuid.UUID
	AuthorID   uuid.UUID
	AuthorName string
	TargetID   uuid.UUID
	RouteID    uuid.UUID
	Rating     int
	Comment    string
	CreatedAt  time.Time
}

// ReviewSummary holds aggregated rating data for one user.
type ReviewSummary struct {
	Avg   float64
	Count int
}

// ReviewRepository is the persistence contract for reviews.
type ReviewRepository interface {
	GetByTargetUser(ctx context.Context, userID uuid.UUID) ([]Review, error)
	// GetAverageRatings returns a rating summary keyed by user ID for each
	// requested user that has at least one review.
	GetAverageRatings(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]ReviewSummary, error)
}
