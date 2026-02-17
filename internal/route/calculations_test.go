package route

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lng1     float64
		lat2     float64
		lng2     float64
		want     float64
		tolerance float64
	}{
		{
			name:     "same point",
			lat1:     0.0,
			lng1:     0.0,
			lat2:     0.0,
			lng2:     0.0,
			want:     0.0,
			tolerance: 0.001,
		},
		{
			name:     "New York to Los Angeles",
			lat1:     40.7128,
			lng1:     -74.0060,
			lat2:     34.0522,
			lng2:     -118.2437,
			want:     3944.0,
			tolerance: 50.0,
		},
		{
			name:     "London to Paris",
			lat1:     51.5074,
			lng1:     -0.1278,
			lat2:     48.8566,
			lng2:     2.3522,
			want:     344.0,
			tolerance: 10.0,
		},
		{
			name:     "short distance",
			lat1:     0.0,
			lng1:     0.0,
			lat2:     0.1,
			lng2:     0.1,
			want:     15.7,
			tolerance: 1.0,
		},
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

func TestGroupStopsByParticipant(t *testing.T) {
	appID1 := uuid.New()
	appID2 := uuid.New()

	tests := []struct {
		name  string
		stops []Stop
		want  int
	}{
		{
			name:  "empty stops",
			stops: []Stop{},
			want:  0,
		},
		{
			name: "single creator stop",
			stops: []Stop{
				{ID: uuid.New(), ApplicationID: nil, Position: 0},
			},
			want: 1,
		},
		{
			name: "multiple creator stops",
			stops: []Stop{
				{ID: uuid.New(), ApplicationID: nil, Position: 0},
				{ID: uuid.New(), ApplicationID: nil, Position: 1},
			},
			want: 1,
		},
		{
			name: "creator and application stops",
			stops: []Stop{
				{ID: uuid.New(), ApplicationID: nil, Position: 0},
				{ID: uuid.New(), ApplicationID: &appID1, Position: 1},
				{ID: uuid.New(), ApplicationID: &appID1, Position: 2},
				{ID: uuid.New(), ApplicationID: &appID2, Position: 3},
			},
			want: 3,
		},
		{
			name: "multiple applications",
			stops: []Stop{
				{ID: uuid.New(), ApplicationID: &appID1, Position: 0},
				{ID: uuid.New(), ApplicationID: &appID2, Position: 1},
				{ID: uuid.New(), ApplicationID: &appID1, Position: 2},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupStopsByParticipant(tt.stops)
			if len(got) != tt.want {
				t.Errorf("groupStopsByParticipant() returned %d groups, want %d", len(got), tt.want)
			}

			// Verify all stops are accounted for
			totalStops := 0
			for _, group := range got {
				totalStops += len(group)
			}
			if totalStops != len(tt.stops) {
				t.Errorf("groupStopsByParticipant() total stops = %d, want %d", totalStops, len(tt.stops))
			}
		})
	}
}

func TestAllInterleavings(t *testing.T) {
	appID1 := uuid.New()

	tests := []struct {
		name  string
		groups [][]Stop
		max    int
		want   int
	}{
		{
			name:   "empty groups",
			groups: [][]Stop{},
			max:    10,
			want:   0,
		},
		{
			name: "single group",
			groups: [][]Stop{
				{
					{ID: uuid.New(), Position: 0},
					{ID: uuid.New(), Position: 1},
				},
			},
			max:  10,
			want: 1,
		},
		{
			name: "two groups",
			groups: [][]Stop{
				{{ID: uuid.New(), ApplicationID: nil, Position: 0}},
				{{ID: uuid.New(), ApplicationID: &appID1, Position: 1}},
			},
			max:  10,
			want: 2,
		},
		{
			name: "respects max limit",
			groups: [][]Stop{
				{{ID: uuid.New(), Position: 0}},
				{{ID: uuid.New(), Position: 1}},
				{{ID: uuid.New(), Position: 2}},
			},
			max:  2,
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allInterleavings(tt.groups, tt.max)
			if len(got) < tt.want {
				t.Errorf("allInterleavings() returned %d interleavings, want at least %d", len(got), tt.want)
			}
			if len(got) > tt.max {
				t.Errorf("allInterleavings() returned %d interleavings, exceeded max %d", len(got), tt.max)
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
		stops    []Stop
		want     int
	}{
		{
			name:     "no stops",
			startLat: 0.0,
			startLng: 0.0,
			endLat:   1.0,
			endLng:   1.0,
			stops:    []Stop{},
			want:     1,
		},
		{
			name:     "one stop",
			startLat: 0.0,
			startLng: 0.0,
			endLat:   2.0,
			endLng:   2.0,
			stops: []Stop{
				{Lat: 1.0, Lng: 1.0},
			},
			want: 2,
		},
		{
			name:     "multiple stops",
			startLat: 0.0,
			startLng: 0.0,
			endLat:   3.0,
			endLng:   3.0,
			stops: []Stop{
				{Lat: 1.0, Lng: 1.0},
				{Lat: 2.0, Lng: 2.0},
			},
			want: 3,
		},
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
	segments := [][2]float64{
		{0.0, 0.0},
		{1.0, 1.0},
		{2.0, 2.0},
	}

	tests := []struct {
		name     string
		lat      float64
		lng      float64
		segments [][2]float64
		want     float64
		tolerance float64
	}{
		{
			name:      "point on segment",
			lat:       1.0,
			lng:       1.0,
			segments:  segments,
			want:      0.0,
			tolerance: 0.1,
		},
		{
			name:      "point near segment",
			lat:       1.1,
			lng:       1.1,
			segments:  segments,
			want:      15.7,
			tolerance: 5.0,
		},
		{
			name:      "empty segments",
			lat:       1.0,
			lng:       1.0,
			segments:  [][2]float64{},
			want:      math.MaxFloat64,
			tolerance: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minDistanceToSegment(tt.lat, tt.lng, tt.segments)
			if tt.segments == nil || len(tt.segments) == 0 {
				if got != tt.want {
					t.Errorf("minDistanceToSegment() = %v, want %v", got, tt.want)
				}
			} else if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("minDistanceToSegment() = %v, want %v (tolerance: %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestDeviationForStops(t *testing.T) {
	tests := []struct {
		name     string
		startLat float64
		startLng float64
		endLat   float64
		endLng   float64
		stops    []Stop
		search   SearchInput
		want     float64
		tolerance float64
	}{
		{
			name:     "no search stops",
			startLat: 0.0,
			startLng: 0.0,
			endLat:   1.0,
			endLng:   1.0,
			stops:    []Stop{},
			search: SearchInput{
				Stops: []SearchStopInput{},
			},
			want:     0.0,
			tolerance: 0.001,
		},
		{
			name:     "search stop on route",
			startLat: 0.0,
			startLng: 0.0,
			endLat:   2.0,
			endLng:   2.0,
			stops: []Stop{
				{Lat: 1.0, Lng: 1.0},
			},
			search: SearchInput{
				Stops: []SearchStopInput{
					{Lat: 1.0, Lng: 1.0},
				},
			},
			want:     78.6,
			tolerance: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deviationForStops(tt.startLat, tt.startLng, tt.endLat, tt.endLng, tt.stops, tt.search)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("deviationForStops() = %v, want %v (tolerance: %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestCalculateDeviation(t *testing.T) {
	appID1 := uuid.New()

	tests := []struct {
		name     string
		route    *Route
		search   SearchInput
		want     float64
		tolerance float64
	}{
		{
			name: "exact match",
			route: &Route{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   1.0,
				EndLng:   1.0,
				Stops:    []Stop{},
			},
			search: SearchInput{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   1.0,
				EndLng:   1.0,
				Stops:    []SearchStopInput{},
			},
			want:     0.0,
			tolerance: 0.001,
		},
		{
			name: "different start and end",
			route: &Route{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   1.0,
				EndLng:   1.0,
				Stops:    []Stop{},
			},
			search: SearchInput{
				StartLat: 0.1,
				StartLng: 0.1,
				EndLat:   1.1,
				EndLng:   1.1,
				Stops:    []SearchStopInput{},
			},
			want:     31.4,
			tolerance: 5.0,
		},
		{
			name: "route with stops, search with stops",
			route: &Route{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   2.0,
				EndLng:   2.0,
				Stops: []Stop{
					{Lat: 1.0, Lng: 1.0, ApplicationID: nil},
				},
			},
			search: SearchInput{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   2.0,
				EndLng:   2.0,
				Stops: []SearchStopInput{
					{Lat: 1.0, Lng: 1.0},
				},
			},
			want:     78.6,
			tolerance: 5.0,
		},
		{
			name: "route with participant stops",
			route: &Route{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   3.0,
				EndLng:   3.0,
				Stops: []Stop{
					{Lat: 1.0, Lng: 1.0, ApplicationID: &appID1, Position: 0},
					{Lat: 2.0, Lng: 2.0, ApplicationID: &appID1, Position: 1},
				},
			},
			search: SearchInput{
				StartLat: 0.0,
				StartLng: 0.0,
				EndLat:   3.0,
				EndLng:   3.0,
				Stops: []SearchStopInput{
					{Lat: 1.5, Lng: 1.5},
				},
			},
			want:     0.0,
			tolerance: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDeviation(tt.route, tt.search)
			if got < 0 {
				t.Errorf("CalculateDeviation() returned negative value: %v", got)
			}
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("CalculateDeviation() = %v, want %v (tolerance: %v)", got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestTotalLen(t *testing.T) {
	tests := []struct {
		name   string
		groups [][]Stop
		want   int
	}{
		{
			name:   "empty groups",
			groups: [][]Stop{},
			want:   0,
		},
		{
			name: "single group",
			groups: [][]Stop{
				{{ID: uuid.New()}, {ID: uuid.New()}},
			},
			want: 2,
		},
		{
			name: "multiple groups",
			groups: [][]Stop{
				{{ID: uuid.New()}},
				{{ID: uuid.New()}, {ID: uuid.New()}},
				{{ID: uuid.New()}},
			},
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := totalLen(tt.groups)
			if got != tt.want {
				t.Errorf("totalLen() = %v, want %v", got, tt.want)
			}
		})
	}
}
