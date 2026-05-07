package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/nats-io/nats.go"
)

type applicationRepository struct {
	db *sql.DB
	nc *nats.Conn
}

func NewApplicationRepository(db *sql.DB, nc *nats.Conn) domain.ApplicationRepository {
	return &applicationRepository{db: db, nc: nc}
}

func (r *applicationRepository) Create(ctx context.Context, userID, routeID uuid.UUID, in domain.ApplyInput) (uuid.UUID, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("application create: begin tx: %w", err)
	}
	defer tx.Rollback()

	participantID := uuid.New()
	_, err = sq.Insert("participants").
		Columns("id", "user_id", "route_id", "status").
		Values(participantID.String(), userID.String(), routeID.String(), "pending").
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return uuid.Nil, errs.ErrAlreadyApplied
		}

		return uuid.Nil, fmt.Errorf("application create: %w", err)
	}

	requestID := uuid.New()
	_, err = sq.Insert("requests").
		Columns("id", "participant_id", "comment").
		Values(requestID.String(), participantID.String(), nullablePtr(in.Comment)).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("application create request: %w", err)
	}

	for i, s := range in.Stops {
		_, err = sq.Insert("request_stops").
			Columns("id", "request_id", "position", "lat", "lng", "place_id", "formatted_address", "route_stop_id").
			Values(uuid.New().String(), requestID.String(), s.Position, s.Lat, s.Lng, nullablePtr(s.PlaceID), nullablePtr(s.FormattedAddress), nullablePtr(s.RouteStopID)).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return uuid.Nil, fmt.Errorf("application create stop %d: %w", i, err)
		}
	}

	var driverParticipantID string
	err = sq.Select("id").From("participants").
		Where(sq.Eq{"route_id": routeID.String(), "status": "driver", "deleted_at": nil}).
		RunWith(tx).QueryRowContext(ctx).Scan(&driverParticipantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("application create: find driver participant: %w", err)
	}

	_, err = sq.Insert("private_chats").
		Columns("id", "user1_id", "user2_id").
		Values(uuid.New().String(), driverParticipantID, participantID.String()).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("application create: create private chat: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("application create: commit: %w", err)
	}

	return participantID, nil
}

func (r *applicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Application, error) {
	var a domain.Application
	var idStr, userIDStr, routeIDStr string

	err := sq.Select("p.id", "p.user_id", "COALESCE(u.name, u.email, '')", "p.route_id", "p.status", "req.comment", "p.created_at", "p.pending_stop_change").
		From("participants p").
		Join("users u ON u.id = p.user_id").
		LeftJoin("requests req ON req.participant_id = p.id").
		Where(sq.Eq{"p.id": id.String(), "p.deleted_at": nil}).
		Where("p.status != 'driver'").
		RunWith(r.db).QueryRowContext(ctx).
		Scan(&idStr, &userIDStr, &a.UserName, &routeIDStr, &a.Status, &a.Comment, &a.CreatedAt, &a.PendingStopChange)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, errs.ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("application get by id: %w", err)
	}

	a.ID, _ = uuid.Parse(idStr)
	a.UserID, _ = uuid.Parse(userIDStr)
	a.RouteID, _ = uuid.Parse(routeIDStr)

	stops, err := fetchApplicationStops(ctx, r.db, []string{idStr})
	if err != nil {
		return nil, err
	}

	a.Stops = []domain.ApplicationStop{}
	for _, s := range stops {
		a.Stops = append(a.Stops, s.ApplicationStop)
	}

	return &a, nil
}

func (r *applicationRepository) GetByUserAndRoute(ctx context.Context, userID, routeID uuid.UUID) (*domain.Application, error) {
	var idStr string
	err := sq.Select("id").
		From("participants").
		Where(sq.Eq{"user_id": userID.String(), "route_id": routeID.String(), "deleted_at": nil}).
		Where("status != 'driver'").
		RunWith(r.db).QueryRowContext(ctx).Scan(&idStr)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("application get by user and route: %w", err)
	}

	parsed, _ := uuid.Parse(idStr)
	return r.GetByID(ctx, parsed)
}

