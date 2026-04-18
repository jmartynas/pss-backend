package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/nats-io/nats.go"
)

type routeRepository struct {
	db *sql.DB
	nc *nats.Conn
}

// NewRouteRepository returns a domain.RouteRepository backed by MySQL.
func NewRouteRepository(db *sql.DB, nc *nats.Conn) domain.RouteRepository {
	return &routeRepository{db: db, nc: nc}
}

// routeColumns are the SELECT columns used by all route queries.
var routeColumns = []string{
	"r.id",
	"r.creator_user_id",
	"COALESCE(u.name, u.email, '')",
	"r.vehicle_id",
	"r.description",
	"r.start_lat", "r.start_lng",
	"r.start_place_id", "r.start_formatted_address",
	"r.end_lat", "r.end_lng",
	"r.end_place_id", "r.end_formatted_address",
	"r.max_passengers",
	"r.max_deviation",
	"GREATEST(0, r.max_passengers - (SELECT COUNT(*) FROM participants p WHERE p.route_id = r.id AND p.status = 'approved' AND p.deleted_at IS NULL)) AS available_passengers",
	"r.price",
	"r.leaving_at",
}

func routeBaseSelect() sq.SelectBuilder {
	return sq.Select(routeColumns...).
		From("routes r").
		Join("users u ON u.id = r.creator_user_id")
}

func scanRouteRow(row sq.RowScanner) (domain.Route, string, string, error) {
	var d domain.Route
	var idStr, creatorIDStr string
	var vehicleIDStr *string
	err := row.Scan(
		&idStr, &creatorIDStr, &d.CreatorName, &vehicleIDStr, &d.Description,
		&d.StartLat, &d.StartLng, &d.StartPlaceID, &d.StartFormattedAddress,
		&d.EndLat, &d.EndLng, &d.EndPlaceID, &d.EndFormattedAddress,
		&d.MaxPassengers, &d.MaxDeviation, &d.AvailablePassengers,
		&d.Price, &d.LeavingAt,
	)
	if vehicleIDStr != nil {
		parsed, _ := uuid.Parse(*vehicleIDStr)
		d.VehicleID = &parsed
	}
	return d, idStr, creatorIDStr, err
}

func (r *routeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	d, idStr, creatorIDStr, err := scanRouteRow(
		routeBaseSelect().
			Where(sq.Eq{"r.id": id.String(), "r.deleted_at": nil}).
			RunWith(r.db).QueryRowContext(ctx),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("route get by id: %w", err)
	}
	d.ID, _ = uuid.Parse(idStr)
	d.CreatorID, _ = uuid.Parse(creatorIDStr)

	stopsMap, err := fetchRouteStops(ctx, r.db, []string{idStr})
	if err != nil {
		return nil, err
	}
	d.Stops = stopsMap[idStr]
	if d.Stops == nil {
		d.Stops = []domain.Stop{}
	}

	participants, err := fetchRouteParticipants(ctx, r.db, []string{idStr})
	if err != nil {
		return nil, err
	}
	d.Participants = []domain.Participant{}
	for _, pr := range participants {
		d.Participants = append(d.Participants, pr.Participant)
	}
	return &d, nil
}

