package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/jmartynas/pss-backend/internal/middleware"
	"github.com/jmartynas/pss-backend/internal/service"
)

// RouteHandler handles all route-related HTTP endpoints.
type RouteHandler struct {
	svc *service.RouteService
	log *slog.Logger
}

// NewRouteHandler creates a RouteHandler backed by the given service.
func NewRouteHandler(svc *service.RouteService, log *slog.Logger) *RouteHandler {
	return &RouteHandler{svc: svc, log: log}
}

func (h *RouteHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	route, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.log.Error("get route", slog.String("id", id.String()), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (h *RouteHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var in domain.CreateRouteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if in.LeavingAt != nil && !in.LeavingAt.After(time.Now()) {
		http.Error(w, "departure time must be in the future", http.StatusBadRequest)
		return
	}
	id, err := h.svc.Create(r.Context(), u.ID, in)
	if err != nil {
		h.log.Error("create route", slog.Any("error", err))
		http.Error(w, "failed to create route", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *RouteHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	var in domain.UpdateRouteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.svc.Update(r.Context(), id, u.ID, in); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started and cannot be modified"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			h.log.Error("update route", slog.String("id", id.String()), slog.Any("error", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	route, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		h.log.Error("get route after update", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (h *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id, u.ID); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started and cannot be deleted"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			h.log.Error("delete route", slog.String("id", id.String()), slog.Any("error", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RouteHandler) SearchRoutes(w http.ResponseWriter, r *http.Request) {
	var in domain.SearchRouteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	routes, err := h.svc.Search(r.Context(), in)
	if err != nil {
		h.log.Error("search routes", slog.Any("error", err))
		http.Error(w, "failed to search routes", http.StatusInternalServerError)
		return
	}
	if routes == nil {
		routes = []domain.Route{}
	}
	writeJSON(w, http.StatusOK, routes)
}

func (h *RouteHandler) GetMyRoutes(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	filter := parseRouteFilter(r)
	routes, err := h.svc.ListByCreator(r.Context(), u.ID, filter)
	if err != nil {
		h.log.Error("get my routes", slog.Any("error", err))
		http.Error(w, "failed to get routes", http.StatusInternalServerError)
		return
	}
	if routes == nil {
		routes = []domain.Route{}
	}
	writeJSON(w, http.StatusOK, routes)
}

func (h *RouteHandler) GetMyParticipatedRoutes(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	filter := parseRouteFilter(r)
	routes, err := h.svc.ListByParticipant(r.Context(), u.ID, filter)
	if err != nil {
		h.log.Error("get my participated routes", slog.Any("error", err))
		http.Error(w, "failed to get routes", http.StatusInternalServerError)
		return
	}
	if routes == nil {
		routes = []domain.Route{}
	}
	writeJSON(w, http.StatusOK, routes)
}

func (h *RouteHandler) GetMyReviews(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	reviews, err := h.svc.GetMyReviewsForRoute(r.Context(), routeID, u.ID)
	if err != nil {
		h.log.Error("get my reviews for route", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Return only the target_user_ids so the frontend knows which users are already reviewed.
	type item struct {
		TargetUserID string `json:"target_user_id"`
	}
	out := make([]item, len(reviews))
	for i, rv := range reviews {
		out[i] = item{TargetUserID: rv.TargetID.String()}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RouteHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		TargetUserID string `json:"target_user_id"`
		Rating       int    `json:"rating"`
		Comment      string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	targetID, err := uuid.Parse(in.TargetUserID)
	if err != nil {
		http.Error(w, "invalid target_user_id", http.StatusBadRequest)
		return
	}
	id, err := h.svc.CreateReview(r.Context(), routeID, u.ID, in.Rating, in.Comment, targetID)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "route not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteNotFinished):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has not started yet"})
		case errors.Is(err, errs.ErrNotParticipant):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a participant of this route"})
		case errors.Is(err, errs.ErrAlreadyReviewed):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "you have already reviewed this user for this route"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			h.log.Error("create review", slog.Any("error", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func parseRouteFilter(r *http.Request) domain.RouteFilter {
	switch r.URL.Query().Get("filter") {
	case "active":
		return domain.RouteFilterActive
	case "past":
		return domain.RouteFilterPast
	default:
		return ""
	}
}

func parseUUIDPath(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	raw := strings.TrimSpace(r.PathValue(key))
	if raw == "" {
		http.Error(w, "missing "+key, http.StatusBadRequest)
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid "+key, http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}
