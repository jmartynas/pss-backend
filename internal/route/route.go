package route

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/errs"
)

type Stop struct {
	ID               uuid.UUID  `json:"id"`
	ApplicationID    *uuid.UUID `json:"application_id,omitempty"` // nil = creator's stop
	Position         uint       `json:"position"`
	Lat              float64    `json:"lat"`
	Lng              float64    `json:"lng"`
	PlaceID          *string    `json:"place_id"`
	FormattedAddress *string    `json:"formatted_address"`
	Status           string     `json:"status"`
}

type Route struct {
	ID                    uuid.UUID  `json:"id"`
	CreatorID             uuid.UUID  `json:"creator_id"`
	CreatorName           string     `json:"creator_name"`
	Description           *string    `json:"description"`
	StartLat              float64    `json:"start_lat"`
	StartLng              float64    `json:"start_lng"`
	StartPlaceID          *string    `json:"start_place_id"`
	StartFormattedAddress *string    `json:"start_formatted_address"`
	EndLat                float64    `json:"end_lat"`
	EndLng                float64    `json:"end_lng"`
	EndPlaceID            *string    `json:"end_place_id"`
	EndFormattedAddress   *string    `json:"end_formatted_address"`
	MaxPassengers         uint       `json:"max_passengers"`
	AvailablePassengers   uint       `json:"available_passengers"`
	LeavingAt             *time.Time `json:"leaving_at"`
	Stops                 []Stop     `json:"stops"`
}

