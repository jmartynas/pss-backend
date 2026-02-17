package route

import (
	"math"
)

func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

const maxInterleavings = 500

func CalculateDeviation(route *Route, search SearchInput) float64 {
	baseDev := Haversine(search.StartLat, search.StartLng, route.StartLat, route.StartLng) +
		Haversine(search.EndLat, search.EndLng, route.EndLat, route.EndLng)

	groups := groupStopsByParticipant(route.Stops)
	orderings := allInterleavings(groups, maxInterleavings)

	if len(orderings) == 0 {
		return baseDev + deviationForStops(route.StartLat, route.StartLng, route.EndLat, route.EndLng, route.Stops, search)
	}

	minStopDev := math.MaxFloat64
	for _, ordered := range orderings {
		d := deviationForStops(route.StartLat, route.StartLng, route.EndLat, route.EndLng, ordered, search)
		if d < minStopDev {
			minStopDev = d
		}
	}
	return baseDev + minStopDev
}

func groupStopsByParticipant(stops []Stop) [][]Stop {
	type key struct {
		appID string
	}
	ordered := make([][]Stop, 0)
	seen := make(map[key]int)

	for _, s := range stops {
		k := key{appID: "creator"}
		if s.ApplicationID != nil {
			k.appID = s.ApplicationID.String()
		}
		idx, ok := seen[k]
		if !ok {
			idx = len(ordered)
			seen[k] = idx
			ordered = append(ordered, nil)
		}
		ordered[idx] = append(ordered[idx], s)
	}
	return ordered
}

func allInterleavings(groups [][]Stop, max int) [][]Stop {
	if len(groups) == 0 {
		return nil
	}
	if len(groups) == 1 {
		return [][]Stop{groups[0]}
	}

	var result [][]Stop
	indices := make([]int, len(groups))
	current := make([]Stop, 0, totalLen(groups))

	var gen func()
	gen = func() {
		if len(result) >= max {
			return
		}
		done := true
		for g := range groups {
			if indices[g] < len(groups[g]) {
				done = false
				break
			}
		}
		if done {
			result = append(result, append([]Stop{}, current...))
			return
		}
		for g := range groups {
			if indices[g] < len(groups[g]) {
				current = append(current, groups[g][indices[g]])
				indices[g]++
				gen()
				indices[g]--
				current = current[:len(current)-1]
			}
		}
	}
	gen()
	return result
}

func totalLen(groups [][]Stop) int {
	n := 0
	for _, g := range groups {
		n += len(g)
	}
	return n
}

func deviationForStops(startLat, startLng, endLat, endLng float64, stops []Stop, search SearchInput) float64 {
	segments := buildSegmentsFromStops(startLat, startLng, endLat, endLng, stops)
	dev := 0.0
	for _, us := range search.Stops {
		dev += minDistanceToSegment(us.Lat, us.Lng, segments)
	}
	return dev
}

func buildSegmentsFromStops(startLat, startLng, endLat, endLng float64, stops []Stop) [][2]float64 {
	if len(stops) == 0 {
		return [][2]float64{{(startLat + endLat) / 2, (startLng + endLng) / 2}}
	}
	segs := make([][2]float64, 0, len(stops)+1)
	prevLat, prevLng := startLat, startLng
	for _, s := range stops {
		segs = append(segs, [2]float64{(prevLat + s.Lat) / 2, (prevLng + s.Lng) / 2})
		prevLat, prevLng = s.Lat, s.Lng
	}
	segs = append(segs, [2]float64{(prevLat + endLat) / 2, (prevLng + endLng) / 2})
	return segs
}

func minDistanceToSegment(lat, lng float64, segments [][2]float64) float64 {
	min := math.MaxFloat64
	for _, seg := range segments {
		d := Haversine(lat, lng, seg[0], seg[1])
		if d < min {
			min = d
		}
	}
	return min
}