func (r *applicationRepository) ListByRoute(ctx context.Context, routeID uuid.UUID) ([]domain.Application, error) {
	rows, err := sq.Select("p.id", "p.user_id", "COALESCE(u.name, u.email, '')", "p.route_id", "p.status", "req.comment", "p.created_at", "p.pending_stop_change").
		From("participants p").
		Join("users u ON u.id = p.user_id").
		LeftJoin("requests req ON req.participant_id = p.id").
		Where(sq.Eq{"p.route_id": routeID.String(), "p.deleted_at": nil}).
		Where("p.status != 'driver'").
		OrderBy("p.created_at ASC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("application list by route: %w", err)
	}
	defer rows.Close()

	return scanApplicationsWithStops(ctx, r.db, rows)
}

func (r *applicationRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Application, error) {
	rows, err := sq.Select(
		"p.id", "p.user_id", "COALESCE(u.name, u.email, '')", "p.route_id", "p.status", "req.comment", "p.created_at", "p.pending_stop_change",
		"ro.leaving_at", "ro.start_formatted_address", "ro.end_formatted_address",
	).
		From("participants p").
		Join("users u ON u.id = p.user_id").
		LeftJoin("requests req ON req.participant_id = p.id").
		Join("routes ro ON ro.id = p.route_id").
		Where(sq.Eq{"p.user_id": userID.String()}).
		Where(sq.Or{
			sq.Eq{"p.deleted_at": nil},
			sq.Eq{"p.status": "left"},
		}).
		Where("p.status != 'driver'").
		OrderBy("p.created_at DESC").
		RunWith(r.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("application list by user: %w", err)
	}
	defer rows.Close()

	return scanUserApplicationsWithStops(ctx, r.db, rows)
}

func (r *applicationRepository) ReviewUpdate(ctx context.Context, id uuid.UUID, status string, appUserID, routeID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("application review: begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = sq.Update("participants").
		Set("status", status).
		Where(sq.Eq{"id": id.String()}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("application review: update status: %w", err)
	}

	if status == "approved" {
		pRows, err := sq.Select("id", "participant_id").
			From("route_stops").
			Where(sq.Eq{"route_id": routeID.String()}).
			RunWith(tx).QueryContext(ctx)
		if err != nil {
			return fmt.Errorf("application review: snapshot route stops: %w", err)
		}

		participantByStopID := make(map[string]*string)
		for pRows.Next() {
			var stopID string
			var pID *string
			if err := pRows.Scan(&stopID, &pID); err != nil {
				pRows.Close()
				return fmt.Errorf("application review: scan route stop: %w", err)
			}

			participantByStopID[stopID] = pID
		}
		pRows.Close()
		if err := pRows.Err(); err != nil {
			return fmt.Errorf("application review: read route stops: %w", err)
		}

		rows, err := sq.Select("rs.position", "rs.lat", "rs.lng", "rs.place_id", "rs.formatted_address", "rs.route_stop_id").
			From("request_stops rs").
			Join("requests req ON req.id = rs.request_id").
			Where(sq.Eq{"req.participant_id": id.String()}).
			OrderBy("rs.position ASC").
			RunWith(tx).QueryContext(ctx)
		if err != nil {
			return fmt.Errorf("application review: fetch request stops: %w", err)
		}

		type stopRow struct {
			position         uint
			lat, lng         float64
			placeID          *string
			formattedAddress *string
			routeStopID      *string
		}
		var newStops []stopRow

		for rows.Next() {
			var s stopRow

			if err := rows.Scan(&s.position, &s.lat, &s.lng, &s.placeID, &s.formattedAddress, &s.routeStopID); err != nil {
				rows.Close()
				return fmt.Errorf("application review: scan request stop: %w", err)
			}

			newStops = append(newStops, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("application review: read request stops: %w", err)
		}

		_, err = sq.Delete("route_stops").
			Where(sq.Eq{"route_id": routeID.String()}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("application review: clear route stops: %w", err)
		}

		appIDStr := id.String()
		for i, s := range newStops {
			var participantID *string

			if s.routeStopID != nil {
				participantID = participantByStopID[*s.routeStopID]
			} else {
				participantID = &appIDStr
			}

			_, err = sq.Insert("route_stops").
				Columns("id", "route_id", "position", "lat", "lng", "place_id", "formatted_address", "participant_id").
				Values(uuid.New().String(), routeID.String(), s.position, s.lat, s.lng, s.placeID, s.formattedAddress, participantID).
				RunWith(tx).ExecContext(ctx)
			if err != nil {
				return fmt.Errorf("application review: insert route stop %d: %w", i, err)
			}
		}
	}

	if status == "approved" {
		var requestID string
		err := sq.Select("id").From("requests").
			Where(sq.Eq{"participant_id": id.String()}).
			RunWith(tx).QueryRowContext(ctx).Scan(&requestID)
		if err != nil {
			return fmt.Errorf("application review: find request for email log: %w", err)
		}

		emailLogID := uuid.New().String()
		_, err = sq.Insert("email_logs").
			Columns("id", "request_id", "type", "status").
			Values(emailLogID, requestID, "application_approved", "created").
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("application review: insert email_log: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("application review: commit: %w", err)
		}
		publishEmailLog(r.nc, emailLogID, "application_approved")

		return nil
	}

	return tx.Commit()
}

func (r *applicationRepository) UpdateStops(ctx context.Context, id uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("application update stops: begin tx: %w", err)
	}
	defer tx.Rollback()

	var requestIDStr string
	err = sq.Select("id").From("requests").
		Where(sq.Eq{"participant_id": id.String()}).
		RunWith(tx).QueryRowContext(ctx).Scan(&requestIDStr)
	if err != nil {
		return fmt.Errorf("application update stops: find request: %w", err)
	}

	_, err = sq.Delete("request_stops").
		Where(sq.Eq{"request_id": requestIDStr}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("application update stops: clear stops: %w", err)
	}

	for i, s := range stops {
		_, err = sq.Insert("request_stops").
			Columns("id", "request_id", "position", "lat", "lng", "place_id", "formatted_address", "route_stop_id").
			Values(uuid.New().String(), requestIDStr, s.Position, s.Lat, s.Lng, nullablePtr(s.PlaceID), nullablePtr(s.FormattedAddress), nullablePtr(s.RouteStopID)).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("application update stops: insert stop %d: %w", i, err)
		}
	}

	if comment != nil {
		_, err = sq.Update("requests").
			Set("comment", nullablePtr(comment)).
			Where(sq.Eq{"id": requestIDStr}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("application update stops: update comment: %w", err)
		}
	}

	return tx.Commit()
}

