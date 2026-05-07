package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

func (h *handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	type routeRow struct {
		ID        string  `json:"id"`
		From      string  `json:"from"`
		To        string  `json:"to"`
		LeavingAt *string `json:"leaving_at"`
		CreatedAt string  `json:"created_at"`
		Creator   string  `json:"creator"`
		Deleted   bool    `json:"deleted"`
	}

	rows, err := sq.Select(
		"r.id",
		"COALESCE(r.start_formatted_address, '')",
		"COALESCE(r.end_formatted_address, '')",
		"r.leaving_at",
		"r.created_at",
		"COALESCE(u.name, u.email)",
		"r.deleted_at IS NOT NULL",
	).From("routes r").
		Join("users u ON u.id = r.creator_user_id").
		Where(sq.Eq{"r.deleted_at": nil}).
		OrderBy("r.created_at DESC").
		Limit(1000).
		RunWith(h.db).QueryContext(r.Context())
	if err != nil {
		h.log.Error("list routes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	routes := []routeRow{}
	for rows.Next() {
		var rt routeRow
		var leavingAt sql.NullTime
		var createdAt time.Time

		err := rows.Scan(&rt.ID, &rt.From, &rt.To, &leavingAt, &createdAt, &rt.Creator, &rt.Deleted)
		if err != nil {
			h.log.Error("scan route", slog.Any("error", err))
			continue
		}

		rt.CreatedAt = createdAt.Format(time.RFC3339)
		if leavingAt.Valid {
			s := leavingAt.Time.Format(time.RFC3339)
			rt.LeavingAt = &s
		}

		routes = append(routes, rt)
	}

	writeJSON(w, http.StatusOK, routes)
}

func (h *handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.cancelRoute(r.Context(), r.PathValue("id")); err != nil {
		h.log.Error("delete route", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *handler) cancelRoute(ctx context.Context, routeID string) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := sq.Update("routes").
		Set("deleted_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": routeID, "deleted_at": nil}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("soft-delete route: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}

	emailLogID, err := cancelRouteCleanup(ctx, tx, routeID)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if h.nc != nil {
		payload := fmt.Sprintf(`{"id":%q,"type":"route_cancelled"}`, emailLogID)
		h.nc.Publish("email", []byte(payload))
	}

	return nil
}

func cancelRouteCleanup(ctx context.Context, tx *sql.Tx, routeID string) (string, error) {
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
		err := pRows.Scan(&pid, &uid, &status)
		if err != nil {
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

	if len(allParticipantIDs) > 0 {
		_, err = sq.Delete("requests").
			Where(sq.Eq{"participant_id": allParticipantIDs}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return "", fmt.Errorf("delete requests: %w", err)
		}
	}

	emailLogID := uuid.New().String()
	if driverParticipantID != "" {
		requestID := uuid.New().String()
		_, err = sq.Insert("requests").
			Columns("id", "participant_id").
			Values(requestID, driverParticipantID).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return "", fmt.Errorf("insert request: %w", err)
		}

		_, err = sq.Insert("email_logs").
			Columns("id", "request_id", "type", "status").
			Values(emailLogID, requestID, "route_cancelled", "created").
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return "", fmt.Errorf("insert email_log: %w", err)
		}
	}

	if _, err = sq.Delete("route_messages").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return "", fmt.Errorf("delete route_messages: %w", err)
	}

	if driverUserID != "" && len(passengerUserIDs) > 0 {
		_, err = sq.Delete("private_chats").
			Where(sq.Or{
				sq.And{sq.Eq{"user1_id": driverUserID}, sq.Eq{"user2_id": passengerUserIDs}},
				sq.And{sq.Eq{"user2_id": driverUserID}, sq.Eq{"user1_id": passengerUserIDs}},
			}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return "", fmt.Errorf("delete private_chats: %w", err)
		}
	}

	if _, err = sq.Delete("route_stops").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return "", fmt.Errorf("delete route_stops: %w", err)
	}

	if len(allParticipantIDs) > 0 {
		_, err = sq.Update("participants").
			Set("deleted_at", sq.Expr("NOW()")).
			Where(sq.Eq{"route_id": routeID, "deleted_at": nil}).
			RunWith(tx).ExecContext(ctx)
		if err != nil {
			return "", fmt.Errorf("soft-delete participants: %w", err)
		}
	}

	return emailLogID, nil
}
