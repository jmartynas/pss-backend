package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jmartynas/pss-backend/internal/domain"
	"github.com/jmartynas/pss-backend/internal/errs"
	"github.com/jmartynas/pss-backend/internal/middleware"
)

type VehicleHandler struct {
	repo domain.VehicleRepository
	log  *slog.Logger
}

func NewVehicleHandler(repo domain.VehicleRepository, log *slog.Logger) *VehicleHandler {
	return &VehicleHandler{repo: repo, log: log}
}

func (h *VehicleHandler) ListMy(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	vehicles, err := h.repo.ListByUser(r.Context(), u.ID)
	if err != nil {
		h.log.Error("list vehicles", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, vehicles)
}

func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var in domain.CreateVehicleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if in.Model == "" || in.PlateNumber == "" {
		http.Error(w, "model and plate_number are required", http.StatusBadRequest)
		return
	}
	if len(in.Model) > 100 || len(in.PlateNumber) > 20 {
		http.Error(w, "model or plate_number too long", http.StatusBadRequest)
		return
	}
	if in.Seats == 0 {
		in.Seats = 4
	}
	if in.Seats > 16 {
		http.Error(w, "seats must be between 1 and 16", http.StatusBadRequest)
		return
	}

	id, err := h.repo.Create(r.Context(), u.ID, in)
	if err != nil {
		h.log.Error("create vehicle", slog.Any("error", err))
		http.Error(w, "failed to create vehicle", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *VehicleHandler) Update(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}

	var in domain.UpdateVehicleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if in.Model == "" || in.PlateNumber == "" {
		http.Error(w, "model and plate_number are required", http.StatusBadRequest)
		return
	}
	if len(in.Model) > 100 || len(in.PlateNumber) > 20 {
		http.Error(w, "model or plate_number too long", http.StatusBadRequest)
		return
	}
	if in.Seats == 0 {
		in.Seats = 4
	}
	if in.Seats > 16 {
		http.Error(w, "seats must be between 1 and 16", http.StatusBadRequest)
		return
	}

	err := h.repo.Update(r.Context(), id, u.ID, in)
	if errors.Is(err, errs.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if errors.Is(err, errs.ErrForbidden) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err != nil {
		h.log.Error("update vehicle", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r.Context())
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, ok := parseUUIDPath(w, r, "id")
	if !ok {
		return
	}

	err := h.repo.Delete(r.Context(), id, u.ID)
	if errors.Is(err, errs.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if errors.Is(err, errs.ErrForbidden) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err != nil {
		h.log.Error("delete vehicle", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
