package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (h *handler) seedAdmin(email, password string) error {
	if email == "" || password == "" {
		return nil
	}

	var count int
	if err := sq.Select("COUNT(*)").From("admins").RunWith(h.db).QueryRow().Scan(&count); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}

	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if _, err = sq.Insert("admins").
		Columns("id", "email", "password", "status", "permissions").
		Values(uuid.New().String(), email, string(hash), 0, 7).
		RunWith(h.db).Exec(); err != nil {
		return fmt.Errorf("insert seed admin: %w", err)
	}

	h.log.Info("seed admin created", slog.String("email", email))
	return nil
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	var id, hash string
	var status, permissions uint8

	err := sq.Select("id", "password", "status", "permissions").
		From("admins").
		Where(sq.Eq{"email": req.Email, "deleted_at": nil}).
		Where(sq.Gt{"permissions": 0}).
		RunWith(h.db).QueryRowContext(r.Context()).Scan(&id, &hash, &status, &permissions)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if err != nil {
		h.log.Error("query admin", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	claims := adminClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		AdminID:     id,
		Permissions: permissions,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.jwtSecret)
	if err != nil {
		h.log.Error("sign token", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":       signed,
		"permissions": permissions,
		"admin_id":    id,
	})
}

func (h *handler) changePassword(w http.ResponseWriter, r *http.Request) {
	a := currentAdmin(r)
	if a == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Current == "" || req.New == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "current_password and new_password are required"})
		return
	}

	var hash string
	if err := sq.Select("password").From("admins").Where(sq.Eq{"id": a.id}).
		RunWith(h.db).QueryRowContext(r.Context()).Scan(&hash); err != nil {
		h.log.Error("fetch admin password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Current)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect current password"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error("hash new password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if _, err = sq.Update("admins").Set("password", string(newHash)).Where(sq.Eq{"id": a.id}).
		RunWith(h.db).ExecContext(r.Context()); err != nil {
		h.log.Error("update password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
