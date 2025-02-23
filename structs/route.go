package structs

import (
	"time"

	"github.com/google/uuid"
)

type Route struct {
	User

	RouteUUID     uuid.UUID `json:"routeUUID"`
	Start         string    `json:"start"`
	End           string    `json:"end"`
	Waypoints     []string  `json:"waypoints"`
	DepartureTime time.Time `json:"departureTime"`
	SeatsLeft     int       `json:"seatsLeft"`
}