func (r *applicationRepository) SoftDelete(ctx context.Context, id uuid.UUID, wasApproved bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("application delete: begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = sq.Delete("requests").
		Where(sq.Eq{"participant_id": id.String()}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("application delete: remove request: %w", err)
	}

	if wasApproved {
		_, err = sq.Update("participants").
			Set("status", "left").
			Set("deleted_at", sq.Expr("NOW()")).
			Where(sq.Eq{"id": id.String()}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("application delete: mark left: %w", err)
		}
	} else {
		_, err = sq.Delete("participants").
			Where(sq.Eq{"id": id.String()}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("application delete: remove participant: %w", err)
		}
	}

	return tx.Commit()
}

func (r *applicationRepository) RequestStopChange(ctx context.Context, id uuid.UUID, stops []domain.ApplicationStopInput, comment *string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("request stop change: begin tx: %w", err)
	}
	defer tx.Rollback()

	var requestIDStr string
	err = sq.Select("id").From("requests").
		Where(sq.Eq{"participant_id": id.String()}).
		RunWith(tx).QueryRowContext(ctx).Scan(&requestIDStr)
	if err != nil {
		return fmt.Errorf("request stop change: find request: %w", err)
	}

	_, err = sq.Delete("request_stops").
		Where(sq.Eq{"request_id": requestIDStr}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("request stop change: clear stops: %w", err)
	}

	for i, s := range stops {
		_, err = sq.Insert("request_stops").
			Columns("id", "request_id", "position", "lat", "lng", "place_id", "formatted_address", "route_stop_id").
			Values(uuid.New().String(), requestIDStr, s.Position, s.Lat, s.Lng, nullablePtr(s.PlaceID), nullablePtr(s.FormattedAddress), s.RouteStopID).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("request stop change: insert stop %d: %w", i, err)
		}
	}

	_, err = sq.Update("requests").
		Set("comment", nullablePtr(comment)).
		Where(sq.Eq{"id": requestIDStr}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("request stop change: update comment: %w", err)
	}

	_, err = sq.Update("participants").
		Set("pending_stop_change", 1).
		Where(sq.Eq{"id": id.String()}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("request stop change: set flag: %w", err)
	}

	return tx.Commit()
}

