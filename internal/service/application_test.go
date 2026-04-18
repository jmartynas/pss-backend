package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

// ── mock repositories ─────────────────────────────────────────────────────────

type mockRouteRepo struct {
	getByID        func(ctx context.Context, id uuid.UUID) (*domain.Route, error)
	create         func(ctx context.Context, creatorID uuid.UUID, in domain.CreateRouteInput) (uuid.UUID, error)
	update         func(ctx context.Context, id, creatorID uuid.UUID, in domain.UpdateRouteInput) error
	delete         func(ctx context.Context, id, creatorID uuid.UUID) error
	listByCreator  func(ctx context.Context, creatorID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error)
	listByParticipant func(ctx context.Context, userID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error)
	listSearchable func(ctx context.Context) ([]domain.Route, error)
}

func (m *mockRouteRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	return m.getByID(ctx, id)
}
func (m *mockRouteRepo) Create(ctx context.Context, creatorID uuid.UUID, in domain.CreateRouteInput) (uuid.UUID, error) {
	return m.create(ctx, creatorID, in)
}
func (m *mockRouteRepo) Update(ctx context.Context, id, creatorID uuid.UUID, in domain.UpdateRouteInput) error {
	return m.update(ctx, id, creatorID, in)
}
func (m *mockRouteRepo) Delete(ctx context.Context, id, creatorID uuid.UUID) error {
	return m.delete(ctx, id, creatorID)
}
func (m *mockRouteRepo) ListByCreator(ctx context.Context, creatorID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error) {
	return m.listByCreator(ctx, creatorID, filter)
}
func (m *mockRouteRepo) ListByParticipant(ctx context.Context, userID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error) {
	return m.listByParticipant(ctx, userID, filter)
}
func (m *mockRouteRepo) ListSearchable(ctx context.Context) ([]domain.Route, error) {
	return m.listSearchable(ctx)
}

