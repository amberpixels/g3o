package geo

import "fmt"

// TrackStats is what a GPS track accumulated inside one area.
type TrackStats struct {
	// TimeS is the time spent inside, in seconds.
	TimeS float64
	// DistanceM is the distance covered inside, in meters.
	DistanceM float64
}

// TrackInArea measures how much of a GPS track falls inside the area: points
// and timeS are parallel slices (per-sample time offsets in seconds, the
// shape activity streams decode to), and bufferM is the same jitter tolerance
// ContainsM takes, so the two agree on what "inside" means.
//
// Attribution is per segment: both endpoints inside counts the segment's full
// dt and distance, exactly one endpoint inside counts half of each, neither
// counts nothing. At normal watch sample rates (about a point per second) the
// half-segment error at a boundary crossing is below the GPS noise floor, so
// exact intersection math would buy nothing. Negative time deltas (a stream
// glitch) count as zero rather than subtracting.
//
// The result reports measured quantities only; shares, thresholds and any
// primary-vs-visited classification are the caller's business.
func TrackInArea(points []Point, timeS []float64, a *Area, bufferM float64) (TrackStats, error) {
	if len(points) != len(timeS) {
		return TrackStats{}, fmt.Errorf(
			"geo: track points (%d) and times (%d) length mismatch",
			len(points),
			len(timeS),
		)
	}
	if len(points) < 2 {
		return TrackStats{}, nil
	}

	inside := make([]bool, len(points))
	for i, p := range points {
		inside[i] = a.ContainsM(p, bufferM)
	}

	var st TrackStats
	for i := 1; i < len(points); i++ {
		var f float64
		switch {
		case inside[i-1] && inside[i]:
			f = 1
		case inside[i-1] || inside[i]:
			f = 0.5
		default:
			continue
		}
		dt := timeS[i] - timeS[i-1]
		if dt < 0 {
			dt = 0
		}
		st.TimeS += f * dt
		st.DistanceM += f * DistanceM(points[i-1], points[i])
	}
	return st, nil
}
