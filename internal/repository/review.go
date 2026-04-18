package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

func (r *reviewRepository) GetAverageRatings(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]domain.ReviewSummary, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]domain.ReviewSummary{}, nil
	}
	ids := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		ids[i] = id.String()
	}
	rows, err := sq.Select("target_user_id", "AVG(rating)", "COUNT(*)").
		From("reviews").
		Where(sq.Eq{"target_user_id": ids}).
		GroupBy("target_user_id").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("review get average ratings: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]domain.ReviewSummary)
	for rows.Next() {
		var idStr string
		var avg float64
		var count int
		if err := rows.Scan(&idStr, &avg, &count); err != nil {
			return nil, fmt.Errorf("review avg scan: %w", err)
		}
		id, _ := uuid.Parse(idStr)
		out[id] = domain.ReviewSummary{Avg: avg, Count: count}
	}
	return out, rows.Err()
}

type reviewRepository struct{ db *sql.DB }

// NewReviewRepository returns a domain.ReviewRepository backed by MySQL.
func NewReviewRepository(db *sql.DB) domain.ReviewRepository {
	return &reviewRepository{db: db}
}

func (r *reviewRepository) Create(ctx context.Context, in domain.CreateReviewInput) (uuid.UUID, error) {
	id := uuid.New()
	_, err := sq.Insert("reviews").
		Columns("id", "author_user_id", "target_user_id", "route_id", "rating", "comment").
		Values(id.String(), in.AuthorID.String(), in.TargetID.String(), in.RouteID.String(), in.Rating, sql.NullString{String: in.Comment, Valid: in.Comment != ""}).
		RunWith(r.db).ExecContext(ctx)
	if err != nil {
		var mysqlErr *mysqldrv.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return uuid.Nil, errs.ErrAlreadyReviewed
		}
		return uuid.Nil, fmt.Errorf("review create: %w", err)
	}
	return id, nil
}

func (r *reviewRepository) GetByAuthorAndRoute(ctx context.Context, authorID, routeID uuid.UUID) ([]domain.Review, error) {
	rows, err := sq.Select("rv.id", "rv.author_user_id", "COALESCE(u.name, u.email)", "rv.target_user_id", "rv.route_id", "rv.rating", "COALESCE(rv.comment, '')", "rv.created_at").
		From("reviews rv").
		Join("users u ON u.id = rv.author_user_id").
		Where(sq.Eq{"rv.author_user_id": authorID.String(), "rv.route_id": routeID.String()}).
		OrderBy("rv.created_at DESC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("review get by author and route: %w", err)
	}
	defer rows.Close()

	var out []domain.Review
	for rows.Next() {
		var rv domain.Review
		var idStr, authorIDStr, targetIDStr, routeIDStr string
		if err := rows.Scan(&idStr, &authorIDStr, &rv.AuthorName, &targetIDStr, &routeIDStr, &rv.Rating, &rv.Comment, &rv.CreatedAt); err != nil {
			return nil, fmt.Errorf("review scan: %w", err)
		}
		rv.ID, _ = uuid.Parse(idStr)
		rv.AuthorID, _ = uuid.Parse(authorIDStr)
		rv.TargetID, _ = uuid.Parse(targetIDStr)
		rv.RouteID, _ = uuid.Parse(routeIDStr)
		out = append(out, rv)
	}
	return out, rows.Err()
}

func (r *reviewRepository) GetByTargetUser(ctx context.Context, userID uuid.UUID) ([]domain.Review, error) {
	rows, err := sq.Select(
		"rv.id", "rv.author_user_id", "COALESCE(u.name, u.email)", "rv.target_user_id",
		"rv.route_id", "rv.rating", "COALESCE(rv.comment, '')", "rv.created_at",
	).
		From("reviews rv").
		Join("users u ON u.id = rv.author_user_id").
		Where(sq.Eq{"rv.target_user_id": userID.String()}).
		OrderBy("rv.created_at DESC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("review get by target user: %w", err)
	}
	defer rows.Close()

	var out []domain.Review
	for rows.Next() {
		var rv domain.Review
		var idStr, authorIDStr, targetIDStr, routeIDStr string
		if err := rows.Scan(
			&idStr, &authorIDStr, &rv.AuthorName, &targetIDStr,
			&routeIDStr, &rv.Rating, &rv.Comment, &rv.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("review scan: %w", err)
		}
		rv.ID, _ = uuid.Parse(idStr)
		rv.AuthorID, _ = uuid.Parse(authorIDStr)
		rv.TargetID, _ = uuid.Parse(targetIDStr)
		rv.RouteID, _ = uuid.Parse(routeIDStr)
		out = append(out, rv)
	}
	return out, rows.Err()
}
