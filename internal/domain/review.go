package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

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

type ReviewSummary struct {
	Avg   float64
	Count int
}

type CreateReviewInput struct {
	AuthorID uuid.UUID
	TargetID uuid.UUID
	RouteID  uuid.UUID
	Rating   int
	Comment  string
}

type ReviewRepository interface {
	Create(ctx context.Context, in CreateReviewInput) (uuid.UUID, error)
	GetByTargetUser(ctx context.Context, userID uuid.UUID) ([]Review, error)
	GetByAuthorAndRoute(ctx context.Context, authorID, routeID uuid.UUID) ([]Review, error)
	GetAverageRatings(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]ReviewSummary, error)
}
