package service

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
)

type RouteService struct {
	routes  domain.RouteRepository
	reviews domain.ReviewRepository
}

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

func (s *RouteService) GetMyReviewsForRoute(ctx context.Context, routeID, authorID uuid.UUID) ([]domain.Review, error) {
	return s.reviews.GetByAuthorAndRoute(ctx, authorID, routeID)
}

func (s *RouteService) CreateReview(ctx context.Context, routeID, authorID uuid.UUID, rating int, comment string, targetID uuid.UUID) (uuid.UUID, error) {
	route, err := s.routes.GetByID(ctx, routeID)
	if err != nil {
		return uuid.Nil, err
	}

	if route.LeavingAt == nil || route.LeavingAt.After(time.Now()) {
		return uuid.Nil, errs.ErrRouteNotFinished
	}

	if authorID == targetID {
		return uuid.Nil, errs.ErrForbidden
	}

	if rating < 1 || rating > 5 {
		return uuid.Nil, errs.ErrForbidden
	}

	authorParticipant := false
	targetParticipant := false
	for _, p := range route.Participants {
		if p.UserID == authorID && (p.Status == "driver" || p.Status == "approved") {
			authorParticipant = true
		}

		if p.UserID == targetID && (p.Status == "driver" || p.Status == "approved") {
			targetParticipant = true
		}
	}

	if !authorParticipant {
		return uuid.Nil, errs.ErrNotParticipant
	}

	if !targetParticipant {
		return uuid.Nil, errs.ErrNotParticipant
	}

	return s.reviews.Create(ctx, domain.CreateReviewInput{
		AuthorID: authorID,
		TargetID: targetID,
		RouteID:  routeID,
		Rating:   rating,
		Comment:  comment,
	})
}

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
	allStops := make([]domain.SearchStop, 0, len(search.Stops)+2)
	allStops = append(allStops, domain.SearchStop{Lat: search.StartLat, Lng: search.StartLng})
	allStops = append(allStops, search.Stops...)
	allStops = append(allStops, domain.SearchStop{Lat: search.EndLat, Lng: search.EndLng})

	return deviationForStops(r.StartLat, r.StartLng, r.EndLat, r.EndLng, r.Stops, domain.SearchRouteInput{Stops: allStops}, false)
}

func deviationForStops(startLat, startLng, endLat, endLng float64, stops []domain.Stop, search domain.SearchRouteInput, strict bool) float64 {
	segments := buildSegmentsFromStops(startLat, startLng, endLat, endLng, stops)
	dev := 0.0
	currentSeg := 0

	for _, us := range search.Stops {
		bestDist := math.MaxFloat64
		bestSeg := currentSeg
		bestT := 0.0
		bestRawT := 0.0

		for j := currentSeg; j < len(segments); j++ {
			seg := segments[j]
			d, t, rawT := pointToSegmentDistanceT(us.Lat, us.Lng, seg.aLat, seg.aLng, seg.bLat, seg.bLng)
			if d < bestDist {
				bestDist = d
				bestSeg = j
				bestT = t
				bestRawT = rawT
			}
		}

		if strict && (bestRawT < 0 || (bestSeg == len(segments)-1 && bestRawT > 1)) {
			return math.MaxFloat64
		}

		dev += bestDist

		seg := segments[bestSeg]
		segments[bestSeg] = routeSegment{
			aLat: seg.aLat + bestT*(seg.bLat-seg.aLat),
			aLng: seg.aLng + bestT*(seg.bLng-seg.aLng),
			bLat: seg.bLat,
			bLng: seg.bLng,
		}
		currentSeg = bestSeg
	}

	return dev
}

type routeSegment struct{ aLat, aLng, bLat, bLng float64 }

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

func pointToSegmentDistance(pLat, pLng, aLat, aLng, bLat, bLng float64) float64 {
	d, _, _ := pointToSegmentDistanceT(pLat, pLng, aLat, aLng, bLat, bLng)
	return d
}

func pointToSegmentDistanceT(pLat, pLng, aLat, aLng, bLat, bLng float64) (dist, t, rawT float64) {
	if aLat == bLat && aLng == bLng {
		return Haversine(pLat, pLng, aLat, aLng), 0, 0
	}

	midLat := (aLat + bLat) / 2
	cos := math.Cos(midLat * math.Pi / 180)

	px, py := pLng*cos, pLat
	ax, ay := aLng*cos, aLat
	bx, by := bLng*cos, bLat

	abx := bx - ax
	aby := by - ay
	len2 := abx*abx + aby*aby

	rawT = ((px-ax)*abx + (py-ay)*aby) / len2
	t = rawT
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	closestLat := aLat + t*(bLat-aLat)
	closestLng := aLng + t*(bLng-aLng)
	return Haversine(pLat, pLng, closestLat, closestLng), t, rawT
}
