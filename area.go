package geo

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Geometry types an Area accepts.
const (
	GeomLineString   = "LineString"
	GeomPolygon      = "Polygon"
	GeomMultiPolygon = "MultiPolygon"
)

// Sanity bounds - keep payloads small and the UI editable.
const (
	MinWidthMeters = 1.0
	MaxWidthMeters = 100.0
	MaxVertices    = 500
	MaxAreas       = 20
)

// Area is a named-place footprint: a GeoJSON geometry, plus a width when the
// geometry is a LineString (a "strip" - a trail, an embankment, a street
// segment). WidthMeters is required iff Geometry.Type == LineString.
//
// The JSON shape ({geometry, widthMeters}) is a storage contract shared by
// the consuming apps: rows already persist it and map editors produce it, so
// it must not drift.
type Area struct {
	Geometry    json.RawMessage `json:"geometry"`
	WidthMeters *float64        `json:"widthMeters,omitempty"`
}

// geomHeader is just enough to discriminate on type without fully decoding coordinates.
type geomHeader struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// Validate ensures the area is well-formed. Returns the geometry type detected.
func (a *Area) Validate() (string, error) {
	if a == nil {
		return "", errors.New("area: nil")
	}
	if len(a.Geometry) == 0 {
		return "", errors.New("area: geometry is required")
	}

	var hdr geomHeader
	if err := json.Unmarshal(a.Geometry, &hdr); err != nil {
		return "", fmt.Errorf("area: invalid geometry json: %w", err)
	}

	switch hdr.Type {
	case GeomLineString:
		if a.WidthMeters == nil {
			return "", errors.New("area: widthMeters is required for LineString")
		}
		if *a.WidthMeters < MinWidthMeters || *a.WidthMeters > MaxWidthMeters {
			return "", fmt.Errorf("area: widthMeters must be in [%g, %g]", MinWidthMeters, MaxWidthMeters)
		}
		if err := validateLineString(hdr.Coordinates); err != nil {
			return "", err
		}
	case GeomPolygon:
		if a.WidthMeters != nil {
			return "", errors.New("area: widthMeters must be omitted for Polygon")
		}
		if err := validatePolygon(hdr.Coordinates); err != nil {
			return "", err
		}
	case GeomMultiPolygon:
		if a.WidthMeters != nil {
			return "", errors.New("area: widthMeters must be omitted for MultiPolygon")
		}
		if err := validateMultiPolygon(hdr.Coordinates); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("area: unsupported geometry type %q", hdr.Type)
	}

	return hdr.Type, nil
}

func validateLineString(raw json.RawMessage) error {
	var coords [][2]float64
	if err := json.Unmarshal(raw, &coords); err != nil {
		return fmt.Errorf("area: invalid LineString coordinates: %w", err)
	}
	if len(coords) < 2 {
		return errors.New("area: LineString needs >= 2 points")
	}
	if len(coords) > MaxVertices {
		return fmt.Errorf("area: LineString exceeds %d points", MaxVertices)
	}
	return validateLngLats(coords)
}

func validatePolygon(raw json.RawMessage) error {
	var rings [][][2]float64
	if err := json.Unmarshal(raw, &rings); err != nil {
		return fmt.Errorf("area: invalid Polygon coordinates: %w", err)
	}
	return validateRings(rings)
}

func validateMultiPolygon(raw json.RawMessage) error {
	var polys [][][][2]float64
	if err := json.Unmarshal(raw, &polys); err != nil {
		return fmt.Errorf("area: invalid MultiPolygon coordinates: %w", err)
	}
	if len(polys) == 0 {
		return errors.New("area: MultiPolygon is empty")
	}
	for _, rings := range polys {
		if err := validateRings(rings); err != nil {
			return err
		}
	}
	return nil
}

func validateRings(rings [][][2]float64) error {
	if len(rings) == 0 {
		return errors.New("area: Polygon has no rings")
	}
	total := 0
	for i, ring := range rings {
		if len(ring) < 4 {
			return fmt.Errorf("area: ring %d needs >= 4 points (closed)", i)
		}
		first, last := ring[0], ring[len(ring)-1]
		if first[0] != last[0] || first[1] != last[1] {
			return fmt.Errorf("area: ring %d is not closed", i)
		}
		total += len(ring)
		if err := validateLngLats(ring); err != nil {
			return err
		}
	}
	if total > MaxVertices {
		return fmt.Errorf("area: Polygon exceeds %d vertices total", MaxVertices)
	}
	return nil
}

func validateLngLats(pts [][2]float64) error {
	for i, p := range pts {
		lng, lat := p[0], p[1]
		if lng < -180 || lng > 180 {
			return fmt.Errorf("area: point %d longitude %g out of range", i, lng)
		}
		if lat < -90 || lat > 90 {
			return fmt.Errorf("area: point %d latitude %g out of range", i, lat)
		}
	}
	return nil
}