func (r *applicationRepository) ReviewStopChange(ctx context.Context, id uuid.UUID, routeID uuid.UUID, approve bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("review stop change: begin tx: %w", err)
	}
	defer tx.Rollback()

	if approve {
		participantByStopID := make(map[string]*string)
		snapRows, err := sq.Select("id", "participant_id").
			From("route_stops").
			Where(sq.Eq{"route_id": routeID.String()}).
			RunWith(tx).QueryContext(ctx)
		if err != nil {
			return fmt.Errorf("review stop change: snapshot route stops: %w", err)
		}

		for snapRows.Next() {
			var stopID string
			var participantID *string

			if err := snapRows.Scan(&stopID, &participantID); err != nil {
				snapRows.Close()
				return fmt.Errorf("review stop change: scan snapshot: %w", err)
			}

			participantByStopID[stopID] = participantID
		}
		snapRows.Close()
		if err := snapRows.Err(); err != nil {
			return fmt.Errorf("review stop change: read snapshot: %w", err)
		}

		rows, err := sq.Select("rs.position", "rs.lat", "rs.lng", "rs.place_id", "rs.formatted_address", "rs.route_stop_id").
			From("request_stops rs").
			Join("requests req ON req.id = rs.request_id").
			Where(sq.Eq{"req.participant_id": id.String()}).
			OrderBy("rs.position ASC").
			RunWith(tx).QueryContext(ctx)
		if err != nil {
			return fmt.Errorf("review stop change: fetch request stops: %w", err)
		}

		type stopRow struct {
			position         uint
			lat, lng         float64
			placeID          *string
			formattedAddress *string
			routeStopID      *string
		}
		var newStops []stopRow

		for rows.Next() {
			var s stopRow
			if err := rows.Scan(&s.position, &s.lat, &s.lng, &s.placeID, &s.formattedAddress, &s.routeStopID); err != nil {
				rows.Close()
				return fmt.Errorf("review stop change: scan stop: %w", err)
			}
			newStops = append(newStops, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("review stop change: read stops: %w", err)
		}

		_, err = sq.Delete("route_stops").
			Where(sq.Eq{"route_id": routeID.String()}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("review stop change: clear route stops: %w", err)
		}

		appIDStr := id.String()
		for i, s := range newStops {
			var participantID *string

			if s.routeStopID != nil {
				participantID = participantByStopID[*s.routeStopID]
			} else {
				participantID = &appIDStr
			}

			_, err = sq.Insert("route_stops").
				Columns("id", "route_id", "position", "lat", "lng", "place_id", "formatted_address", "participant_id").
				Values(uuid.New().String(), routeID.String(), s.position, s.lat, s.lng, s.placeID, s.formattedAddress, participantID).
				RunWith(tx).ExecContext(ctx)
			if err != nil {
				return fmt.Errorf("review stop change: insert route stop %d: %w", i, err)
			}
		}
	} else {
		var requestIDStr string
		err = sq.Select("id").From("requests").
			Where(sq.Eq{"participant_id": id.String()}).
			RunWith(tx).QueryRowContext(ctx).Scan(&requestIDStr)
		if err != nil {
			return fmt.Errorf("review stop change: find request: %w", err)
		}

		_, err = sq.Delete("request_stops").
			Where(sq.Eq{"request_id": requestIDStr}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("review stop change: clear proposed stops: %w", err)
		}
	}

	_, err = sq.Update("participants").
		Set("pending_stop_change", 0).
		Where(sq.Eq{"id": id.String()}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("review stop change: clear flag: %w", err)
	}

	if approve {
		var requestID string
		err = sq.Select("id").From("requests").
			Where(sq.Eq{"participant_id": id.String()}).
			RunWith(tx).QueryRowContext(ctx).Scan(&requestID)
		if err != nil {
			return fmt.Errorf("review stop change: find request for email log: %w", err)
		}

		emailLogID := uuid.New().String()
		_, err = sq.Insert("email_logs").
			Columns("id", "request_id", "type", "status").
			Values(emailLogID, requestID, "stop_change_approved", "created").
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return fmt.Errorf("review stop change: insert email_log: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("review stop change: commit: %w", err)
		}
		publishEmailLog(r.nc, emailLogID, "stop_change_approved")

		return nil
	}

	return tx.Commit()
}

func (r *applicationRepository) CancelStopChange(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("cancel stop change: begin tx: %w", err)
	}
	defer tx.Rollback()

	var requestIDStr string
	err = sq.Select("id").From("requests").
		Where(sq.Eq{"participant_id": id.String()}).
		RunWith(tx).QueryRowContext(ctx).Scan(&requestIDStr)
	if err != nil {
		return fmt.Errorf("cancel stop change: find request: %w", err)
	}

	_, err = sq.Delete("request_stops").
		Where(sq.Eq{"request_id": requestIDStr}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("cancel stop change: clear proposed stops: %w", err)
	}

	_, err = sq.Update("participants").
		Set("pending_stop_change", 0).
		Where(sq.Eq{"id": id.String()}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("cancel stop change: clear flag: %w", err)
	}

	return tx.Commit()
}