func (r *routeRepository) Create(ctx context.Context, creatorID uuid.UUID, in domain.CreateRouteInput) (uuid.UUID, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("route create: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	id := uuid.New()
	var vehicleIDVal interface{}
	if in.VehicleID != nil {
		vehicleIDVal = in.VehicleID.String()
	}
	_, err = sq.Insert("routes").
		Columns(
			"id", "creator_user_id", "vehicle_id", "description",
			"start_lat", "start_lng", "start_place_id", "start_formatted_address",
			"end_lat", "end_lng", "end_place_id", "end_formatted_address",
			"max_passengers", "max_deviation", "price", "leaving_at",
		).
		Values(
			id.String(), creatorID.String(), vehicleIDVal, nullablePtr(in.Description),
			in.StartLat, in.StartLng, nullablePtr(in.StartPlaceID), nullablePtr(in.StartFormattedAddress),
			in.EndLat, in.EndLng, nullablePtr(in.EndPlaceID), nullablePtr(in.EndFormattedAddress),
			in.MaxPassengers, in.MaxDeviation, nullableFloat(in.Price), nullableTime(in.LeavingAt),
		).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("route create: %w", err)
	}

	// Creator becomes a participant with status='driver'.
	_, err = sq.Insert("participants").
		Columns("id", "user_id", "route_id", "status").
		Values(uuid.New().String(), creatorID.String(), id.String(), "driver").
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("route create driver participant: %w", err)
	}

	for i, s := range in.Stops {
		_, err = sq.Insert("route_stops").
			Columns("id", "route_id", "position", "lat", "lng", "place_id", "formatted_address").
			Values(uuid.New().String(), id.String(), uint(i), s.Lat, s.Lng, nullablePtr(s.PlaceID), nullablePtr(s.FormattedAddress)).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return uuid.Nil, fmt.Errorf("route create stop %d: %w", i, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("route create: commit: %w", err)
	}
	return id, nil
}

func (r *routeRepository) Update(ctx context.Context, id, creatorID uuid.UUID, in domain.UpdateRouteInput) error {
	var ownerIDStr string
	err := sq.Select("creator_user_id").
		From("routes").
		Where(sq.Eq{"id": id.String(), "deleted_at": nil}).
		RunWith(r.db).QueryRowContext(ctx).Scan(&ownerIDStr)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("route update: fetch owner: %w", err)
	}
	if ownerIDStr != creatorID.String() {
		return errs.ErrForbidden
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("route update: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	ub := sq.Update("routes").Where(sq.Eq{"id": id.String()})
	if in.Description != nil {
		ub = ub.Set("description", nullablePtr(in.Description))
	}
	if in.MaxPassengers != nil {
		ub = ub.Set("max_passengers", *in.MaxPassengers)
	}
	if in.MaxDeviation != nil {
		ub = ub.Set("max_deviation", *in.MaxDeviation)
	}
	if in.Price != nil {
		ub = ub.Set("price", *in.Price)
	}
	if in.LeavingAt != nil {
		ub = ub.Set("leaving_at", nullableTime(in.LeavingAt))
	}
	if in.StartLat != nil {
		ub = ub.Set("start_lat", *in.StartLat).
			Set("start_lng", *in.StartLng).
			Set("start_place_id", nullablePtr(in.StartPlaceID)).
			Set("start_formatted_address", nullablePtr(in.StartFormattedAddress))
	}
	if in.EndLat != nil {
		ub = ub.Set("end_lat", *in.EndLat).
			Set("end_lng", *in.EndLng).
			Set("end_place_id", nullablePtr(in.EndPlaceID)).
			Set("end_formatted_address", nullablePtr(in.EndFormattedAddress))
	}

	sqlStr, args, sqlErr := ub.ToSql()
	if sqlErr == nil && len(args) > 1 {
		if _, err = tx.ExecContext(ctx, sqlStr, args...); err != nil {
			return fmt.Errorf("route update: %w", err)
		}
	}

	if in.Stops != nil {
		// Only delete driver-owned stops (participant_id IS NULL); passenger stops are untouched.
		if _, err = tx.ExecContext(ctx, "DELETE FROM route_stops WHERE route_id = ? AND participant_id IS NULL", id.String()); err != nil {
			return fmt.Errorf("route update: delete stops: %w", err)
		}
		for i, s := range *in.Stops {
			_, err = sq.Insert("route_stops").
				Columns("id", "route_id", "position", "lat", "lng", "place_id", "formatted_address").
				Values(uuid.New().String(), id.String(), uint(i), s.Lat, s.Lng, nullablePtr(s.PlaceID), nullablePtr(s.FormattedAddress)).
				RunWith(tx).ExecContext(ctx)
			if err != nil {
				return fmt.Errorf("route update: insert stop %d: %w", i, err)
			}
		}
	}

	// Find driver participant and create a request + email_log for this update.
	var driverParticipantID string
	err = sq.Select("id").From("participants").
		Where(sq.Eq{"route_id": id.String(), "status": "driver", "deleted_at": nil}).
		RunWith(tx).QueryRowContext(ctx).Scan(&driverParticipantID)
	if err != nil {
		return fmt.Errorf("route update: find driver participant: %w", err)
	}
	requestID := uuid.New().String()
	_, err = sq.Insert("requests").
		Columns("id", "participant_id").
		Values(requestID, driverParticipantID).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("route update: insert request: %w", err)
	}
	emailLogID := uuid.New().String()
	_, err = sq.Insert("email_logs").
		Columns("id", "request_id", "type", "status").
		Values(emailLogID, requestID, "route_updated", "created").
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("route update: insert email_log: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("route update: commit: %w", err)
	}
	publishEmailLog(r.nc, emailLogID, "route_updated")
	return nil
}

