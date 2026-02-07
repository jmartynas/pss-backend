package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
)

// UserService contains all user business logic.
type UserService struct {
	users   domain.UserRepository
	reviews domain.ReviewRepository
}

// NewUserService creates a UserService backed by the given repositories.
func NewUserService(users domain.UserRepository, reviews domain.ReviewRepository) *UserService {
	return &UserService{users: users, reviews: reviews}
}

func (s *UserService) GetProfile(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *UserService) UpdateProfile(ctx context.Context, id uuid.UUID, name *string) error {
	return s.users.UpdateName(ctx, id, name)
}

func (s *UserService) DisableAccount(ctx context.Context, id uuid.UUID) error {
	return s.users.Disable(ctx, id)
}

// GetPublicProfile returns a user's public info together with reviews written about them.
func (s *UserService) GetPublicProfile(ctx context.Context, id uuid.UUID) (*domain.User, []domain.Review, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	reviews, err := s.reviews.GetByTargetUser(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return user, reviews, nil
}
