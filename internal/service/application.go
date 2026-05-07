package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

func routeStarted(r *domain.Route) bool {
	return r.LeavingAt != nil && r.LeavingAt.Before(time.Now())
}

func toSearchStops(stops []domain.ApplicationStopInput) []domain.SearchStop {
	out := make([]domain.SearchStop, len(stops))
	for i, s := range stops {
		out[i] = domain.SearchStop{Lat: s.Lat, Lng: s.Lng}
	}
	return out
}

func appStopsDeviation(route *domain.Route, stops []domain.ApplicationStopInput) float64 {
	return deviationForStops(
		route.StartLat, route.StartLng,
		route.EndLat, route.EndLng,
		route.Stops,
		domain.SearchRouteInput{Stops: toSearchStops(stops)},
		false,
	)
}

func appStopsPastEnd(route *domain.Route, stops []domain.ApplicationStopInput) bool {
	return deviationForStops(
		route.StartLat, route.StartLng,
		route.EndLat, route.EndLng,
		route.Stops,
		domain.SearchRouteInput{Stops: toSearchStops(stops)},
		true,
	) >= math.MaxFloat64/2
}

func checkAppStops(route *domain.Route, stops []domain.ApplicationStopInput) error {
	if len(stops) == 0 {
		return nil
	}
	if appStopsPastEnd(route, stops) {
		return errs.ErrStopPastRouteEnd
	}
	if appStopsDeviation(route, stops) > route.MaxDeviation {
		return errs.ErrStopTooFarFromRoute
	}
	return nil
}

type ApplicationService struct {
	apps   domain.ApplicationRepository
	routes domain.RouteRepository
}

func NewApplicationService(apps domain.ApplicationRepository, routes domain.RouteRepository) *ApplicationService {
	return &ApplicationService{apps: apps, routes: routes}
}

func (s *ApplicationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Application, error) {
	return s.apps.GetByID(ctx, id)
}

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

	if err := checkAppStops(route, in.Stops); err != nil {
		return uuid.Nil, err
	}

	return s.apps.Create(ctx, userID, routeID, in)
}

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

func (s *ApplicationService) GetMyForRoute(ctx context.Context, userID, routeID uuid.UUID) (*domain.Application, error) {
	return s.apps.GetByUserAndRoute(ctx, userID, routeID)
}

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

	if err := checkAppStops(route, stops); err != nil {
		return err
	}

	return s.apps.UpdateStops(ctx, appID, stops, comment)
}

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

	if err := checkAppStops(route, stops); err != nil {
		return err
	}

	return s.apps.RequestStopChange(ctx, appID, stops, comment)
}

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