func (r *routeRepository) Delete(ctx context.Context, id, creatorID uuid.UUID) error {
	var ownerIDStr string
	err := sq.Select("creator_user_id").
		From("routes").
		Where(sq.Eq{"id": id.String(), "deleted_at": nil}).
		RunWith(r.db).QueryRowContext(ctx).Scan(&ownerIDStr)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("route delete: fetch owner: %w", err)
	}
	if ownerIDStr != creatorID.String() {
		return errs.ErrForbidden
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("route delete: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err = sq.Update("routes").
		Set("deleted_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id.String()}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return fmt.Errorf("route delete: %w", err)
	}

	emailLogID, err := cancelRouteCleanup(ctx, tx, id.String())
	if err != nil {
		return fmt.Errorf("route delete: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("route delete: commit: %w", err)
	}
	publishEmailLog(r.nc, emailLogID, "route_cancelled")
	return nil
}

// cancelRouteCleanup deletes all related data for a soft-deleted route within an existing
// transaction and returns the email_log ID for NATS notification.
func cancelRouteCleanup(ctx context.Context, tx *sql.Tx, routeID string) (string, error) {
	// Collect all participant IDs + driver/passenger user IDs.
	pRows, err := sq.Select("id", "user_id", "status").
		From("participants").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).QueryContext(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch participants: %w", err)
	}
	var allParticipantIDs []string
	var driverParticipantID, driverUserID string
	var passengerUserIDs []string
	for pRows.Next() {
		var pid, uid, status string
		if err := pRows.Scan(&pid, &uid, &status); err != nil {
			pRows.Close()
			return "", fmt.Errorf("scan participant: %w", err)
		}
		allParticipantIDs = append(allParticipantIDs, pid)
		if status == "driver" {
			driverParticipantID, driverUserID = pid, uid
		} else {
			passengerUserIDs = append(passengerUserIDs, uid)
		}
	}
	pRows.Close()
	if err := pRows.Err(); err != nil {
		return "", fmt.Errorf("iter participants: %w", err)
	}

	// Delete existing requests (request_stops cascade).
	if len(allParticipantIDs) > 0 {
		if _, err = sq.Delete("requests").
			Where(sq.Eq{"participant_id": allParticipantIDs}).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("delete requests: %w", err)
		}
	}

	// Insert cancellation request + email_log for notification.
	emailLogID := uuid.New().String()
	if driverParticipantID != "" {
		requestID := uuid.New().String()
		if _, err = sq.Insert("requests").
			Columns("id", "participant_id").
			Values(requestID, driverParticipantID).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("insert request: %w", err)
		}
		if _, err = sq.Insert("email_logs").
			Columns("id", "request_id", "type", "status").
			Values(emailLogID, requestID, "route_cancelled", "created").
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("insert email_log: %w", err)
		}
	}

	// Delete route group messages.
	if _, err = sq.Delete("route_messages").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return "", fmt.Errorf("delete route_messages: %w", err)
	}

	// Delete private chats between driver and passengers (private_messages cascade).
	if driverUserID != "" && len(passengerUserIDs) > 0 {
		if _, err = sq.Delete("private_chats").
			Where(sq.Or{
				sq.And{sq.Eq{"user1_id": driverUserID}, sq.Eq{"user2_id": passengerUserIDs}},
				sq.And{sq.Eq{"user2_id": driverUserID}, sq.Eq{"user1_id": passengerUserIDs}},
			}).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("delete private_chats: %w", err)
		}
	}

	// Delete route stops.
	if _, err = sq.Delete("route_stops").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return "", fmt.Errorf("delete route_stops: %w", err)
	}

	// Soft-delete all participants.
	if len(allParticipantIDs) > 0 {
		if _, err = sq.Update("participants").
			Set("deleted_at", sq.Expr("NOW()")).
			Where(sq.Eq{"route_id": routeID, "deleted_at": nil}).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("soft-delete participants: %w", err)
		}
	}

	return emailLogID, nil
}