type mockAppRepo struct {
	create              func(ctx context.Context, userID, routeID uuid.UUID, in domain.ApplyInput) (uuid.UUID, error)
	getByID             func(ctx context.Context, id uuid.UUID) (*domain.Application, error)
	getByUserAndRoute   func(ctx context.Context, userID, routeID uuid.UUID) (*domain.Application, error)
	listByRoute         func(ctx context.Context, routeID uuid.UUID) ([]domain.Application, error)
	listByUser          func(ctx context.Context, userID uuid.UUID) ([]domain.Application, error)
	reviewUpdate        func(ctx context.Context, id uuid.UUID, status string, appUserID, routeID uuid.UUID) error
	updateStops         func(ctx context.Context, id uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error
	requestStopChange   func(ctx context.Context, id uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error
	reviewStopChange    func(ctx context.Context, id uuid.UUID, routeID uuid.UUID, approve bool) error
	cancelStopChange    func(ctx context.Context, id uuid.UUID) error
	softDelete          func(ctx context.Context, id uuid.UUID, wasApproved bool) error
}

func (m *mockAppRepo) Create(ctx context.Context, userID, routeID uuid.UUID, in domain.ApplyInput) (uuid.UUID, error) {
	return m.create(ctx, userID, routeID, in)
}
func (m *mockAppRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Application, error) {
	return m.getByID(ctx, id)
}
func (m *mockAppRepo) GetByUserAndRoute(ctx context.Context, userID, routeID uuid.UUID) (*domain.Application, error) {
	return m.getByUserAndRoute(ctx, userID, routeID)
}
func (m *mockAppRepo) ListByRoute(ctx context.Context, routeID uuid.UUID) ([]domain.Application, error) {
	return m.listByRoute(ctx, routeID)
}
func (m *mockAppRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Application, error) {
	return m.listByUser(ctx, userID)
}
func (m *mockAppRepo) ReviewUpdate(ctx context.Context, id uuid.UUID, status string, appUserID, routeID uuid.UUID) error {
	return m.reviewUpdate(ctx, id, status, appUserID, routeID)
}
func (m *mockAppRepo) UpdateStops(ctx context.Context, id uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error {
	return m.updateStops(ctx, id, stops, comment)
}
func (m *mockAppRepo) RequestStopChange(ctx context.Context, id uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error {
	return m.requestStopChange(ctx, id, stops, comment)
}
func (m *mockAppRepo) ReviewStopChange(ctx context.Context, id uuid.UUID, routeID uuid.UUID, approve bool) error {
	return m.reviewStopChange(ctx, id, routeID, approve)
}
func (m *mockAppRepo) CancelStopChange(ctx context.Context, id uuid.UUID) error {
	return m.cancelStopChange(ctx, id)
}
func (m *mockAppRepo) SoftDelete(ctx context.Context, id uuid.UUID, wasApproved bool) error {
	return m.softDelete(ctx, id, wasApproved)
}

type mockReviewRepo struct {
	create            func(ctx context.Context, in domain.CreateReviewInput) (uuid.UUID, error)
	getByTargetUser   func(ctx context.Context, userID uuid.UUID) ([]domain.Review, error)
	getByAuthorAndRoute func(ctx context.Context, authorID, routeID uuid.UUID) ([]domain.Review, error)
	getAverageRatings func(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]domain.ReviewSummary, error)
}

func (m *mockReviewRepo) Create(ctx context.Context, in domain.CreateReviewInput) (uuid.UUID, error) {
	return m.create(ctx, in)
}
func (m *mockReviewRepo) GetByTargetUser(ctx context.Context, userID uuid.UUID) ([]domain.Review, error) {
	return m.getByTargetUser(ctx, userID)
}
func (m *mockReviewRepo) GetByAuthorAndRoute(ctx context.Context, authorID, routeID uuid.UUID) ([]domain.Review, error) {
	return m.getByAuthorAndRoute(ctx, authorID, routeID)
}
func (m *mockReviewRepo) GetAverageRatings(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]domain.ReviewSummary, error) {
	return m.getAverageRatings(ctx, userIDs)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func pastTime() *time.Time  { t := time.Now().Add(-1 * time.Hour); return &t }
func futureTime() *time.Time { t := time.Now().Add(time.Hour); return &t }

func activeRoute(creatorID uuid.UUID, available uint) *domain.Route {
	return &domain.Route{
		ID:                  uuid.New(),
		CreatorID:           creatorID,
		LeavingAt:           futureTime(),
		AvailablePassengers: available,
		Stops:               []domain.Stop{},
		Participants:        []domain.Participant{},
	}
}

func startedRoute(creatorID uuid.UUID) *domain.Route {
	return &domain.Route{
		ID:                  uuid.New(),
		CreatorID:           creatorID,
		LeavingAt:           pastTime(),
		AvailablePassengers: 1,
		Stops:               []domain.Stop{},
		Participants:        []domain.Participant{},
	}
}

// ── ApplicationService tests ──────────────────────────────────────────────────

func TestApplicationService_Apply_CreatorForbidden(t *testing.T) {
	creatorID := uuid.New()
	route := activeRoute(creatorID, 2)

	svc := NewApplicationService(&mockAppRepo{}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	_, err := svc.Apply(context.Background(), creatorID, route.ID, domain.ApplyInput{})
	if !errors.Is(err, errs.ErrForbidden) {
		t.Errorf("Apply(creator) = %v, want ErrForbidden", err)
	}
}

func TestApplicationService_Apply_RouteFull(t *testing.T) {
	creatorID := uuid.New()
	userID := uuid.New()
	route := activeRoute(creatorID, 0)

	svc := NewApplicationService(&mockAppRepo{}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	_, err := svc.Apply(context.Background(), userID, route.ID, domain.ApplyInput{})
	if !errors.Is(err, errs.ErrRouteFull) {
		t.Errorf("Apply(full route) = %v, want ErrRouteFull", err)
	}
}

func TestApplicationService_Apply_AlreadyApplied(t *testing.T) {
	creatorID := uuid.New()
	userID := uuid.New()
	route := activeRoute(creatorID, 2)

	existing := &domain.Application{ID: uuid.New(), Status: "pending"}
	svc := NewApplicationService(&mockAppRepo{
		getByUserAndRoute: func(_ context.Context, _, _ uuid.UUID) (*domain.Application, error) { return existing, nil },
	}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	_, err := svc.Apply(context.Background(), userID, route.ID, domain.ApplyInput{})
	if !errors.Is(err, errs.ErrAlreadyApplied) {
		t.Errorf("Apply(already applied) = %v, want ErrAlreadyApplied", err)
	}
}

func TestApplicationService_Apply_RouteStarted(t *testing.T) {
	creatorID := uuid.New()
	userID := uuid.New()
	route := startedRoute(creatorID)

	svc := NewApplicationService(&mockAppRepo{}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	_, err := svc.Apply(context.Background(), userID, route.ID, domain.ApplyInput{})
	if !errors.Is(err, errs.ErrRouteStarted) {
		t.Errorf("Apply(started route) = %v, want ErrRouteStarted", err)
	}
}

func TestApplicationService_Apply_Success(t *testing.T) {
	creatorID := uuid.New()
	userID := uuid.New()
	newID := uuid.New()
	route := activeRoute(creatorID, 2)

	svc := NewApplicationService(&mockAppRepo{
		getByUserAndRoute: func(_ context.Context, _, _ uuid.UUID) (*domain.Application, error) { return nil, nil },
		create:            func(_ context.Context, _, _ uuid.UUID, _ domain.ApplyInput) (uuid.UUID, error) { return newID, nil },
	}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	got, err := svc.Apply(context.Background(), userID, route.ID, domain.ApplyInput{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got != newID {
		t.Errorf("Apply() = %v, want %v", got, newID)
	}
}

func TestApplicationService_Review_Forbidden(t *testing.T) {
	creatorID := uuid.New()
	callerID := uuid.New()
	appID := uuid.New()
	route := activeRoute(creatorID, 2)
	app := &domain.Application{ID: appID, RouteID: route.ID, Status: "pending"}

	svc := NewApplicationService(&mockAppRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Application, error) { return app, nil },
	}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	err := svc.Review(context.Background(), appID, "approved", callerID)
	if !errors.Is(err, errs.ErrForbidden) {
		t.Errorf("Review(non-creator) = %v, want ErrForbidden", err)
	}
}

func TestApplicationService_Review_NotPending(t *testing.T) {
	creatorID := uuid.New()
	appID := uuid.New()
	route := activeRoute(creatorID, 2)
	app := &domain.Application{ID: appID, RouteID: route.ID, Status: "approved"}

	svc := NewApplicationService(&mockAppRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Application, error) { return app, nil },
	}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	err := svc.Review(context.Background(), appID, "rejected", creatorID)
	if !errors.Is(err, errs.ErrConflict) {
		t.Errorf("Review(already approved) = %v, want ErrConflict", err)
	}
}

func TestApplicationService_Review_RouteStarted(t *testing.T) {
	creatorID := uuid.New()
	appID := uuid.New()
	route := startedRoute(creatorID)
	app := &domain.Application{ID: appID, RouteID: route.ID, Status: "pending"}

	svc := NewApplicationService(&mockAppRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Application, error) { return app, nil },
	}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	err := svc.Review(context.Background(), appID, "approved", creatorID)
	if !errors.Is(err, errs.ErrRouteStarted) {
		t.Errorf("Review(started route) = %v, want ErrRouteStarted", err)
	}
}

func TestApplicationService_Cancel_NotOwner(t *testing.T) {
	ownerID := uuid.New()
	callerID := uuid.New()
	appID := uuid.New()
	app := &domain.Application{ID: appID, UserID: ownerID, Status: "pending"}

	svc := NewApplicationService(&mockAppRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Application, error) { return app, nil },
	}, &mockRouteRepo{})
	err := svc.Cancel(context.Background(), appID, callerID)
	if !errors.Is(err, errs.ErrForbidden) {
		t.Errorf("Cancel(not owner) = %v, want ErrForbidden", err)
	}
}

func TestApplicationService_Cancel_NotPending(t *testing.T) {
	ownerID := uuid.New()
	appID := uuid.New()
	app := &domain.Application{ID: appID, UserID: ownerID, Status: "approved"}

	svc := NewApplicationService(&mockAppRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Application, error) { return app, nil },
	}, &mockRouteRepo{})
	err := svc.Cancel(context.Background(), appID, ownerID)
	if !errors.Is(err, errs.ErrConflict) {
		t.Errorf("Cancel(not pending) = %v, want ErrConflict", err)
	}
}

func TestApplicationService_Cancel_RouteStarted(t *testing.T) {
	ownerID := uuid.New()
	appID := uuid.New()
	routeID := uuid.New()
	route := startedRoute(uuid.New())
	route.ID = routeID
	app := &domain.Application{ID: appID, UserID: ownerID, RouteID: routeID, Status: "pending"}

	svc := NewApplicationService(&mockAppRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Application, error) { return app, nil },
	}, &mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	})
	err := svc.Cancel(context.Background(), appID, ownerID)
	if !errors.Is(err, errs.ErrRouteStarted) {
		t.Errorf("Cancel(started route) = %v, want ErrRouteStarted", err)
	}
}

