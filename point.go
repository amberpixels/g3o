// Package g3o is pure WGS84 / GeoJSON geometry for city-scale features:
// points, distances, areas (Polygon / MultiPolygon / LineString strips),
// containment, and GPS-track statistics.
//
// All planar math runs through a local equirectangular projection, which is
// plenty accurate at the scale of parks, lakes and streets. Nothing here is
// meant for continent-sized shapes or geometries crossing the antimeridian.
// The package is stdlib-only by design: both of its consumers value a
// dependency-free graph.
package g3o

import "math"

// Point is a WGS84 coordinate pair.
type Point struct {
	Lat float64
	Lng float64
}

const metersPerDegLat = 111_320.0

// project maps a point to planar meters relative to origin using a local
// equirectangular projection - plenty accurate at city scale.
func project(p, origin Point) (float64, float64) {
	x := (p.Lng - origin.Lng) * metersPerDegLat * math.Cos(origin.Lat*math.Pi/180)
	y := (p.Lat - origin.Lat) * metersPerDegLat
	return x, y
}

// DistanceM returns the planar distance between two points in meters.
func DistanceM(a, b Point) float64 {
	x, y := project(b, a)
	return math.Hypot(x, y)
}

func polylineLengthM(pts []Point) float64 {
	total := 0.0
	for i := 1; i < len(pts); i++ {
		total += DistanceM(pts[i-1], pts[i])
	}
	return total
}

func pointAlongPolyline(pts []Point, t float64) (Point, bool) {
	if len(pts) == 0 {
		return Point{}, false
	}
	if len(pts) == 1 {
		return pts[0], true
	}

	total := polylineLengthM(pts)
	if total == 0 {
		return pts[0], true
	}

	remaining := total * t
	for i := 1; i < len(pts); i++ {
		seg := DistanceM(pts[i-1], pts[i])
		if remaining <= seg && seg > 0 {
			f := remaining / seg
			return Point{
				Lat: pts[i-1].Lat + (pts[i].Lat-pts[i-1].Lat)*f,
				Lng: pts[i-1].Lng + (pts[i].Lng-pts[i-1].Lng)*f,
			}, true
		}
		remaining -= seg
	}
	return pts[len(pts)-1], true
}
