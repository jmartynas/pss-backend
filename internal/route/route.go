package route

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/errs"
)

type Stop struct {
	ID               uuid.UUID  `json:"id"`
	ApplicationID    *uuid.UUID `json:"application_id,omitempty"`
	Position         uint       `json:"position"`
	Lat              float64    `json:"lat"`
	Lng              float64    `json:"lng"`
	PlaceID          *string    `json:"place_id"`
	FormattedAddress *string    `json:"formatted_address"`
	Status           string     `json:"status"`
}

type Participant struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
}

type Route struct {
	ID                    uuid.UUID     `json:"id"`
	CreatorID             uuid.UUID     `json:"creator_id"`
	CreatorName           string        `json:"creator_name"`
	Description           *string       `json:"description"`
	StartLat              float64       `json:"start_lat"`
	StartLng              float64       `json:"start_lng"`
	StartPlaceID          *string       `json:"start_place_id"`
	StartFormattedAddress *string       `json:"start_formatted_address"`
	EndLat                float64       `json:"end_lat"`
	EndLng                float64       `json:"end_lng"`
	EndPlaceID            *string       `json:"end_place_id"`
	EndFormattedAddress   *string       `json:"end_formatted_address"`
	MaxPassengers         uint          `json:"max_passengers"`
	MaxDeviation          float64       `json:"max_deviation"`
	AvailablePassengers   uint          `json:"available_passengers"`
	LeavingAt             *time.Time    `json:"leaving_at"`
	Stops                 []Stop        `json:"stops"`
	Participants          []Participant `json:"participants"`
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
			r.max_deviation,
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
		&d.MaxDeviation,
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

	participants, err := fetchRouteParticipants(ctx, db, id.String())
	if err != nil {
		return nil, fmt.Errorf("get route participants: %w", err)
	}
	d.Participants = participants

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
	MaxDeviation          float64     `json:"max_deviation"`
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
			max_passengers, max_deviation, leaving_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		id.String(), creatorID.String(), nullStr(in.Description),
		in.StartLat, in.StartLng, nullStr(in.StartPlaceID), nullStr(in.StartFormattedAddress),
		in.EndLat, in.EndLng, nullStr(in.EndPlaceID), nullStr(in.EndFormattedAddress),
		in.MaxPassengers, in.MaxDeviation, nullTime(in.LeavingAt),
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

func fetchSearchableRoutes(ctx context.Context, db *sql.DB) ([]Route, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.id, r.creator_user_id, COALESCE(u.name, u.email, ''),
			r.description, r.start_lat, r.start_lng,
			r.start_place_id, r.start_formatted_address,
			r.end_lat, r.end_lng, r.end_place_id, r.end_formatted_address,
			r.max_passengers, r.max_deviation,
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
			&d.MaxPassengers, &d.MaxDeviation, &d.AvailablePassengers, &d.LeavingAt); err != nil {
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

func fetchRouteParticipants(ctx context.Context, db *sql.DB, routeID string) ([]Participant, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.user_id, COALESCE(u.name, '')
		FROM participants p
		JOIN users u ON u.id = p.user_id
		WHERE p.route_id = ? AND p.deleted_at IS NULL
		ORDER BY p.created_at ASC
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("get route participants: %w", err)
	}
	defer rows.Close()

	var participants []Participant
	for rows.Next() {
		var p Participant
		var userIDStr string
		if err := rows.Scan(&userIDStr, &p.Name); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		p.UserID, _ = uuid.Parse(userIDStr)
		participants = append(participants, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get route participants: %w", err)
	}

	if participants == nil {
		participants = []Participant{}
	}

	return participants, nil
}

// fetchRoutesWithDetails fetches routes with stops and participants using batched queries (3 queries total)
func fetchRoutesWithDetails(ctx context.Context, db *sql.DB, baseQuery string, whereClause string, args ...interface{}) ([]Route, error) {
	// First, fetch routes
	query := baseQuery + whereClause + " ORDER BY r.created_at DESC"
	
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch routes: %w", err)
	}
	defer rows.Close()

	routesMap := make(map[string]*Route)
	var routeIDs []string

	for rows.Next() {
		var d Route
		var idStr, creatorIDStr string
		if err := rows.Scan(&idStr, &creatorIDStr, &d.CreatorName, &d.Description,
			&d.StartLat, &d.StartLng, &d.StartPlaceID, &d.StartFormattedAddress,
			&d.EndLat, &d.EndLng, &d.EndPlaceID, &d.EndFormattedAddress,
			&d.MaxPassengers, &d.MaxDeviation, &d.AvailablePassengers, &d.LeavingAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		d.ID, _ = uuid.Parse(idStr)
		d.CreatorID, _ = uuid.Parse(creatorIDStr)
		d.Stops = []Stop{}
		d.Participants = []Participant{}
		routesMap[idStr] = &d
		routeIDs = append(routeIDs, idStr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch routes: %w", err)
	}

	if len(routeIDs) == 0 {
		return []Route{}, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(routeIDs))
	queryArgs := make([]interface{}, len(routeIDs))
	for i, id := range routeIDs {
		placeholders[i] = "?"
		queryArgs[i] = id
	}
	inClause := "(" + strings.Join(placeholders, ",") + ")"

	// Fetch all stops for these routes in one query
	stopsRows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT route_id, id, application_id, position, lat, lng, place_id, formatted_address, status
		FROM route_stops
		WHERE route_id IN %s AND deleted_at IS NULL AND status = 'approved'
		ORDER BY route_id, position
	`, inClause), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("fetch stops: %w", err)
	}
	defer stopsRows.Close()

	for stopsRows.Next() {
		var routeIDStr, stopIDStr string
		var s Stop
		var appID *string
		if err := stopsRows.Scan(&routeIDStr, &stopIDStr, &appID, &s.Position, &s.Lat, &s.Lng, &s.PlaceID, &s.FormattedAddress, &s.Status); err != nil {
			return nil, fmt.Errorf("scan stop: %w", err)
		}
		s.ID, _ = uuid.Parse(stopIDStr)
		if appID != nil && *appID != "" {
			parsed, _ := uuid.Parse(*appID)
			s.ApplicationID = &parsed
		}
		if route, exists := routesMap[routeIDStr]; exists {
			route.Stops = append(route.Stops, s)
		}
	}

	if err := stopsRows.Err(); err != nil {
		return nil, fmt.Errorf("fetch stops: %w", err)
	}

	// Fetch all participants for these routes in one query
	participantsRows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.route_id, p.user_id, COALESCE(u.name, '')
		FROM participants p
		JOIN users u ON u.id = p.user_id
		WHERE p.route_id IN %s AND p.deleted_at IS NULL
		ORDER BY p.route_id, p.created_at ASC
	`, inClause), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("fetch participants: %w", err)
	}
	defer participantsRows.Close()

	for participantsRows.Next() {
		var routeIDStr, userIDStr string
		var p Participant
		if err := participantsRows.Scan(&routeIDStr, &userIDStr, &p.Name); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		p.UserID, _ = uuid.Parse(userIDStr)
		if route, exists := routesMap[routeIDStr]; exists {
			route.Participants = append(route.Participants, p)
		}
	}

	if err := participantsRows.Err(); err != nil {
		return nil, fmt.Errorf("fetch participants: %w", err)
	}

	// Convert map to slice maintaining order
	routes := make([]Route, 0, len(routeIDs))
	for _, id := range routeIDs {
		if route, exists := routesMap[id]; exists {
			if route.Stops == nil {
				route.Stops = []Stop{}
			}
			if route.Participants == nil {
				route.Participants = []Participant{}
			}
			routes = append(routes, *route)
		}
	}

	return routes, nil
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
	withDev := make([]routeWithDev, 0, len(routes))
	for i := range routes {
		dev := CalculateDeviation(&routes[i], in)
		if dev > routes[i].MaxDeviation {
			continue
		}
		withDev = append(withDev, routeWithDev{
			route: routes[i],
			dev:   dev,
		})
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

type RouteFilter string

const (
	RouteFilterActive RouteFilter = "active"
	RouteFilterPast   RouteFilter = "past"
)

func GetRoutesByCreator(ctx context.Context, db *sql.DB, creatorID uuid.UUID, filter RouteFilter) ([]Route, error) {
	baseQuery := `
		SELECT r.id, r.creator_user_id, COALESCE(u.name, u.email, ''),
			r.description, r.start_lat, r.start_lng,
			r.start_place_id, r.start_formatted_address,
			r.end_lat, r.end_lng, r.end_place_id, r.end_formatted_address,
			r.max_passengers, r.max_deviation,
			GREATEST(0, r.max_passengers - (SELECT COUNT(*) FROM participants p WHERE p.route_id = r.id AND p.deleted_at IS NULL)) AS available_passengers,
			r.leaving_at
		FROM routes r
		JOIN users u ON u.id = r.creator_user_id
		WHERE r.creator_user_id = ? AND r.deleted_at IS NULL`

	var timeFilter string
	switch filter {
	case RouteFilterActive:
		timeFilter = " AND (r.leaving_at IS NULL OR r.leaving_at > NOW())"
	case RouteFilterPast:
		timeFilter = " AND r.leaving_at IS NOT NULL AND r.leaving_at <= NOW()"
	default:
		timeFilter = ""
	}

	whereClause := timeFilter
	return fetchRoutesWithDetails(ctx, db, baseQuery, whereClause, creatorID.String())
}

// GetRoutesByParticipant fetches all routes where the specified user is a participant, optionally filtered by status.
// filter can be "active" (leaving_at IS NULL OR leaving_at > NOW()), "past" (leaving_at IS NOT NULL AND leaving_at <= NOW()), or empty for all routes.
func GetRoutesByParticipant(ctx context.Context, db *sql.DB, userID uuid.UUID, filter RouteFilter) ([]Route, error) {
	baseQuery := `
		SELECT r.id, r.creator_user_id, COALESCE(u.name, u.email, ''),
			r.description, r.start_lat, r.start_lng,
			r.start_place_id, r.start_formatted_address,
			r.end_lat, r.end_lng, r.end_place_id, r.end_formatted_address,
			r.max_passengers, r.max_deviation,
			GREATEST(0, r.max_passengers - (SELECT COUNT(*) FROM participants p WHERE p.route_id = r.id AND p.deleted_at IS NULL)) AS available_passengers,
			r.leaving_at
		FROM routes r
		JOIN users u ON u.id = r.creator_user_id
		JOIN participants p_filter ON p_filter.route_id = r.id AND p_filter.user_id = ? AND p_filter.deleted_at IS NULL
		WHERE r.deleted_at IS NULL`

	var timeFilter string
	switch filter {
	case RouteFilterActive:
		timeFilter = " AND (r.leaving_at IS NULL OR r.leaving_at > NOW())"
	case RouteFilterPast:
		timeFilter = " AND r.leaving_at IS NOT NULL AND r.leaving_at <= NOW()"
	default:
		timeFilter = ""
	}

	whereClause := timeFilter
	return fetchRoutesWithDetails(ctx, db, baseQuery, whereClause, userID.String())
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
