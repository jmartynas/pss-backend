package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
)

func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	type userRow struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Provider  string `json:"provider"`
		CreatedAt string `json:"created_at"`
	}

	rows, err := sq.Select("id", "email", "COALESCE(name, '')", "status", "provider", "created_at").
		From("users").
		OrderBy("created_at DESC").
		Limit(1000).
		RunWith(h.db).QueryContext(r.Context())
	if err != nil {
		h.log.Error("list users", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	users := []userRow{}
	for rows.Next() {
		var u userRow
		var createdAt time.Time

		err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Status, &u.Provider, &createdAt)
		if err != nil {
			h.log.Error("scan user", slog.Any("error", err))
			continue
		}

		u.CreatedAt = createdAt.Format(time.RFC3339)
		users = append(users, u)
	}

	writeJSON(w, http.StatusOK, users)
}

func (h *handler) blockUser(w http.ResponseWriter, r *http.Request) {
	h.setUserStatus(w, r, "blocked")
}

func (h *handler) unblockUser(w http.ResponseWriter, r *http.Request) {
	h.setUserStatus(w, r, "active")
}

func (h *handler) setUserStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := r.PathValue("id")
	if status == "blocked" {
		var current string
		err := sq.Select("status").From("users").Where(sq.Eq{"id": id}).
			RunWith(h.db).QueryRowContext(r.Context()).Scan(&current)
		if err != nil || current == "inactive" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "cannot block inactive user"})
			return
		}
	}

	_, err := sq.Update("users").
		Set("status", status).
		Where(sq.Eq{"id": id}).
		RunWith(h.db).ExecContext(r.Context())
	if err != nil {
		h.log.Error("set user status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if status == "blocked" {
		h.cancelDriverRoutes(r.Context(), id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (h *handler) cancelDriverRoutes(ctx context.Context, userID string) {
	rows, err := sq.Select("r.id").
		From("routes r").
		Join("participants p ON p.route_id = r.id").
		Where(sq.Eq{"p.user_id": userID, "p.status": "driver", "p.deleted_at": nil, "r.deleted_at": nil}).
		RunWith(h.db).QueryContext(ctx)
	if err != nil {
		h.log.Error("find driver routes", slog.Any("error", err))
		return
	}
	defer rows.Close()

	var routeIDs []string
	for rows.Next() {
		var rid string
		err := rows.Scan(&rid)
		if err == nil {
			routeIDs = append(routeIDs, rid)
		}
	}
	rows.Close()

	for _, rid := range routeIDs {
		if err := h.cancelRoute(ctx, rid); err != nil {
			h.log.Error("cancel driver route", slog.String("route_id", rid), slog.Any("error", err))
		}
	}
}