// ── RouteService tests ────────────────────────────────────────────────────────

func TestRouteService_CreateReview_RouteNotFinished(t *testing.T) {
	route := activeRoute(uuid.New(), 1)
	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	_, err := svc.CreateReview(context.Background(), route.ID, uuid.New(), 5, "", uuid.New())
	if !errors.Is(err, errs.ErrRouteNotFinished) {
		t.Errorf("CreateReview(active route) = %v, want ErrRouteNotFinished", err)
	}
}

func TestRouteService_CreateReview_SelfReview(t *testing.T) {
	userID := uuid.New()
	route := &domain.Route{ID: uuid.New(), LeavingAt: pastTime(), Stops: []domain.Stop{}, Participants: []domain.Participant{}}

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	_, err := svc.CreateReview(context.Background(), route.ID, userID, 5, "", userID)
	if !errors.Is(err, errs.ErrForbidden) {
		t.Errorf("CreateReview(self) = %v, want ErrForbidden", err)
	}
}

func TestRouteService_CreateReview_InvalidRating(t *testing.T) {
	route := &domain.Route{ID: uuid.New(), LeavingAt: pastTime(), Stops: []domain.Stop{}, Participants: []domain.Participant{}}

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	for _, rating := range []int{0, 6, -1} {
		_, err := svc.CreateReview(context.Background(), route.ID, uuid.New(), rating, "", uuid.New())
		if !errors.Is(err, errs.ErrForbidden) {
			t.Errorf("CreateReview(rating=%d) = %v, want ErrForbidden", rating, err)
		}
	}
}