func GetRoute(ctx context.Context, db *sql.DB, id uuid.UUID) (*Route, error) {
	var d Route
	var idStr, creatorIDStr string

	err := db.QueryRowContext(ctx, `
		SELECT
			r.id,
			r.creator_user_id,
			COALESCE(u.name, u.email, ''),
			r.description,
			r.start_lat, r.start_lng,
			r.start_place_id, r.start_formatted_address,
			r.end_lat, r.end_lng,
			r.end_place_id, r.end_formatted_address,
			r.max_passengers,
			GREATEST(0, r.max_passengers - (
				SELECT COUNT(*) FROM participants p
				WHERE p.route_id = r.id AND p.deleted_at IS NULL
			)) AS available_passengers,
			r.leaving_at
		FROM routes r
		JOIN users u ON u.id = r.creator_user_id
		WHERE r.id = ? AND r.deleted_at IS NULL
	`, id.String()).Scan(
		&idStr,
		&creatorIDStr,
		&d.CreatorName,
		&d.Description,
		&d.StartLat, &d.StartLng,
		&d.StartPlaceID, &d.StartFormattedAddress,
		&d.EndLat, &d.EndLng,
		&d.EndPlaceID, &d.EndFormattedAddress,
		&d.MaxPassengers,
		&d.AvailablePassengers,
		&d.LeavingAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, fmt.Errorf("get route detail: %w", err)
	}

	d.ID, _ = uuid.Parse(idStr)
	d.CreatorID, _ = uuid.Parse(creatorIDStr)

	rows, err := db.QueryContext(ctx, `
		SELECT id, application_id, position, lat, lng, place_id, formatted_address, status
		FROM route_stops
		WHERE route_id = ? AND deleted_at IS NULL AND status = 'approved'
		ORDER BY position
	`, id.String())
	if err != nil {
		return nil, fmt.Errorf("get route stops: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s Stop
		var sidStr string
		var appID *string
		if err := rows.Scan(&sidStr, &appID, &s.Position, &s.Lat, &s.Lng, &s.PlaceID, &s.FormattedAddress, &s.Status); err != nil {
			return nil, fmt.Errorf("scan route stop: %w", err)
		}
		s.ID, _ = uuid.Parse(sidStr)
		if appID != nil {
			parsed, _ := uuid.Parse(*appID)
			s.ApplicationID = &parsed
		}
		d.Stops = append(d.Stops, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get route stops: %w", err)
	}

	if d.Stops == nil {
		d.Stops = []Stop{}
	}

	return &d, nil
}

type StopInput struct {
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	PlaceID          *string `json:"place_id"`
	FormattedAddress *string `json:"formatted_address"`
}

type CreateInput struct {
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
	LeavingAt             *time.Time  `json:"leaving_at"`
	Stops                 []StopInput `json:"stops"`
}

func Create(ctx context.Context, db *sql.DB, creatorID uuid.UUID, in CreateInput) (uuid.UUID, error) {
	id := uuid.New()
	_, err := db.ExecContext(ctx, `
		INSERT INTO routes (
			id, creator_user_id, description,
			start_lat, start_lng, start_place_id, start_formatted_address,
			end_lat, end_lng, end_place_id, end_formatted_address,
			max_passengers, leaving_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		id.String(), creatorID.String(), nullStr(in.Description),
		in.StartLat, in.StartLng, nullStr(in.StartPlaceID), nullStr(in.StartFormattedAddress),
		in.EndLat, in.EndLng, nullStr(in.EndPlaceID), nullStr(in.EndFormattedAddress),
		in.MaxPassengers, nullTime(in.LeavingAt),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create route: %w", err)
	}

	for i, s := range in.Stops {
		stopID := uuid.New()
		_, err := db.ExecContext(ctx, `
			INSERT INTO route_stops (
				id, route_id, application_id, position,
				lat, lng, place_id, formatted_address, status
			) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, 'approved')
		`,
			stopID.String(), id.String(), uint(i),
			s.Lat, s.Lng, nullStr(s.PlaceID), nullStr(s.FormattedAddress),
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("create route stop: %w", err)
		}
	}

	return id, nil
}

type SearchStopInput struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type SearchInput struct {
	StartLat float64           `json:"start_lat"`
	StartLng float64           `json:"start_lng"`
	EndLat   float64           `json:"end_lat"`
	EndLng   float64           `json:"end_lng"`
	Stops    []SearchStopInput `json:"stops"`
}

// Haversine returns distance in kilometers between two points.
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

const maxInterleavings = 500

// CalculateDeviation computes the path deviation between a route (with its stops) and the search input.
// Finds the best interleaving of participants' stops (each participant's order preserved) that
// minimizes the searching user's deviation. Each user stop is placed in the segment that minimizes
// its distance for that ordering.
func CalculateDeviation(route *Route, search SearchInput) float64 {
	baseDev := Haversine(search.StartLat, search.StartLng, route.StartLat, route.StartLng) +
		Haversine(search.EndLat, search.EndLng, route.EndLat, route.EndLng)

	groups := groupStopsByParticipant(route.Stops)
	orderings := allInterleavings(groups, maxInterleavings)

	if len(orderings) == 0 {
		return baseDev + deviationForStops(route.StartLat, route.StartLng, route.EndLat, route.EndLng, route.Stops, search)
	}

	minStopDev := math.MaxFloat64
	for _, ordered := range orderings {
		d := deviationForStops(route.StartLat, route.StartLng, route.EndLat, route.EndLng, ordered, search)
		if d < minStopDev {
			minStopDev = d
		}
	}
	return baseDev + minStopDev
}

// groupStopsByParticipant groups stops by application_id (nil = creator). Each group keeps position order.
func groupStopsByParticipant(stops []Stop) [][]Stop {
	// Use a stable key for grouping: application_id string or "creator"
	type key struct {
		appID string
	}
	ordered := make([][]Stop, 0)
	seen := make(map[key]int)

	for _, s := range stops {
		k := key{appID: "creator"}
		if s.ApplicationID != nil {
			k.appID = s.ApplicationID.String()
		}
		idx, ok := seen[k]
		if !ok {
			idx = len(ordered)
			seen[k] = idx
			ordered = append(ordered, nil)
		}
		ordered[idx] = append(ordered[idx], s)
	}
	return ordered
}

// allInterleavings returns all valid merges of groups (each group's order preserved), up to max.
func allInterleavings(groups [][]Stop, max int) [][]Stop {
	if len(groups) == 0 {
		return nil
	}
	if len(groups) == 1 {
		return [][]Stop{groups[0]}
	}

	var result [][]Stop
	indices := make([]int, len(groups))
	current := make([]Stop, 0, totalLen(groups))

	var gen func()
	gen = func() {
		if len(result) >= max {
			return
		}
		done := true
		for g := range groups {
			if indices[g] < len(groups[g]) {
				done = false
				break
			}
		}
		if done {
			result = append(result, append([]Stop{}, current...))
			return
		}
		for g := range groups {
			if indices[g] < len(groups[g]) {
				current = append(current, groups[g][indices[g]])
				indices[g]++
				gen()
				indices[g]--
				current = current[:len(current)-1]
			}
		}
	}
	gen()
	return result
}

func totalLen(groups [][]Stop) int {
	n := 0
	for _, g := range groups {
		n += len(g)
	}
	return n
}

// deviationForStops returns the sum of min distances from each search stop to the route segments.
func deviationForStops(startLat, startLng, endLat, endLng float64, stops []Stop, search SearchInput) float64 {
	segments := buildSegmentsFromStops(startLat, startLng, endLat, endLng, stops)
	dev := 0.0
	for _, us := range search.Stops {
		dev += minDistanceToSegment(us.Lat, us.Lng, segments)
	}
	return dev
}

// buildSegmentsFromStops returns segment midpoints for the given stop order.
func buildSegmentsFromStops(startLat, startLng, endLat, endLng float64, stops []Stop) [][2]float64 {
	if len(stops) == 0 {
		return [][2]float64{{(startLat + endLat) / 2, (startLng + endLng) / 2}}
	}
	segs := make([][2]float64, 0, len(stops)+1)
	prevLat, prevLng := startLat, startLng
	for _, s := range stops {
		segs = append(segs, [2]float64{(prevLat + s.Lat) / 2, (prevLng + s.Lng) / 2})
		prevLat, prevLng = s.Lat, s.Lng
	}
	segs = append(segs, [2]float64{(prevLat + endLat) / 2, (prevLng + endLng) / 2})
	return segs
}

func minDistanceToSegment(lat, lng float64, segments [][2]float64) float64 {
	min := math.MaxFloat64
	for _, seg := range segments {
		d := Haversine(lat, lng, seg[0], seg[1])
		if d < min {
			min = d
		}
	}
	return min
}

func fetchSearchableRoutes(ctx context.Context, db *sql.DB) ([]Route, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.id, r.creator_user_id, COALESCE(u.name, u.email, ''),
			r.description, r.start_lat, r.start_lng,
			r.start_place_id, r.start_formatted_address,
			r.end_lat, r.end_lng, r.end_place_id, r.end_formatted_address,
			r.max_passengers,
			GREATEST(0, r.max_passengers - (SELECT COUNT(*) FROM participants p WHERE p.route_id = r.id AND p.deleted_at IS NULL)) AS available_passengers,
			r.leaving_at
		FROM routes r
		JOIN users u ON u.id = r.creator_user_id
		WHERE r.deleted_at IS NULL
		  AND r.max_passengers > (SELECT COUNT(*) FROM participants p WHERE p.route_id = r.id AND p.deleted_at IS NULL)
	`)
	if err != nil {
		return nil, fmt.Errorf("search routes: %w", err)
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var d Route
		var idStr, creatorIDStr string
		if err := rows.Scan(&idStr, &creatorIDStr, &d.CreatorName, &d.Description,
			&d.StartLat, &d.StartLng, &d.StartPlaceID, &d.StartFormattedAddress,
			&d.EndLat, &d.EndLng, &d.EndPlaceID, &d.EndFormattedAddress,
			&d.MaxPassengers, &d.AvailablePassengers, &d.LeavingAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		d.ID, _ = uuid.Parse(idStr)
		d.CreatorID, _ = uuid.Parse(creatorIDStr)

		stops, err := fetchRouteStops(ctx, db, idStr)
		if err != nil {
			return nil, err
		}
		d.Stops = stops

		routes = append(routes, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search routes: %w", err)
	}
	return routes, nil
}

func fetchRouteStops(ctx context.Context, db *sql.DB, routeID string) ([]Stop, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, application_id, position, lat, lng, place_id, formatted_address, status
		FROM route_stops
		WHERE route_id = ? AND deleted_at IS NULL AND status = 'approved'
		ORDER BY position
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("get route stops: %w", err)
	}
	defer rows.Close()

	var stops []Stop
	for rows.Next() {
		var s Stop
		var sidStr string
		var appID *string
		if err := rows.Scan(&sidStr, &appID, &s.Position, &s.Lat, &s.Lng, &s.PlaceID, &s.FormattedAddress, &s.Status); err != nil {
			return nil, fmt.Errorf("scan stop: %w", err)
		}
		s.ID, _ = uuid.Parse(sidStr)
		if appID != nil {
			parsed, _ := uuid.Parse(*appID)
			s.ApplicationID = &parsed
		}
		stops = append(stops, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get route stops: %w", err)
	}
	return stops, nil
}

func Search(ctx context.Context, db *sql.DB, in SearchInput) ([]Route, error) {
	routes, err := fetchSearchableRoutes(ctx, db)
	if err != nil {
		return nil, err
	}

	type routeWithDev struct {
		route Route
		dev   float64
	}
	withDev := make([]routeWithDev, len(routes))
	for i := range routes {
		withDev[i] = routeWithDev{
			route: routes[i],
			dev:   CalculateDeviation(&routes[i], in),
		}
	}

	sort.Slice(withDev, func(i, j int) bool {
		return withDev[i].dev < withDev[j].dev
	})

	out := make([]Route, len(withDev))
	for i, rwd := range withDev {
		out[i] = rwd.route
	}
	return out, nil
}

func nullStr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