func (r *routeRepository) ListByCreator(ctx context.Context, creatorID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error) {
	qb := routeBaseSelect().
		Where(sq.Eq{"r.creator_user_id": creatorID.String(), "r.deleted_at": nil})
	qb = applyRouteTimeFilter(qb, filter)
	rows, err := qb.OrderBy("r.created_at DESC").RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("route list by creator: %w", err)
	}
	defer rows.Close()
	return scanRoutesWithDetails(ctx, r.db, rows)
}

func (r *routeRepository) ListByParticipant(ctx context.Context, userID uuid.UUID, filter domain.RouteFilter) ([]domain.Route, error) {
	qb := routeBaseSelect().
		Join("participants p_filter ON p_filter.route_id = r.id AND p_filter.deleted_at IS NULL AND p_filter.status IN ('driver','approved')").
		Where(sq.Eq{"p_filter.user_id": userID.String(), "r.deleted_at": nil})
	qb = applyRouteTimeFilter(qb, filter)
	rows, err := qb.OrderBy("r.created_at DESC").RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("route list by participant: %w", err)
	}
	defer rows.Close()
	return scanRoutesWithDetails(ctx, r.db, rows)
}

func (r *routeRepository) ListSearchable(ctx context.Context) ([]domain.Route, error) {
	rows, err := routeBaseSelect().
		Where(sq.And{
			sq.Eq{"r.deleted_at": nil},
			sq.Expr("r.max_passengers > (SELECT COUNT(*) FROM participants p WHERE p.route_id = r.id AND p.deleted_at IS NULL)"),
			sq.Or{
				sq.Eq{"r.leaving_at": nil},
				sq.Expr("r.leaving_at > NOW()"),
			},
		}).
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("route list searchable: %w", err)
	}
	defer rows.Close()
	return scanRoutesWithDetails(ctx, r.db, rows)
}

func applyRouteTimeFilter(qb sq.SelectBuilder, filter domain.RouteFilter) sq.SelectBuilder {
	switch filter {
	case domain.RouteFilterActive:
		return qb.Where(sq.Or{sq.Eq{"r.leaving_at": nil}, sq.Expr("r.leaving_at > NOW()")})
	case domain.RouteFilterPast:
		return qb.Where(sq.And{sq.NotEq{"r.leaving_at": nil}, sq.Expr("r.leaving_at <= NOW()")})
	}
	return qb
}

