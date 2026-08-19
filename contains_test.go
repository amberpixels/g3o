package g3o_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/g3o"
)

func TestContainsSquare(t *testing.T) {
	sq := square100(t)
	cases := []struct {
		name    string
		p       g3o.Point
		bufferM float64
		want    bool
	}{
		{"center", offsetM(50, 50), 0, true},
		{"outside", offsetM(200, 50), 0, false},
		{"just outside, no buffer", offsetM(103, 50), 0, false},
		{"just outside, buffered in", offsetM(103, 50), 5, true},
		{"well outside, buffer too small", offsetM(120, 50), 5, false},
		{"negative buffer rejected", offsetM(50, 50), -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sq.ContainsM(tc.p, tc.bufferM); got != tc.want {
				t.Errorf("ContainsM = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestContainsPolygonWithHole(t *testing.T) {
	// 100m square with a 40..60m hole in the middle.
	a := polygonArea(t,
		ring([2]float64{0, 0}, [2]float64{100, 0}, [2]float64{100, 100}, [2]float64{0, 100}),
		ring([2]float64{40, 40}, [2]float64{60, 40}, [2]float64{60, 60}, [2]float64{40, 60}),
	)
	if !a.Contains(offsetM(20, 20)) {
		t.Error("point between outer ring and hole should be inside")
	}
	if a.Contains(offsetM(50, 50)) {
		t.Error("point in the hole should be outside")
	}
	// 2m inside the hole, 5m buffer: pulled back in via the hole boundary.
	if !a.ContainsM(offsetM(42, 50), 5) {
		t.Error("point just inside the hole should be buffered back in")
	}
}

func TestContainsMultiPolygonSmallMember(t *testing.T) {
	// A large member and a small distant member. decodeShape-based methods
	// only see the large one; containment must see both.
	a := multiPolygonArea(t,
		[][][2]float64{ring([2]float64{0, 0}, [2]float64{100, 0}, [2]float64{100, 100}, [2]float64{0, 100})},
		[][][2]float64{ring([2]float64{300, 300}, [2]float64{320, 300}, [2]float64{320, 320}, [2]float64{300, 320})},
	)
	if !a.Contains(offsetM(310, 310)) {
		t.Error("point in the small MultiPolygon member should be inside")
	}
	if a.Contains(offsetM(200, 200)) {
		t.Error("point between the members should be outside")
	}
}

func TestContainsStrip(t *testing.T) {
	// 200m east-west strip, 10m wide: contains up to 5m off its axis.
	strip := stripArea(t, 10, [2]float64{0, 0}, [2]float64{200, 0})
	cases := []struct {
		name    string
		p       g3o.Point
		bufferM float64
		want    bool
	}{
		{"on the line", offsetM(100, 0), 0, true},
		{"4m off axis", offsetM(100, 4), 0, true},
		{"8m off axis", offsetM(100, 8), 0, false},
		{"8m off axis, buffered", offsetM(100, 8), 4, true},
		{"beyond the end", offsetM(210, 0), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strip.ContainsM(tc.p, tc.bufferM); got != tc.want {
				t.Errorf("ContainsM = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestContainsMalformed(t *testing.T) {
	width := 5.0
	for name, a := range map[string]*g3o.Area{
		"nil area":              nil,
		"empty geometry":        {},
		"bad json":              {Geometry: json.RawMessage(`{`)},
		"unsupported type":      {Geometry: json.RawMessage(`{"type":"Point","coordinates":[28.83,47.02]}`)},
		"strip without width":   {Geometry: json.RawMessage(`{"type":"LineString","coordinates":[[28.83,47.02],[28.84,47.02]]}`)},
		"single-point line":     {Geometry: json.RawMessage(`{"type":"LineString","coordinates":[[28.83,47.02]]}`), WidthMeters: &width},
		"polygon with no rings": {Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[]}`)},
	} {
		if a.ContainsM(offsetM(50, 50), 10) {
			t.Errorf("%s: ContainsM = true, want false", name)
		}
	}
}
