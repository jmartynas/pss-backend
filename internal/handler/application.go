package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/jmartynas/pss-backend/internal/middleware"
	"github.com/jmartynas/pss-backend/internal/service"
)

// ApplicationHandler handles all application-related HTTP endpoints.
type ApplicationHandler struct {
	svc *service.ApplicationService
	log *slog.Logger
}

// NewApplicationHandler creates an ApplicationHandler backed by the given service.
func NewApplicationHandler(svc *service.ApplicationService, log *slog.Logger) *ApplicationHandler {
	return &ApplicationHandler{svc: svc, log: log}
}

// Apply handles POST /routes/{id}/applications
func (h *ApplicationHandler) Apply(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	var in domain.ApplyInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	appID, err := h.svc.Apply(r.Context(), u.ID, routeID, in)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "route not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			http.Error(w, "route has already started", http.StatusConflict)
		case errors.Is(err, errs.ErrForbidden):
			http.Error(w, "route creator cannot apply to own route", http.StatusForbidden)
		case errors.Is(err, errs.ErrRouteFull):
			http.Error(w, "route is full", http.StatusConflict)
		case errors.Is(err, errs.ErrAlreadyApplied):
			http.Error(w, "already applied to this route", http.StatusConflict)
		default:
			h.log.Error("apply to route", slog.Any("error", err))
			http.Error(w, "failed to apply", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": appID.String()})
}

// ListByRoute handles GET /routes/{id}/applications
// Only the route creator can list applications for their route.
func (h *ApplicationHandler) ListByRoute(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	apps, err := h.svc.ListByRoute(r.Context(), routeID, u.ID)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "route not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			h.log.Error("list route applications", slog.Any("error", err))
			http.Error(w, "failed to list applications", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

// ReviewApplication handles PATCH /routes/{id}/applications/{appId}
func (h *ApplicationHandler) ReviewApplication(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	appID, ok := parseUUIDPath(w, r, "appId")
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Status != "approved" && body.Status != "rejected" {
		http.Error(w, `status must be "approved" or "rejected"`, http.StatusBadRequest)
		return
	}
	if err := h.svc.Review(r.Context(), appID, body.Status, u.ID); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "application not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, errs.ErrConflict):
			http.Error(w, "application is not in pending state", http.StatusConflict)
		default:
			h.log.Error("review application", slog.Any("error", err))
			http.Error(w, "failed to review application", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Cancel handles DELETE /routes/{id}/applications/{appId}
func (h *ApplicationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	appID, ok := parseUUIDPath(w, r, "appId")
	if !ok {
		return
	}
	if err := h.svc.Cancel(r.Context(), appID, u.ID); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "application not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, errs.ErrConflict):
			http.Error(w, "application cannot be cancelled once accepted or rejected", http.StatusConflict)
		default:
			h.log.Error("cancel application", slog.Any("error", err))
			http.Error(w, "failed to cancel application", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CancelStopChange handles DELETE /routes/{id}/applications/{appId}/stop-change
func (h *ApplicationHandler) CancelStopChange(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	appID, ok := parseUUIDPath(w, r, "appId")
	if !ok {
		return
	}
	if err := h.svc.CancelStopChange(r.Context(), appID, u.ID); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "application not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, errs.ErrConflict):
			http.Error(w, "no pending stop-change request", http.StatusConflict)
		default:
			h.log.Error("cancel stop change", slog.Any("error", err))
			http.Error(w, "failed to cancel stop-change request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetMyForRoute handles GET /routes/{id}/applications/my
func (h *ApplicationHandler) GetMyForRoute(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	routeID, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}
	app, err := h.svc.GetMyForRoute(r.Context(), u.ID, routeID)
	if err != nil {
		h.log.Error("get my application for route", slog.Any("error", err))
		http.Error(w, "failed to get application", http.StatusInternalServerError)
		return
	}
	if app == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// UpdateMyStops handles PATCH /routes/{id}/applications/{appId}/stops
func (h *ApplicationHandler) UpdateMyStops(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	appID, ok := parseUUIDPath(w, r, "appId")
	if !ok {
		return
	}
	var body struct {
		Stops   []domain.ApplicationStopInput `json:"stops"`
		Comment *string                       `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdateStops(r.Context(), appID, u.ID, body.Stops, body.Comment); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "application not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, errs.ErrConflict):
			http.Error(w, "application cannot be edited once accepted or rejected", http.StatusConflict)
		default:
			h.log.Error("update application stops", slog.Any("error", err))
			http.Error(w, "failed to update stops", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RequestStopChange handles POST /routes/{id}/applications/{appId}/stop-change
func (h *ApplicationHandler) RequestStopChange(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	appID, ok := parseUUIDPath(w, r, "appId")
	if !ok {
		return
	}
	var body struct {
		Stops   []domain.ApplicationStopInput `json:"stops"`
		Comment *string                       `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.svc.RequestStopChange(r.Context(), appID, u.ID, body.Stops, body.Comment); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "application not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, errs.ErrConflict):
			http.Error(w, "application is not approved", http.StatusConflict)
		default:
			h.log.Error("request stop change", slog.Any("error", err))
			http.Error(w, "failed to submit stop change request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReviewStopChange handles PATCH /routes/{id}/applications/{appId}/stop-change
func (h *ApplicationHandler) ReviewStopChange(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	appID, ok := parseUUIDPath(w, r, "appId")
	if !ok {
		return
	}
	var body struct {
		Approve bool `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.svc.ReviewStopChange(r.Context(), appID, u.ID, body.Approve); err != nil {
		switch {
		case errors.Is(err, errs.ErrNotFound):
			http.Error(w, "application not found", http.StatusNotFound)
		case errors.Is(err, errs.ErrRouteStarted):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "route has already started"})
		case errors.Is(err, errs.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, errs.ErrConflict):
			http.Error(w, "no pending stop change request", http.StatusConflict)
		default:
			h.log.Error("review stop change", slog.Any("error", err))
			http.Error(w, "failed to review stop change", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetMyApplications handles GET /applications/my
func (h *ApplicationHandler) GetMyApplications(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	apps, err := h.svc.ListByUser(r.Context(), u.ID)
	if err != nil {
		h.log.Error("get my applications", slog.Any("error", err))
		http.Error(w, "failed to get applications", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, apps)
}
