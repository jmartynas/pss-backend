package service

import (
	"context"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

// RouteService contains all route business logic.
type RouteService struct {
	routes  domain.RouteRepository
	reviews domain.ReviewRepository
}

// NewRouteService creates a RouteService backed by the given repository.
func NewRouteService(routes domain.RouteRepository, reviews domain.ReviewRepository) *RouteService {
	return &RouteService{routes: routes, reviews: reviews}
}

func (s *RouteService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	return s.routes.GetByID(ctx, id)
}

func (s *RouteService) Create(ctx context.Context, creatorID uuid.UUID, in domain.CreateRouteInput) (uuid.UUID, error) {
	return s.routes.Create(ctx, creatorID, in)
}

func (s *RouteService) Update(ctx context.Context, id, creatorID uuid.UUID, in domain.UpdateRouteInput) error {
	route, err := s.routes.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	return s.routes.Update(ctx, id, creatorID, in)
}

func (s *RouteService) Delete(ctx context.Context, id, creatorID uuid.UUID) error {
	route, err := s.routes.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if routeStarted(route) {
		return errs.ErrRouteStarted
	}
	return s.routes.Delete(ctx, id, creatorID)
}

func (s *RouteService) ListByCreator(ctx context.Context, creatorID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error) {
	return s.routes.ListByCreator(ctx, creatorID, filter)
}

func (s *RouteService) ListByParticipant(ctx context.Context, userID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error) {
	return s.routes.ListByParticipant(ctx, userID, filter)
}

// Search returns routes that match the search criteria, sorted by deviation (ascending).
func (s *RouteService) Search(ctx context.Context, in domain.SearchRouteInput) ([]domain.Route, error) {
	all, err := s.routes.ListSearchable(ctx)
	if err != nil {
		return nil, err
	}

	type entry struct {
		route domain.Route
		dev   float64
	}
	candidates := make([]entry, 0, len(all))
	for i := range all {
		dev := calculateDeviation(&all[i], in)
		if dev <= all[i].MaxDeviation {
			candidates = append(candidates, entry{route: all[i], dev: dev})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dev < candidates[j].dev
	})

	out := make([]domain.Route, len(candidates))
	for i, c := range candidates {
		out[i] = c.route
	}

	// Enrich with driver ratings.
	creatorIDs := make([]uuid.UUID, 0, len(out))
	seen := make(map[uuid.UUID]bool)
	for _, r := range out {
		if !seen[r.CreatorID] {
			creatorIDs = append(creatorIDs, r.CreatorID)
			seen[r.CreatorID] = true
		}
	}
	if len(creatorIDs) > 0 {
		ratings, err := s.reviews.GetAverageRatings(ctx, creatorIDs)
		if err == nil {
			for i := range out {
				if summary, ok := ratings[out[i].CreatorID]; ok {
					out[i].CreatorReviewCount = summary.Count
					if summary.Count >= 5 {
						avg := summary.Avg
						out[i].CreatorRating = &avg
					}
				}
			}
		}
	}

	return out, nil
}

// ── Geo calculations (moved from route/calculations.go) ──────────────────────

// Haversine returns the great-circle distance in kilometres between two points.
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func calculateDeviation(r *domain.Route, search domain.SearchRouteInput) float64 {
	return Haversine(search.StartLat, search.StartLng, r.StartLat, r.StartLng) +
		Haversine(search.EndLat, search.EndLng, r.EndLat, r.EndLng) +
		deviationForStops(r.StartLat, r.StartLng, r.EndLat, r.EndLng, r.Stops, search)
}

func deviationForStops(startLat, startLng, endLat, endLng float64, stops []domain.Stop, search domain.SearchRouteInput) float64 {
	segments := buildSegmentsFromStops(startLat, startLng, endLat, endLng, stops)
	dev := 0.0
	for _, us := range search.Stops {
		dev += minDistanceToSegment(us.Lat, us.Lng, segments)
	}
	return dev
}

type routeSegment struct{ aLat, aLng, bLat, bLng float64 }

// buildSegmentsFromStops returns the actual path segments of a route:
// start→stop1, stop1→stop2, …, stopN→end.
func buildSegmentsFromStops(startLat, startLng, endLat, endLng float64, stops []domain.Stop) []routeSegment {
	points := make([][2]float64, 0, len(stops)+2)
	points = append(points, [2]float64{startLat, startLng})
	for _, s := range stops {
		points = append(points, [2]float64{s.Lat, s.Lng})
	}
	points = append(points, [2]float64{endLat, endLng})

	segs := make([]routeSegment, 0, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		segs = append(segs, routeSegment{points[i][0], points[i][1], points[i+1][0], points[i+1][1]})
	}
	return segs
}

// minDistanceToSegment returns the minimum great-circle distance (km) from
// point (lat, lng) to any of the given route segments.
func minDistanceToSegment(lat, lng float64, segments []routeSegment) float64 {
	min := math.MaxFloat64
	for _, seg := range segments {
		if d := pointToSegmentDistance(lat, lng, seg.aLat, seg.aLng, seg.bLat, seg.bLng); d < min {
			min = d
		}
	}
	return min
}

// pointToSegmentDistance returns the great-circle distance (km) from point P
// to the closest point on segment AB. It projects P onto the line through A
// and B in lat/lng space (valid approximation for segments < ~500 km) and
// clamps the projection to [0,1] so it never extends beyond the endpoints.
func pointToSegmentDistance(pLat, pLng, aLat, aLng, bLat, bLng float64) float64 {
	abLat := bLat - aLat
	abLng := bLng - aLng
	len2 := abLat*abLat + abLng*abLng
	if len2 == 0 {
		return Haversine(pLat, pLng, aLat, aLng)
	}
	t := ((pLat-aLat)*abLat + (pLng-aLng)*abLng) / len2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return Haversine(pLat, pLng, aLat+t*abLat, aLng+t*abLng)
}
