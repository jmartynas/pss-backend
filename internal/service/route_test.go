package service

import (
	"math"
	"testing"

	"github.com/jmartynas/pss-backend/internal/domain"
)

func strPtr(s string) *string { return &s }

func TestHaversine(t *testing.T) {
	tests := []struct {
		name      string
		lat1      float64
		lng1      float64
		lat2      float64
		lng2      float64
		want      float64
		tolerance float64
	}{
		{"same point", 0.0, 0.0, 0.0, 0.0, 0.0, 0.001},
		{"New York to Los Angeles", 40.7128, -74.0060, 34.0522, -118.2437, 3944.0, 50.0},
		{"London to Paris", 51.5074, -0.1278, 48.8566, 2.3522, 344.0, 10.0},
		{"short distance", 0.0, 0.0, 0.1, 0.1, 15.7, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Haversine(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("Haversine() = %v, want %v (tolerance: %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestBuildSegmentsFromStops(t *testing.T) {
	tests := []struct {
		name     string
		startLat float64
		startLng float64
		endLat   float64
		endLng   float64
		stops    []domain.Stop
		want     int
	}{
		{"no stops", 0, 0, 1, 1, []domain.Stop{}, 1},
		{"one stop", 0, 0, 2, 2, []domain.Stop{{Lat: 1, Lng: 1}}, 2},
		{"multiple stops", 0, 0, 3, 3, []domain.Stop{{Lat: 1, Lng: 1}, {Lat: 2, Lng: 2}}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSegmentsFromStops(tt.startLat, tt.startLng, tt.endLat, tt.endLng, tt.stops)
			if len(got) != tt.want {
				t.Errorf("buildSegmentsFromStops() returned %d segments, want %d", len(got), tt.want)
			}
		})
	}
}

func TestMinDistanceToSegment(t *testing.T) {
	// Segment along the equator from (0,0) to (0,1).
	segments := []routeSegment{{0, 0, 0, 1}}
	tests := []struct {
		name      string
		lat, lng  float64
		segs      []routeSegment
		want      float64
		tolerance float64
	}{
		{"point on segment", 0, 0.5, segments, 0, 0.1},
		{"point off segment", 1, 0.5, segments, 111.2, 5.0},
		{"empty segments", 1, 1, []routeSegment{}, math.MaxFloat64, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minDistanceToSegment(tt.lat, tt.lng, tt.segs)
			if len(tt.segs) == 0 {
				if got != tt.want {
					t.Errorf("minDistanceToSegment() = %v, want %v", got, tt.want)
				}
			} else if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("minDistanceToSegment() = %v, want %v (tolerance %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestDeviationForStops(t *testing.T) {
	tests := []struct {
		name      string
		startLat  float64
		startLng  float64
		endLat    float64
		endLng    float64
		stops     []domain.Stop
		search    domain.SearchRouteInput
		want      float64
		tolerance float64
	}{
		{
			name:   "no search stops",
			endLat: 1, endLng: 1,
			stops:  []domain.Stop{},
			search: domain.SearchRouteInput{Stops: []domain.SearchStop{}},
			want: 0, tolerance: 0.001,
		},
		{
			name:   "search stop exactly on route stop",
			endLat: 2, endLng: 2,
			stops:  []domain.Stop{{Lat: 1, Lng: 1}},
			search: domain.SearchRouteInput{Stops: []domain.SearchStop{{Lat: 1, Lng: 1}}},
			want: 0, tolerance: 0.1,
		},
		{
			name:   "search stop off route",
			endLat: 0, endLng: 1,
			stops:  []domain.Stop{},
			search: domain.SearchRouteInput{Stops: []domain.SearchStop{{Lat: 1, Lng: 0.5}}},
			want: 111.2, tolerance: 5.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deviationForStops(tt.startLat, tt.startLng, tt.endLat, tt.endLng, tt.stops, tt.search)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("deviationForStops() = %v, want %v (tolerance %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestCalculateDeviation(t *testing.T) {
	pid := strPtr("participant-1")

	tests := []struct {
		name      string
		route     *domain.Route
		search    domain.SearchRouteInput
		want      float64
		tolerance float64
	}{
		{
			name:   "exact match",
			route:  &domain.Route{EndLat: 1, EndLng: 1, Stops: []domain.Stop{}},
			search: domain.SearchRouteInput{EndLat: 1, EndLng: 1, Stops: []domain.SearchStop{}},
			want: 0, tolerance: 0.001,
		},
		{
			name:   "different start and end",
			route:  &domain.Route{EndLat: 1, EndLng: 1, Stops: []domain.Stop{}},
			search: domain.SearchRouteInput{StartLat: 0.1, StartLng: 0.1, EndLat: 1.1, EndLng: 1.1, Stops: []domain.SearchStop{}},
			want: 31.4, tolerance: 5.0,
		},
		{
			name:   "route with stops, search stop on route stop",
			route:  &domain.Route{EndLat: 2, EndLng: 2, Stops: []domain.Stop{{Lat: 1, Lng: 1}}},
			search: domain.SearchRouteInput{EndLat: 2, EndLng: 2, Stops: []domain.SearchStop{{Lat: 1, Lng: 1}}},
			want: 0, tolerance: 0.1,
		},
		{
			name: "route with participant stops",
			route: &domain.Route{
				EndLat: 3, EndLng: 3,
				Stops: []domain.Stop{
					{Lat: 1, Lng: 1, ParticipantID: pid, Position: 0},
					{Lat: 2, Lng: 2, ParticipantID: pid, Position: 1},
				},
			},
			search: domain.SearchRouteInput{
				EndLat: 3, EndLng: 3,
				Stops:  []domain.SearchStop{{Lat: 1.5, Lng: 1.5}},
			},
			want: 0, tolerance: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateDeviation(tt.route, tt.search)
			if got < 0 {
				t.Errorf("calculateDeviation() returned negative: %v", got)
			}
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("calculateDeviation() = %v, want %v (tolerance %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}
