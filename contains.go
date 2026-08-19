package g3o

import (
	"encoding/json"
	"math"
)

// Contains reports whether p lies inside the area, with exact boundaries.
// Shorthand for ContainsM(p, 0).
func (a *Area) Contains(p Point) bool {
	return a.ContainsM(p, 0)
}

// ContainsM reports whether p lies inside the area, allowing a tolerance of
// bufferM meters around the boundary: a point that fails the exact test still
// counts when it sits within bufferM of the area's boundary. That is the GPS
// jitter allowance - a few meters in practice.
//
// Unlike the measurement methods, containment reads the full geometry: a
// Polygon's holes exclude (inside a hole is outside the area), and every
// MultiPolygon member counts, not just the largest. A LineString strip
// contains the points within WidthMeters/2 of the line.
//
// A negative buffer is rejected (returns false), not given shrink semantics.
// Malformed geometry returns false, matching the "0 for malformed" convention
// of the measurement methods.
func (a *Area) ContainsM(p Point, bufferM float64) bool {
	if a == nil || bufferM < 0 {
		return false
	}

	var hdr geomHeader
	if err := json.Unmarshal(a.Geometry, &hdr); err != nil {
		return false
	}

	switch hdr.Type {
	case GeomLineString:
		if a.WidthMeters == nil {
			return false
		}
		var coords [][2]float64
		if err := json.Unmarshal(hdr.Coordinates, &coords); err != nil || len(coords) < 2 {
			return false
		}
		return minDistToPolylineM(p, toPoints(coords)) <= *a.WidthMeters/2+bufferM
	case GeomPolygon:
		var rings [][][2]float64
		if err := json.Unmarshal(hdr.Coordinates, &rings); err != nil {
			return false
		}
		return containsInPolygons(p, [][][][2]float64{rings}, bufferM)
	case GeomMultiPolygon:
		var polys [][][][2]float64
		if err := json.Unmarshal(hdr.Coordinates, &polys); err != nil {
			return false
		}
		return containsInPolygons(p, polys, bufferM)
	default:
		return false
	}
}

// containsInPolygons is the shared Polygon/MultiPolygon containment: inside
// any member's outer ring while outside its holes, or - with a positive
// buffer - within bufferM of any ring. Testing the buffer against hole rings
// too is what lets a point just inside a hole be pulled back in.
func containsInPolygons(p Point, polys [][][][2]float64, bufferM float64) bool {
	for _, rings := range polys {
		if len(rings) == 0 {
			continue
		}
		if !pointInRing(p, rings[0]) {
			continue
		}
		inHole := false
		for _, hole := range rings[1:] {
			if pointInRing(p, hole) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}

	if bufferM <= 0 {
		return false
	}
	for _, rings := range polys {
		for _, ring := range rings {
			if minDistToPolylineM(p, toPoints(ring)) <= bufferM {
				return true
			}
		}
	}
	return false
}

// pointInRing is even-odd ray casting straight on the [lng, lat] coordinates.
// Parity is preserved by the (monotonic, per-axis) projection, so no meters
// conversion is needed for the inside/outside answer itself.
func pointInRing(p Point, ring [][2]float64) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		lngI, latI := ring[i][0], ring[i][1]
		lngJ, latJ := ring[j][0], ring[j][1]
		if (latI > p.Lat) != (latJ > p.Lat) &&
			p.Lng < (lngJ-lngI)*(p.Lat-latI)/(latJ-latI)+lngI {
			in = !in
		}
	}
	return in
}

// minDistToPolylineM returns the distance in meters from p to the nearest
// segment of the polyline (an open line or a closed ring alike - a ring's
// closing segment is present as its duplicated last vertex).
func minDistToPolylineM(p Point, pts []Point) float64 {
	best := math.Inf(1)
	for i := 1; i < len(pts); i++ {
		if d := distToSegmentM(p, pts[i-1], pts[i]); d < best {
			best = d
		}
	}
	return best
}

// distToSegmentM returns the distance in meters from p to the segment ab,
// computed in the local plane projected around p (so p itself is the origin).
func distToSegmentM(p, a, b Point) float64 {
	ax, ay := project(a, p)
	bx, by := project(b, p)
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Hypot(ax, ay)
	}
	// t is the projection of the origin (p) onto ab, clamped to the segment.
	t := -(ax*dx + ay*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(ax+t*dx, ay+t*dy)
}
