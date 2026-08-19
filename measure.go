package geo

import (
	"encoding/json"
	"math"
)

// The measurement methods below (SizeM2, MainExtentM, Center, PointAlong)
// work on a simplified read of the geometry: the outer ring of a Polygon,
// and for a MultiPolygon the outer ring of its largest member. That is good
// enough for what they are used for - ranking, centering, spreading points -
// and deliberately kept as-is. Containment (contains.go) decodes the full
// geometry instead: every ring of every member, holes included.

// SizeM2 returns the approximate footprint of the area in square meters:
// Polygon/MultiPolygon -> planar (equirectangular-projected) shoelace area;
// LineString -> arc length x WidthMeters. Returns 0 for malformed geometry -
// good enough for ranking, which is all this is used for.
func (a *Area) SizeM2() float64 {
	gtype, line, ring := a.decodeShape()
	switch gtype {
	case GeomLineString:
		width := MinWidthMeters
		if a.WidthMeters != nil {
			width = *a.WidthMeters
		}
		return polylineLengthM(line) * width
	case GeomPolygon, GeomMultiPolygon:
		return ringAreaM2(ring)
	default:
		return 0
	}
}

// MainExtentM returns the length in meters of the geometry's main extent:
// the arc length for a LineString, the longest vertex-to-vertex axis for a
// polygon. Returns 0 for malformed geometry.
func (a *Area) MainExtentM() float64 {
	gtype, line, ring := a.decodeShape()
	switch gtype {
	case GeomLineString:
		return polylineLengthM(line)
	case GeomPolygon, GeomMultiPolygon:
		if from, to, ok := longestAxis(ring); ok {
			return DistanceM(from, to)
		}
	}
	return 0
}

// Center returns a representative point of the geometry: the shoelace
// centroid for polygons, the arc-length midpoint for linestrings.
// ok is false for malformed geometry.
func (a *Area) Center() (Point, bool) {
	gtype, line, ring := a.decodeShape()
	switch gtype {
	case GeomLineString:
		return pointAlongPolyline(line, 0.5)
	case GeomPolygon, GeomMultiPolygon:
		return ringCentroid(ring)
	default:
		return Point{}, false
	}
}

// PointAlong returns the point at fraction t in [0, 1] of the geometry's main
// extent: along the line for linestrings, along the longest vertex-to-vertex
// axis for polygons. Useful for spreading multiple targets over one area.
// ok is false for malformed geometry.
func (a *Area) PointAlong(t float64) (Point, bool) {
	t = math.Max(0, math.Min(1, t))
	gtype, line, ring := a.decodeShape()
	switch gtype {
	case GeomLineString:
		return pointAlongPolyline(line, t)
	case GeomPolygon, GeomMultiPolygon:
		from, to, ok := longestAxis(ring)
		if !ok {
			return Point{}, false
		}
		return Point{
			Lat: from.Lat + (to.Lat-from.Lat)*t,
			Lng: from.Lng + (to.Lng-from.Lng)*t,
		}, true
	default:
		return Point{}, false
	}
}

// decodeShape extracts the working geometry: the point list for a LineString,
// or the outer ring for a Polygon (for a MultiPolygon: the outer ring of its
// largest member). GeoJSON coordinates are [lng, lat].
func (a *Area) decodeShape() (string, []Point, []Point) {
	var hdr geomHeader
	if err := json.Unmarshal(a.Geometry, &hdr); err != nil {
		return "", nil, nil
	}

	switch hdr.Type {
	case GeomLineString:
		var coords [][2]float64
		if err := json.Unmarshal(hdr.Coordinates, &coords); err != nil {
			return "", nil, nil
		}
		return hdr.Type, toPoints(coords), nil
	case GeomPolygon:
		var rings [][][2]float64
		if err := json.Unmarshal(hdr.Coordinates, &rings); err != nil || len(rings) == 0 {
			return "", nil, nil
		}
		return hdr.Type, nil, toPoints(rings[0])
	case GeomMultiPolygon:
		var polys [][][][2]float64
		if err := json.Unmarshal(hdr.Coordinates, &polys); err != nil {
			return "", nil, nil
		}
		var best []Point
		bestArea := -1.0
		for _, rings := range polys {
			if len(rings) == 0 {
				continue
			}
			outer := toPoints(rings[0])
			if area := ringAreaM2(outer); area > bestArea {
				best, bestArea = outer, area
			}
		}
		if best == nil {
			return "", nil, nil
		}
		return hdr.Type, nil, best
	default:
		return "", nil, nil
	}
}

func toPoints(coords [][2]float64) []Point {
	pts := make([]Point, len(coords))
	for i, c := range coords {
		pts[i] = Point{Lat: c[1], Lng: c[0]}
	}
	return pts
}

// ringAreaM2 computes the shoelace area of a closed ring, in m².
func ringAreaM2(ring []Point) float64 {
	if len(ring) < 4 {
		return 0
	}
	origin := ring[0]
	sum := 0.0
	for i := 1; i < len(ring); i++ {
		x1, y1 := project(ring[i-1], origin)
		x2, y2 := project(ring[i], origin)
		sum += x1*y2 - x2*y1
	}
	return math.Abs(sum) / 2
}

// ringCentroid computes the shoelace centroid of a closed ring, falling back
// to the vertex average for degenerate (zero-area) rings.
func ringCentroid(ring []Point) (Point, bool) {
	if len(ring) == 0 {
		return Point{}, false
	}

	origin := ring[0]
	var a, cx, cy float64
	for i := 1; i < len(ring); i++ {
		x1, y1 := project(ring[i-1], origin)
		x2, y2 := project(ring[i], origin)
		cross := x1*y2 - x2*y1
		a += cross
		cx += (x1 + x2) * cross
		cy += (y1 + y2) * cross
	}

	if math.Abs(a) < 1e-9 {
		// Degenerate ring: average the vertices (skip the closing duplicate).
		var lat, lng float64
		n := len(ring) - 1
		if n < 1 {
			n = len(ring)
		}
		for _, p := range ring[:n] {
			lat += p.Lat
			lng += p.Lng
		}
		return Point{Lat: lat / float64(n), Lng: lng / float64(n)}, true
	}

	cx /= 3 * a
	cy /= 3 * a
	return Point{
		Lat: origin.Lat + cy/metersPerDegLat,
		Lng: origin.Lng + cx/(metersPerDegLat*math.Cos(origin.Lat*math.Pi/180)),
	}, true
}

// longestAxis returns the most distant pair of ring vertices - the main
// extent of the polygon.
func longestAxis(ring []Point) (Point, Point, bool) {
	var from, to Point
	if len(ring) < 2 {
		return Point{}, Point{}, false
	}
	best := -1.0
	for i := range ring {
		for j := i + 1; j < len(ring); j++ {
			if d := DistanceM(ring[i], ring[j]); d > best {
				best, from, to = d, ring[i], ring[j]
			}
		}
	}
	return from, to, best > 0
}
