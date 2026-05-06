// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tiles handles slippy-map raster tiles: addressing, caching, and
// fetching. The renderer uses Overpass-derived vector data by default, so
// raster tiles are only consumed when the user explicitly opts in
// (--raster).
package tiles

import (
	"fmt"

	"github.com/cycl0o0/cartui/internal/geo"
)

// Address is a slippy-map tile reference (zoom, x, y) — the same triplet that
// appears in URLs like `https://tile.openstreetmap.org/13/4084/2952.png`.
type Address struct {
	Z, X, Y int
}

// Key returns a stable string key for use in caches and logs.
func (a Address) Key() string { return fmt.Sprintf("%d/%d/%d", a.Z, a.X, a.Y) }

// URL builds the tile URL using the OpenStreetMap pattern, replacing the
// `{z}/{x}/{y}` placeholders.
func (a Address) URL(template string) string {
	out := template
	for k, v := range map[string]string{
		"{z}": fmt.Sprintf("%d", a.Z),
		"{x}": fmt.Sprintf("%d", a.X),
		"{y}": fmt.Sprintf("%d", a.Y),
	} {
		out = replace(out, k, v)
	}
	return out
}

// CoveringTiles returns every tile address that intersects the given bbox at
// the requested zoom. The slice is ordered row-major (top-to-bottom,
// left-to-right).
func CoveringTiles(b geo.BBox, zoom int) []Address {
	zoom = geo.ClampZoom(zoom)
	x0, y0 := geo.LatLngToTile(geo.LatLng{Lat: b.North, Lng: b.West}, zoom)
	x1, y1 := geo.LatLngToTile(geo.LatLng{Lat: b.South, Lng: b.East}, zoom)
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	out := make([]Address, 0, (x1-x0+1)*(y1-y0+1))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			out = append(out, Address{Z: zoom, X: x, Y: y})
		}
	}
	return out
}

// replace is a tiny strings.Replace alias kept local to avoid pulling the
// import only for one call site.
func replace(s, old, new string) string {
	out := make([]byte, 0, len(s))
	i := 0
	for i+len(old) <= len(s) {
		if s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	out = append(out, s[i:]...)
	return string(out)
}
