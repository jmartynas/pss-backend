package handler

import (
	"net/http"

	"github.com/jmartynas/pss-backend/internal/domain"
)

func validLat(v float64) bool { return v >= -90 && v <= 90 }
func validLng(v float64) bool { return v >= -180 && v <= 180 }
func validCoords(lat, lng float64) bool { return validLat(lat) && validLng(lng) }

func validateStops(w http.ResponseWriter, stops []domain.ApplicationStopInput) bool {
	for _, s := range stops {
		if !validCoords(s.Lat, s.Lng) {
			http.Error(w, "invalid stop coordinates", http.StatusBadRequest)
			return false
		}
	}
	return true
}