func TestRouteService_CreateReview_AuthorNotParticipant(t *testing.T) {
	authorID := uuid.New()
	targetID := uuid.New()
	route := &domain.Route{
		ID:        uuid.New(),
		LeavingAt: pastTime(),
		Stops:     []domain.Stop{},
		Participants: []domain.Participant{
			{UserID: targetID, Status: "approved"},
		},
	}

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	_, err := svc.CreateReview(context.Background(), route.ID, authorID, 5, "", targetID)
	if !errors.Is(err, errs.ErrNotParticipant) {
		t.Errorf("CreateReview(author not participant) = %v, want ErrNotParticipant", err)
	}
}

func TestRouteService_CreateReview_TargetNotParticipant(t *testing.T) {
	authorID := uuid.New()
	targetID := uuid.New()
	route := &domain.Route{
		ID:        uuid.New(),
		LeavingAt: pastTime(),
		Stops:     []domain.Stop{},
		Participants: []domain.Participant{
			{UserID: authorID, Status: "driver"},
		},
	}

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	_, err := svc.CreateReview(context.Background(), route.ID, authorID, 5, "", targetID)
	if !errors.Is(err, errs.ErrNotParticipant) {
		t.Errorf("CreateReview(target not participant) = %v, want ErrNotParticipant", err)
	}
}

func TestRouteService_CreateReview_Success(t *testing.T) {
	authorID := uuid.New()
	targetID := uuid.New()
	newID := uuid.New()
	route := &domain.Route{
		ID:        uuid.New(),
		LeavingAt: pastTime(),
		Stops:     []domain.Stop{},
		Participants: []domain.Participant{
			{UserID: authorID, Status: "driver"},
			{UserID: targetID, Status: "approved"},
		},
	}

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{
		create: func(_ context.Context, _ domain.CreateReviewInput) (uuid.UUID, error) { return newID, nil },
	})

	got, err := svc.CreateReview(context.Background(), route.ID, authorID, 4, "great ride", targetID)
	if err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if got != newID {
		t.Errorf("CreateReview() = %v, want %v", got, newID)
	}
}

