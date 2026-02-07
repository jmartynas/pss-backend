package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/auth"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/jmartynas/pss-backend/internal/middleware"
	"github.com/jmartynas/pss-backend/internal/service"
)

// UserHandler handles user profile endpoints.
type UserHandler struct {
	svc      *service.UserService
	sessions domain.SessionRepository
	secure   bool
	log      *slog.Logger
}

// NewUserHandler creates a UserHandler backed by the given service.
func NewUserHandler(svc *service.UserService, sessions domain.SessionRepository, secure bool, log *slog.Logger) *UserHandler {
	return &UserHandler{svc: svc, sessions: sessions, secure: secure, log: log}
}

type meResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type reviewResponse struct {
	ID         string    `json:"id"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Rating     int       `json:"rating"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

type publicProfileResponse struct {
	ID        string           `json:"id"`
	Email     string           `json:"email"`
	Name      string           `json:"name"`
	Status    string           `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	Reviews   []reviewResponse `json:"reviews"`
}

// GetMe handles GET /users/me
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	fresh, err := h.svc.GetProfile(r.Context(), u.ID)
	if err != nil {
		h.log.Error("get me", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		ID:        fresh.ID.String(),
		Email:     fresh.Email,
		Name:      fresh.Name,
		Provider:  fresh.Provider,
		Status:    fresh.Status,
		CreatedAt: fresh.CreatedAt,
		UpdatedAt: fresh.UpdatedAt,
	})
}

// UpdateMe handles PATCH /users/me
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var body struct {
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdateProfile(r.Context(), u.ID, body.Name); err != nil {
		h.log.Error("update me", slog.Any("error", err))
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}
	fresh, err := h.svc.GetProfile(r.Context(), u.ID)
	if err != nil {
		h.log.Error("get me after update", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		ID:        fresh.ID.String(),
		Email:     fresh.Email,
		Name:      fresh.Name,
		Provider:  fresh.Provider,
		Status:    fresh.Status,
		CreatedAt: fresh.CreatedAt,
		UpdatedAt: fresh.UpdatedAt,
	})
}

// DisableMe handles POST /users/me/disable
func (h *UserHandler) DisableMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if err := h.svc.DisableAccount(r.Context(), u.ID); err != nil {
		h.log.Error("disable account", slog.Any("error", err))
		http.Error(w, "failed to disable account", http.StatusInternalServerError)
		return
	}
	// Clear session so the client is logged out immediately.
	if token, err := auth.GetRefreshToken(r); err == nil && token != "" {
		_ = h.sessions.DeleteByToken(r.Context(), token)
	}
	auth.ClearSession(w, h.secure)
	auth.ClearRefreshToken(w, h.secure)
	w.WriteHeader(http.StatusNoContent)
}

// GetUser handles GET /users/{id} — public profile with reviews.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	user, reviews, err := h.svc.GetPublicProfile(r.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		h.log.Error("get user", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := publicProfileResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		Name:      user.Name,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		Reviews:   make([]reviewResponse, 0, len(reviews)),
	}
	for _, rv := range reviews {
		resp.Reviews = append(resp.Reviews, reviewResponse{
			ID:         rv.ID.String(),
			AuthorID:   rv.AuthorID.String(),
			AuthorName: rv.AuthorName,
			Rating:     rv.Rating,
			Comment:    rv.Comment,
			CreatedAt:  rv.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
