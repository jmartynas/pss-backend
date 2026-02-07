package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

func routeStarted(r *domain.Route) bool {
	return r.LeavingAt != nil && r.LeavingAt.Before(time.Now())
}

// ApplicationService contains all application business logic.
type ApplicationService struct {
	apps   domain.ApplicationRepository
	routes domain.RouteRepository
}

// NewApplicationService creates an ApplicationService backed by the given repositories.
func NewApplicationService(apps domain.ApplicationRepository, routes domain.RouteRepository) *ApplicationService {
	return &ApplicationService{apps: apps, routes: routes}
}

func (s *ApplicationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Application, error) {
	return s.apps.GetByID(ctx, id)
}

// ListByRoute returns applications for a route.
// Accessible by the route creator or any approved participant of that route.
func (s *ApplicationService) ListByRoute(ctx context.Context, routeID, callerID uuid.UUID) ([]domain.Application, error) {
	route, err := s.routes.GetByID(ctx, routeID)
	if err != nil {
		return nil, fmt.Errorf("list by route: load route: %w", err)
	}
	if route.CreatorID != callerID {
		app, err := s.apps.GetByUserAndRoute(ctx, callerID, routeID)
		if err != nil {
			return nil, fmt.Errorf("list by route: check participant: %w", err)
		}
		if app == nil || app.Status != "approved" {
			return nil, errs.ErrForbidden
		}
	}
	return s.apps.ListByRoute(ctx, routeID)
}

func (s *ApplicationService) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Application, error) {
	return s.apps.ListByUser(ctx, userID)
}

// Apply validates business rules then persists a new application.
func (s *ApplicationService) Apply(ctx context.Context, userID, routeID uuid.UUID, in domain.ApplyInput) (uuid.UUID, error) {
	route, err := s.routes.GetByID(ctx, routeID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("apply: load route: %w", err)
	}

	if routeStarted(route) {
		return uuid.Nil, errs.ErrRouteStarted
	}
	if route.CreatorID == userID {
		return uuid.Nil, errs.ErrForbidden
	}
	if route.AvailablePassengers == 0 {
		return uuid.Nil, errs.ErrRouteFull
	}

	existing, err := s.apps.GetByUserAndRoute(ctx, userID, routeID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("apply: check existing: %w", err)
	}
	if existing != nil {
		return uuid.Nil, errs.ErrAlreadyApplied
	}

	return s.apps.Create(ctx, userID, routeID, in)
}

// Review approves or rejects an application. Only the route creator may call this.
func (s *ApplicationService) Review(ctx context.Context, appID uuid.UUID, status string, callerID uuid.UUID) error {
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("review: load application: %w", err)
	}

	route, err := s.routes.GetByID(ctx, app.RouteID)
	if err != nil {
		return fmt.Errorf("review: load route: %w", err)
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	if route.CreatorID != callerID {
		return errs.ErrForbidden
	}
	if app.Status != "pending" {
		return errs.ErrConflict
	}

	return s.apps.ReviewUpdate(ctx, appID, status, app.UserID, app.RouteID)
}

// GetMyForRoute returns the caller's own application for a route, or nil if none.
func (s *ApplicationService) GetMyForRoute(ctx context.Context, userID, routeID uuid.UUID) (*domain.Application, error) {
	return s.apps.GetByUserAndRoute(ctx, userID, routeID)
}

// UpdateStops replaces the stops and optionally updates the comment on a pending application. Only the applicant may call this.
func (s *ApplicationService) UpdateStops(ctx context.Context, appID, callerID uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error {
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("update stops: load application: %w", err)
	}
	if app.UserID != callerID {
		return errs.ErrForbidden
	}
	if app.Status == "approved" || app.Status == "rejected" {
		return errs.ErrConflict
	}
	route, err := s.routes.GetByID(ctx, app.RouteID)
	if err != nil {
		return fmt.Errorf("update stops: load route: %w", err)
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	return s.apps.UpdateStops(ctx, appID, stops, comment)
}

// RequestStopChange stores new proposed stops on an approved application. Only the applicant may call this.
func (s *ApplicationService) RequestStopChange(ctx context.Context, appID, callerID uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error {
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("request stop change: load application: %w", err)
	}
	if app.UserID != callerID {
		return errs.ErrForbidden
	}
	if app.Status != "approved" {
		return errs.ErrConflict
	}
	route, err := s.routes.GetByID(ctx, app.RouteID)
	if err != nil {
		return fmt.Errorf("request stop change: load route: %w", err)
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	return s.apps.RequestStopChange(ctx, appID, stops, comment)
}

// ReviewStopChange approves or rejects a pending stop-change request. Only the route creator may call this.
func (s *ApplicationService) ReviewStopChange(ctx context.Context, appID, callerID uuid.UUID, approve bool) error {
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("review stop change: load application: %w", err)
	}
	route, err := s.routes.GetByID(ctx, app.RouteID)
	if err != nil {
		return fmt.Errorf("review stop change: load route: %w", err)
	}
	if route.CreatorID != callerID {
		return errs.ErrForbidden
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	if !app.PendingStopChange {
		return errs.ErrConflict
	}
	return s.apps.ReviewStopChange(ctx, appID, app.RouteID, approve)
}

// Cancel withdraws a pending application. Only the applicant may call this,
// and only while the application has not yet been accepted or rejected.
func (s *ApplicationService) Cancel(ctx context.Context, appID, callerID uuid.UUID) error {
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("cancel: load application: %w", err)
	}
	if app.UserID != callerID {
		return errs.ErrForbidden
	}
	if app.Status != "pending" {
		return errs.ErrConflict
	}
	route, err := s.routes.GetByID(ctx, app.RouteID)
	if err != nil {
		return fmt.Errorf("cancel: load route: %w", err)
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	return s.apps.SoftDelete(ctx, appID, false)
}

// CancelStopChange lets the applicant withdraw their pending stop-change request.
func (s *ApplicationService) CancelStopChange(ctx context.Context, appID, callerID uuid.UUID) error {
	app, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("cancel stop change: load application: %w", err)
	}
	if app.UserID != callerID {
		return errs.ErrForbidden
	}
	if !app.PendingStopChange {
		return errs.ErrConflict
	}
	route, err := s.routes.GetByID(ctx, app.RouteID)
	if err != nil {
		return fmt.Errorf("cancel stop change: load route: %w", err)
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	return s.apps.CancelStopChange(ctx, appID)
}
