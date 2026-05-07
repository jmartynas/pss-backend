package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (h *handler) listAdmins(w http.ResponseWriter, r *http.Request) {
	type adminRow struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Permissions uint8  `json:"permissions"`
		CreatedAt   string `json:"created_at"`
	}

	rows, err := sq.Select("id", "email", "permissions", "created_at").
		From("admins").
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("created_at ASC").
		RunWith(h.db).QueryContext(r.Context())
	if err != nil {
		h.log.Error("list admins", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	admins := []adminRow{}
	for rows.Next() {
		var a adminRow
		var createdAt time.Time

		err := rows.Scan(&a.ID, &a.Email, &a.Permissions, &createdAt)
		if err != nil {
			h.log.Error("scan admin", slog.Any("error", err))
			continue
		}

		a.CreatedAt = createdAt.Format(time.RFC3339)
		admins = append(admins, a)
	}

	writeJSON(w, http.StatusOK, admins)
}

func (h *handler) createAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Permissions uint8  `json:"permissions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error("hash password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	id := uuid.New().String()
	_, err = sq.Insert("admins").
		Columns("id", "email", "password", "status", "permissions").
		Values(id, req.Email, string(hash), 0, req.Permissions).
		RunWith(h.db).ExecContext(r.Context())
	if err != nil {
		h.log.Error("create admin", slog.Any("error", err))
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "email": req.Email, "permissions": req.Permissions})
}

func (h *handler) deleteAdmin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a := currentAdmin(r); a != nil && a.id == id {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot delete own account"})
		return
	}

	_, err := sq.Update("admins").
		Set("deleted_at", time.Now()).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		RunWith(h.db).ExecContext(r.Context())
	if err != nil {
		h.log.Error("delete admin", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *handler) updateAdminPermissions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a := currentAdmin(r); a != nil && a.id == id {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot change own permissions"})
		return
	}

	var req struct {
		Permissions uint8 `json:"permissions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	_, err := sq.Update("admins").
		Set("permissions", req.Permissions).
		Where(sq.Eq{"id": id}).
		RunWith(h.db).ExecContext(r.Context())
	if err != nil {
		h.log.Error("update admin permissions", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]uint8{"permissions": req.Permissions})
}
