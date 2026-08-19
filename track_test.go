package g3o_test

import (
	"testing"

	"github.com/amberpixels/g3o"
)

// walkEast builds a track of one point per second moving 5 m/s east, starting
// startXM meters east of the fixture origin, staying on the square's midline.
func walkEast(startXM float64, seconds int) ([]g3o.Point, []float64) {
	pts := make([]g3o.Point, seconds+1)
	times := make([]float64, seconds+1)
	for i := 0; i <= seconds; i++ {
		pts[i] = offsetM(startXM+float64(i)*5, 50)
		times[i] = float64(i)
	}
	return pts, times
}

func TestTrackInAreaCrossing(t *testing.T) {
	sq := square100(t)

	// 60 seconds at 5 m/s from x=-100 to x=200: 100m before the square, 100m
	// inside, 100m after.
	pts, times := walkEast(-100, 60)
	st, err := g3o.TrackInArea(pts, times, sq, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Samples land exactly on both edges; the ray-cast tie-break counts the
	// entry edge (x=0) as inside and the exit edge (x=100) as outside. So the
	// inside samples are x=0..95 (20 points, 19 full segments) and the two
	// crossing segments (-5..0 and 95..100) add half each: 20 segments worth
	// of time (20s) and distance (100m).
	wantNear(t, st.TimeS, 20, 0.01, "crossing TimeS")
	wantNear(t, st.DistanceM, 100, 0.5, "crossing DistanceM")
}

func TestTrackInAreaFullyInsideAndOutside(t *testing.T) {
	sq := square100(t)

	pts, times := walkEast(10, 10) // x=10..60, entirely inside
	st, err := g3o.TrackInArea(pts, times, sq, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantNear(t, st.TimeS, 10, 0.01, "inside TimeS")
	wantNear(t, st.DistanceM, 50, 0.3, "inside DistanceM")

	pts, times = walkEast(500, 10) // far east of the square
	st, err = g3o.TrackInArea(pts, times, sq, 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.TimeS != 0 || st.DistanceM != 0 {
		t.Errorf("outside track accumulated %+v, want zero", st)
	}
}

func TestTrackInAreaEdgeCases(t *testing.T) {
	sq := square100(t)

	if _, err := g3o.TrackInArea(make([]g3o.Point, 3), make([]float64, 2), sq, 0); err == nil {
		t.Error("length mismatch: want error")
	}

	st, err := g3o.TrackInArea(nil, nil, sq, 0)
	if err != nil || st != (g3o.TrackStats{}) {
		t.Errorf("empty track: got %+v, %v; want zero, nil", st, err)
	}

	st, err = g3o.TrackInArea([]g3o.Point{offsetM(50, 50)}, []float64{0}, sq, 0)
	if err != nil || st != (g3o.TrackStats{}) {
		t.Errorf("single point: got %+v, %v; want zero, nil", st, err)
	}

	// A negative time delta (stream glitch) contributes zero, not negative.
	pts := []g3o.Point{offsetM(50, 50), offsetM(55, 50)}
	st, err = g3o.TrackInArea(pts, []float64{10, 4}, sq, 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.TimeS != 0 {
		t.Errorf("negative dt: TimeS = %g, want 0", st.TimeS)
	}
	wantNear(t, st.DistanceM, 5, 0.1, "negative dt DistanceM")
}

func TestTrackInAreaBufferWidensInside(t *testing.T) {
	sq := square100(t)

	// A track hugging the outside of the east edge, 3m out: exact containment
	// sees nothing, a 5m buffer sees all of it.
	pts := make([]g3o.Point, 11)
	times := make([]float64, 11)
	for i := range pts {
		pts[i] = offsetM(103, float64(i*10))
		times[i] = float64(i)
	}
	st, err := g3o.TrackInArea(pts, times, sq, 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.TimeS != 0 {
		t.Errorf("unbuffered TimeS = %g, want 0", st.TimeS)
	}
	st, err = g3o.TrackInArea(pts, times, sq, 5)
	if err != nil {
		t.Fatal(err)
	}
	wantNear(t, st.TimeS, 10, 0.01, "buffered TimeS")
}
