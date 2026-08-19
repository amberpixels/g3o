package g3o_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/amberpixels/g3o"
)

// The fixtures live near Chișinău (47.02N, 28.83E) so the projection math is
// exercised at a realistic latitude, where a degree of longitude is ~76 km,
// not ~111 km.

const (
	lat0 = 47.02
	lng0 = 28.83
)

// offsetM returns the point dxM meters east and dyM meters north of the
// fixture origin, inverting the library's own equirectangular projection.
func offsetM(dxM, dyM float64) g3o.Point {
	return g3o.Point{
		Lat: lat0 + dyM/111_320.0,
		Lng: lng0 + dxM/(111_320.0*math.Cos(lat0*math.Pi/180)),
	}
}

// ring builds a closed [lng, lat] ring from corner offsets in meters.
func ring(corners ...[2]float64) [][2]float64 {
	out := make([][2]float64, 0, len(corners)+1)
	for _, c := range corners {
		p := offsetM(c[0], c[1])
		out = append(out, [2]float64{p.Lng, p.Lat})
	}
	out = append(out, out[0])
	return out
}

func polygonArea(t *testing.T, rings ...[][2]float64) *g3o.Area {
	t.Helper()
	coords, err := json.Marshal(rings)
	if err != nil {
		t.Fatal(err)
	}
	return &g3o.Area{Geometry: json.RawMessage(`{"type":"Polygon","coordinates":` + string(coords) + `}`)}
}

func multiPolygonArea(t *testing.T, polys ...[][][2]float64) *g3o.Area {
	t.Helper()
	coords, err := json.Marshal(polys)
	if err != nil {
		t.Fatal(err)
	}
	return &g3o.Area{Geometry: json.RawMessage(`{"type":"MultiPolygon","coordinates":` + string(coords) + `}`)}
}

func stripArea(t *testing.T, widthM float64, pts ...[2]float64) *g3o.Area {
	t.Helper()
	coords := make([][2]float64, len(pts))
	for i, c := range pts {
		p := offsetM(c[0], c[1])
		coords[i] = [2]float64{p.Lng, p.Lat}
	}
	raw, err := json.Marshal(coords)
	if err != nil {
		t.Fatal(err)
	}
	return &g3o.Area{
		Geometry:    json.RawMessage(`{"type":"LineString","coordinates":` + string(raw) + `}`),
		WidthMeters: &widthM,
	}
}

// square100 is the 100m x 100m square with corners (0,0)..(100,100), in
// meter offsets from the fixture origin.
func square100(t *testing.T) *g3o.Area {
	t.Helper()
	return polygonArea(t, ring([2]float64{0, 0}, [2]float64{100, 0}, [2]float64{100, 100}, [2]float64{0, 100}))
}

func wantNear(t *testing.T, got, want, tolerance float64, what string) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %g, want %g +- %g", what, got, want, tolerance)
	}
}

func TestDistanceM(t *testing.T) {
	// 300m east, 400m north: a 3-4-5 triangle.
	wantNear(t, g3o.DistanceM(offsetM(0, 0), offsetM(300, 400)), 500, 0.5, "DistanceM")
	if d := g3o.DistanceM(offsetM(50, 50), offsetM(50, 50)); d != 0 {
		t.Errorf("DistanceM to itself = %g, want 0", d)
	}
}

func TestSizeM2AndCenter(t *testing.T) {
	sq := square100(t)
	wantNear(t, sq.SizeM2(), 100*100, 100, "square SizeM2")

	c, ok := sq.Center()
	if !ok {
		t.Fatal("Center not ok")
	}
	wantNear(t, g3o.DistanceM(c, offsetM(50, 50)), 0, 1, "square center offset")
}

func TestMainExtentM(t *testing.T) {
	// Longest axis of the 100m square is its ~141.4m diagonal.
	wantNear(t, square100(t).MainExtentM(), 100*math.Sqrt2, 1, "square MainExtentM")

	// A 250m strip's main extent is its arc length.
	wantNear(t, stripArea(t, 10, [2]float64{0, 0}, [2]float64{250, 0}).MainExtentM(), 250, 1, "strip MainExtentM")
}

func TestPointAlong(t *testing.T) {
	strip := stripArea(t, 10, [2]float64{0, 0}, [2]float64{200, 0})
	p, ok := strip.PointAlong(0.25)
	if !ok {
		t.Fatal("PointAlong not ok")
	}
	wantNear(t, g3o.DistanceM(p, offsetM(50, 0)), 0, 1, "strip PointAlong(0.25)")
}

func TestCenterDegenerateRing(t *testing.T) {
	// Zero-area ring (all points on a line): centroid falls back to the
	// vertex average.
	a := polygonArea(t, ring([2]float64{0, 0}, [2]float64{100, 0}, [2]float64{200, 0}))
	c, ok := a.Center()
	if !ok {
		t.Fatal("Center not ok on degenerate ring")
	}
	wantNear(t, g3o.DistanceM(c, offsetM(100, 0)), 0, 1, "degenerate ring center")
}

func TestValidate(t *testing.T) {
	width := 5.0
	tooWide := 500.0
	cases := []struct {
		name    string
		area    *g3o.Area
		wantErr string // empty = valid
		want    string // expected geometry type when valid
	}{
		{"polygon ok", square100(t), "", g3o.GeomPolygon},
		{"strip ok", stripArea(t, 5, [2]float64{0, 0}, [2]float64{100, 0}), "", g3o.GeomLineString},
		{"nil geometry", &g3o.Area{}, "geometry is required", ""},
		{"bad json", &g3o.Area{Geometry: json.RawMessage(`{`)}, "invalid geometry json", ""},
		{
			"unsupported type",
			&g3o.Area{Geometry: json.RawMessage(`{"type":"Point","coordinates":[28.83,47.02]}`)},
			"unsupported geometry type",
			"",
		},
		{
			"linestring without width",
			&g3o.Area{Geometry: json.RawMessage(`{"type":"LineString","coordinates":[[28.83,47.02],[28.84,47.02]]}`)},
			"widthMeters is required", "",
		},
		{
			"linestring width out of bounds",
			&g3o.Area{
				Geometry:    json.RawMessage(`{"type":"LineString","coordinates":[[28.83,47.02],[28.84,47.02]]}`),
				WidthMeters: &tooWide,
			},
			"widthMeters must be in",
			"",
		},
		{
			"polygon with width",
			&g3o.Area{Geometry: square100(t).Geometry, WidthMeters: &width},
			"widthMeters must be omitted", "",
		},
		{
			"open ring",
			&g3o.Area{
				Geometry: json.RawMessage(
					`{"type":"Polygon","coordinates":[[[28.83,47.02],[28.84,47.02],[28.84,47.03],[28.83,47.03]]]}`,
				),
			},
			"not closed",
			"",
		},
		{
			"longitude out of range",
			&g3o.Area{
				Geometry:    json.RawMessage(`{"type":"LineString","coordinates":[[181,47.02],[28.84,47.02]]}`),
				WidthMeters: &width,
			},
			"longitude",
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.area.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				if got != tc.want {
					t.Fatalf("Validate() type = %q, want %q", got, tc.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