func scanApplicationsWithStops(ctx context.Context, db *sql.DB, rows *sql.Rows) ([]domain.Application, error) {
	var appIDs []string
	appMap := make(map[string]*domain.Application)

	for rows.Next() {
		var a domain.Application
		var idStr, userIDStr, routeIDStr string
		if err := rows.Scan(&idStr, &userIDStr, &a.UserName, &routeIDStr, &a.Status, &a.Comment, &a.CreatedAt, &a.PendingStopChange); err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}

		a.ID, _ = uuid.Parse(idStr)
		a.UserID, _ = uuid.Parse(userIDStr)
		a.RouteID, _ = uuid.Parse(routeIDStr)
		a.Stops = []domain.ApplicationStop{}
		appMap[idStr] = &a
		appIDs = append(appIDs, idStr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan applications: %w", err)
	}

	if len(appIDs) == 0 {
		return []domain.Application{}, nil
	}

	stops, err := fetchApplicationStops(ctx, db, appIDs)
	if err != nil {
		return nil, err
	}

	for _, s := range stops {
		if a, ok := appMap[s.appID]; ok {
			a.Stops = append(a.Stops, s.ApplicationStop)
		}
	}

	result := make([]domain.Application, 0, len(appIDs))
	for _, id := range appIDs {
		if a, ok := appMap[id]; ok {
			result = append(result, *a)
		}
	}

	return result, nil
}

func scanUserApplicationsWithStops(ctx context.Context, db *sql.DB, rows *sql.Rows) ([]domain.Application, error) {
	var appIDs []string
	appMap := make(map[string]*domain.Application)

	for rows.Next() {
		var a domain.Application
		var idStr, userIDStr, routeIDStr string
		err := rows.Scan(
			&idStr, &userIDStr, &a.UserName, &routeIDStr, &a.Status, &a.Comment, &a.CreatedAt, &a.PendingStopChange,
			&a.RouteLeavingAt, &a.RouteStartAddress, &a.RouteEndAddress,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user application: %w", err)
		}
		a.ID, _ = uuid.Parse(idStr)
		a.UserID, _ = uuid.Parse(userIDStr)
		a.RouteID, _ = uuid.Parse(routeIDStr)
		a.Stops = []domain.ApplicationStop{}
		appMap[idStr] = &a
		appIDs = append(appIDs, idStr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan user applications: %w", err)
	}

	if len(appIDs) == 0 {
		return []domain.Application{}, nil
	}

	stops, err := fetchApplicationStops(ctx, db, appIDs)
	if err != nil {
		return nil, err
	}

	for _, s := range stops {
		if a, ok := appMap[s.appID]; ok {
			a.Stops = append(a.Stops, s.ApplicationStop)
		}
	}

	result := make([]domain.Application, 0, len(appIDs))
	for _, id := range appIDs {
		if a, ok := appMap[id]; ok {
			result = append(result, *a)
		}
	}

	return result, nil
}

type appStopWithID struct {
	domain.ApplicationStop
	appID string
}

func fetchApplicationStops(ctx context.Context, db *sql.DB, participantIDs []string) ([]appStopWithID, error) {
	if len(participantIDs) == 0 {
		return nil, nil
	}

	rows, err := sq.Select("rs.id", "req.participant_id", "rs.position", "rs.lat", "rs.lng", "rs.place_id", "rs.formatted_address", "rs.route_stop_id").
		From("request_stops rs").
		Join("requests req ON req.id = rs.request_id").
		Where(sq.Eq{"req.participant_id": participantIDs}).
		OrderBy("req.participant_id", "rs.position").
		RunWith(db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch application stops: %w", err)
	}
	defer rows.Close()

	var result []appStopWithID
	for rows.Next() {
		var s appStopWithID
		var idStr, participantIDStr string

		if err := rows.Scan(&idStr, &participantIDStr, &s.Position, &s.Lat, &s.Lng, &s.PlaceID, &s.FormattedAddress, &s.RouteStopID); err != nil {
			return nil, fmt.Errorf("scan application stop: %w", err)
		}

		s.ID, _ = uuid.Parse(idStr)
		s.appID = participantIDStr
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch application stops: %w", err)
	}

	return result, nil
}