func TestRouteService_Search_FiltersByDeviation(t *testing.T) {
	creatorID := uuid.New()
	withinRoute := domain.Route{
		ID: uuid.New(), CreatorID: creatorID,
		StartLat: 0, StartLng: 0, EndLat: 1, EndLng: 1,
		MaxDeviation: 1000,
		Stops:        []domain.Stop{},
		Participants: []domain.Participant{},
	}
	farRoute := domain.Route{
		ID: uuid.New(), CreatorID: creatorID,
		StartLat: 50, StartLng: 50, EndLat: 51, EndLng: 51,
		MaxDeviation: 1,
		Stops:        []domain.Stop{},
		Participants: []domain.Participant{},
	}

	svc := NewRouteService(&mockRouteRepo{
		listSearchable: func(_ context.Context) ([]domain.Route, error) {
			return []domain.Route{withinRoute, farRoute}, nil
		},
	}, &mockReviewRepo{
		getAverageRatings: func(_ context.Context, _ []uuid.UUID) (map[uuid.UUID]domain.ReviewSummary, error) {
			return map[uuid.UUID]domain.ReviewSummary{}, nil
		},
	})

	results, err := svc.Search(context.Background(), domain.SearchRouteInput{
		StartLat: 0, StartLng: 0, EndLat: 1, EndLng: 1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].ID != withinRoute.ID {
		t.Errorf("Search() returned wrong route")
	}
}

func TestRouteService_Search_SortedByDeviation(t *testing.T) {
	creatorID := uuid.New()
	closeRoute := domain.Route{
		ID: uuid.New(), CreatorID: creatorID,
		StartLat: 0, StartLng: 0, EndLat: 1, EndLng: 1,
		MaxDeviation: 1000,
		Stops:        []domain.Stop{},
		Participants: []domain.Participant{},
	}
	fartherRoute := domain.Route{
		ID: uuid.New(), CreatorID: creatorID,
		StartLat: 0.1, StartLng: 0.1, EndLat: 1.1, EndLng: 1.1,
		MaxDeviation: 1000,
		Stops:        []domain.Stop{},
		Participants: []domain.Participant{},
	}

	svc := NewRouteService(&mockRouteRepo{
		listSearchable: func(_ context.Context) ([]domain.Route, error) {
			return []domain.Route{fartherRoute, closeRoute}, nil
		},
	}, &mockReviewRepo{
		getAverageRatings: func(_ context.Context, _ []uuid.UUID) (map[uuid.UUID]domain.ReviewSummary, error) {
			return map[uuid.UUID]domain.ReviewSummary{}, nil
		},
	})

	results, err := svc.Search(context.Background(), domain.SearchRouteInput{
		StartLat: 0, StartLng: 0, EndLat: 1, EndLng: 1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search() returned %d results, want 2", len(results))
	}
	if results[0].ID != closeRoute.ID {
		t.Errorf("Search() first result should be closest route")
	}
}

func TestRouteService_Delete_RouteStarted(t *testing.T) {
	creatorID := uuid.New()
	route := startedRoute(creatorID)

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	err := svc.Delete(context.Background(), route.ID, creatorID)
	if !errors.Is(err, errs.ErrRouteStarted) {
		t.Errorf("Delete(started route) = %v, want ErrRouteStarted", err)
	}
}

func TestRouteService_Update_RouteStarted(t *testing.T) {
	creatorID := uuid.New()
	route := startedRoute(creatorID)

	svc := NewRouteService(&mockRouteRepo{
		getByID: func(_ context.Context, _ uuid.UUID) (*domain.Route, error) { return route, nil },
	}, &mockReviewRepo{})

	err := svc.Update(context.Background(), route.ID, creatorID, domain.UpdateRouteInput{})
	if !errors.Is(err, errs.ErrRouteStarted) {
		t.Errorf("Update(started route) = %v, want ErrRouteStarted", err)
	}
}
