package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/jmartynas/pss-backend/internal/middleware"
	"github.com/jmartynas/pss-backend/internal/route"
)

type RoutesHandler struct {
	DB  *sql.DB
	Log *slog.Logger
}

func (h *RoutesHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	if idStr == "" {
		http.Error(w, "missing route id", http.StatusBadRequest)
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid route id", http.StatusBadRequest)
		return
	}

	detail, err := route.GetRoute(r.Context(), h.DB, id)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.Log.Error("get route", slog.String("id", idStr), slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(detail)
}

func (h *RoutesHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var in route.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	id, err := route.Create(r.Context(), h.DB, u.ID, in)
	if err != nil {
		h.Log.Error("create route", slog.Any("error", err))
		http.Error(w, "failed to create route", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id.String()})
}

func (h *RoutesHandler) SearchRoutes(w http.ResponseWriter, r *http.Request) {
	var in route.SearchInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	routes, err := route.Search(r.Context(), h.DB, in)
	if err != nil {
		h.Log.Error("search routes", slog.Any("error", err))
		http.Error(w, "failed to search routes", http.StatusInternalServerError)
		return
	}

	if routes == nil {
		routes = []route.Route{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(routes)
}

func (h *RoutesHandler) GetMyRoutes(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	filterStr := r.URL.Query().Get("filter")
	var filter route.RouteFilter
	switch filterStr {
	case "active":
		filter = route.RouteFilterActive
	case "past":
		filter = route.RouteFilterPast
	default:
		filter = ""
	}

	routes, err := route.GetRoutesByCreator(r.Context(), h.DB, u.ID, filter)
	if err != nil {
		h.Log.Error("get my routes", slog.Any("error", err))
		http.Error(w, "failed to get routes", http.StatusInternalServerError)
		return
	}

	if routes == nil {
		routes = []route.Route{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(routes)
}

func (h *RoutesHandler) GetMyParticipatedRoutes(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	filterStr := r.URL.Query().Get("filter")
	var filter route.RouteFilter
	switch filterStr {
	case "active":
		filter = route.RouteFilterActive
	case "past":
		filter = route.RouteFilterPast
	default:
		filter = ""
	}

	routes, err := route.GetRoutesByParticipant(r.Context(), h.DB, u.ID, filter)
	if err != nil {
		h.Log.Error("get my participated routes", slog.Any("error", err))
		http.Error(w, "failed to get routes", http.StatusInternalServerError)
		return
	}

	if routes == nil {
		routes = []route.Route{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(routes)
}
