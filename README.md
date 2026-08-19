<p align="center">
  <img src="logo.svg" alt="geo" width="208">
</p>

<div align="center">

### Know your place.

Dependency-free WGS84/GeoJSON geometry for Go: points, areas, containment, GPS-track stats.

[![Go Reference](https://pkg.go.dev/badge/github.com/amberpixels/geo.svg)](https://pkg.go.dev/github.com/amberpixels/geo)
[![Go Version](https://img.shields.io/github/go-mod/go-version/amberpixels/geo)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)

</div>

---

geo answers the questions city-scale apps keep asking about named places: how big is this park, where is its center, is this GPS point inside it, and how much of this run happened there. An `Area` is a GeoJSON `Polygon`, `MultiPolygon`, or `LineString` strip (a line plus a width in meters - a trail, an embankment); everything else is functions over it.

All planar math uses a local equirectangular projection, which is accurate to well under GPS noise at park-and-street scale. The library is stdlib-only.

> [!NOTE]
> City-scale on purpose: don't feed it continent-sized shapes or geometries crossing the antimeridian.

## Install

```bash
go get github.com/amberpixels/geo
```

## Quick Start

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/geo"
)

func main() {
	park := &geo.Area{Geometry: json.RawMessage(`{
		"type": "Polygon",
		"coordinates": [[
			[28.8225, 47.0210], [28.8317, 47.0210],
			[28.8317, 47.0271], [28.8225, 47.0271],
			[28.8225, 47.0210]
		]]
	}`)}

	if _, err := park.Validate(); err != nil {
		panic(err)
	}

	fmt.Printf("%.0f m² around ", park.SizeM2())
	center, _ := park.Center()
	fmt.Println(center)

	// Is this GPS sample in the park? Allow 5 m of jitter.
	fmt.Println(park.ContainsM(geo.Point{Lat: 47.024, Lng: 28.827}, 5))
}
```

## Areas

`Area` is a storage-friendly value: a raw GeoJSON geometry plus `WidthMeters` (required iff the geometry is a `LineString`). Its JSON shape `{geometry, widthMeters}` is meant to be persisted as-is and round-trips byte-for-byte.

- `Validate()` - checks geometry type, ring closure, coordinate ranges, and the sanity bounds (`MaxVertices`, width in `[MinWidthMeters, MaxWidthMeters]`).
- `SizeM2()`, `Center()`, `MainExtentM()`, `PointAlong(t)` - measurement and placement. These read a simplified geometry (the outer ring; a MultiPolygon's largest member), which is exactly enough for ranking, centering, and spreading points over an area.

Plus the standalone `DistanceM(a, b Point)` for plain point-to-point meters.

## Containment

Containment reads the full geometry, unlike the measurement methods: Polygon holes exclude, every MultiPolygon member counts, and a strip contains whatever lies within `WidthMeters/2` of its line.

```go
area.Contains(p)        // exact boundaries
area.ContainsM(p, 5)    // p counts if within 5 m of the boundary (GPS jitter)
```

Malformed geometry is never an error here - it just contains nothing.

## Track Stats

`TrackInArea` measures how much of a GPS track fell inside an area: points and per-sample time offsets (seconds) come as parallel slices, the same buffer tolerance applies, and the result is time-inside and distance-inside.

```go
stats, err := geo.TrackInArea(points, timeS, park, 5)
// stats.TimeS, stats.DistanceM
```

Segments fully inside count whole, boundary-crossing segments count half - at watch sample rates the difference from exact intersection math is below GPS noise. What a share means (a visit? the run's main location?) is deliberately left to the caller.

## Feedback

geo is a solo, opinionated project - but if you stumbled upon it and have
ideas, questions, or bug reports, an [issue](https://github.com/amberpixels/geo/issues) is always welcome :)

## License

[MIT](LICENSE) © [amberpixels](https://amberpixels.io)