// scanRoutesWithDetails reads route rows then batch-fetches stops and participants.
func scanRoutesWithDetails(ctx context.Context, db *sql.DB, rows *sql.Rows) ([]domain.Route, error) {
	routesMap := make(map[string]*domain.Route)
	var routeIDs []string

	for rows.Next() {
		d, idStr, creatorIDStr, err := scanRouteRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		d.ID, _ = uuid.Parse(idStr)
		d.CreatorID, _ = uuid.Parse(creatorIDStr)
		d.Stops = []domain.Stop{}
		d.Participants = []domain.Participant{}
		routesMap[idStr] = &d
		routeIDs = append(routeIDs, idStr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan routes: %w", err)
	}
	if len(routeIDs) == 0 {
		return []domain.Route{}, nil
	}

	stopsMap, err := fetchRouteStops(ctx, db, routeIDs)
	if err != nil {
		return nil, err
	}
	for routeIDStr, stops := range stopsMap {
		if r, ok := routesMap[routeIDStr]; ok {
			r.Stops = stops
		}
	}

	allParticipants, err := fetchRouteParticipants(ctx, db, routeIDs)
	if err != nil {
		return nil, err
	}
	for _, pr := range allParticipants {
		if r, ok := routesMap[pr.routeID]; ok {
			r.Participants = append(r.Participants, pr.Participant)
		}
	}

	result := make([]domain.Route, 0, len(routeIDs))
	for _, id := range routeIDs {
		if r, ok := routesMap[id]; ok {
			result = append(result, *r)
		}
	}
	return result, nil
}

// fetchRouteStops batch-fetches stops for the given route IDs.
func fetchRouteStops(ctx context.Context, db *sql.DB, routeIDs []string) (map[string][]domain.Stop, error) {
	rows, err := sq.Select("route_id", "id", "position", "lat", "lng", "place_id", "formatted_address", "participant_id").
		From("route_stops").
		Where(sq.Eq{"route_id": routeIDs}).
		OrderBy("route_id", "position").
		RunWith(db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch route stops: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]domain.Stop)
	for rows.Next() {
		var routeIDStr, stopIDStr string
		var s domain.Stop
		if err := rows.Scan(&routeIDStr, &stopIDStr, &s.Position, &s.Lat, &s.Lng, &s.PlaceID, &s.FormattedAddress, &s.ParticipantID); err != nil {
			return nil, fmt.Errorf("scan route stop: %w", err)
		}
		s.ID, _ = uuid.Parse(stopIDStr)
		result[routeIDStr] = append(result[routeIDStr], s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch route stops: %w", err)
	}
	return result, nil
}

type participantWithRoute struct {
	routeID string
	domain.Participant
}

// fetchRouteParticipants batch-fetches participants for the given route IDs.
func fetchRouteParticipants(ctx context.Context, db *sql.DB, routeIDs []string) ([]participantWithRoute, error) {
	rows, err := sq.Select("p.route_id", "p.user_id", "COALESCE(u.name, '')", "p.status").
		From("participants p").
		Join("users u ON u.id = p.user_id").
		Where(sq.Eq{"p.route_id": routeIDs, "p.deleted_at": nil}).
		Where(sq.Expr("p.status IN ('driver', 'approved')")).
		OrderBy("p.route_id", "p.created_at ASC").
		RunWith(db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch route participants: %w", err)
	}
	defer rows.Close()

	var result []participantWithRoute
	for rows.Next() {
		var pr participantWithRoute
		var userIDStr string
		if err := rows.Scan(&pr.routeID, &userIDStr, &pr.Name, &pr.Status); err != nil {
			return nil, fmt.Errorf("scan participant: %w", err)
		}
		pr.UserID, _ = uuid.Parse(userIDStr)
		result = append(result, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch route participants: %w", err)
	}
	return result, nil
}

// publishEmailLog publishes an "email" NATS message after an email_logs insert.
// Errors are silently ignored so a NATS hiccup never rolls back a DB transaction.
func publishEmailLog(nc *nats.Conn, emailLogID, emailType string) {
	if nc == nil {
		return
	}
	payload := fmt.Sprintf(`{"id":%q,"type":%q}`, emailLogID, emailType)
	nc.Publish("email", []byte(payload)) //nolint:errcheck
}

func nullablePtr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func nullableFloat(f *float64) interface{} {
	if f == nil {
		return nil
	}
	return *f
}
