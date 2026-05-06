// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import "math"

// Web Mercator constants. The Earth is modelled as a sphere with radius
// EarthRadiusMeters (a simplification — Vincenty in distance.go uses an
// ellipsoid for high-precision metric distances).
const (
	// TileSize is the canonical OpenStreetMap raster tile edge in pixels.
	TileSize = 256

	// MaxMercatorLat is the latitude clipping point for Web Mercator
	// (EPSG:3857). At this latitude the projection becomes a square world
	// of size 2π·R = (TileSize · 2^zoom) pixels per side.
	MaxMercatorLat = 85.05112877980659

	// EarthRadiusMeters is the mean Earth radius used by spherical
	// computations (haversine, Web Mercator).
	EarthRadiusMeters = 6378137.0

	// MinZoom and MaxZoom mirror the OSM slippy-map convention.
	MinZoom = 0
	MaxZoom = 19
)

// WorldSize returns the side length of the Web Mercator projected world, in
// pixels, at the given zoom level. The world is square and ranges over
// [0, WorldSize) on each axis.
func WorldSize(zoom int) float64 {
	return float64(TileSize) * math.Exp2(float64(zoom))
}

// ClampZoom clamps z to [MinZoom, MaxZoom].
func ClampZoom(z int) int {
	if z < MinZoom {
		return MinZoom
	}
	if z > MaxZoom {
		return MaxZoom
	}
	return z
}

// LatLngToWorldPixel projects a coordinate to absolute world pixel space at
// the given zoom level. The result is fractional — quantisation to integer
// pixels is the caller's responsibility.
//
// Reference: https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
func LatLngToWorldPixel(p LatLng, zoom int) (px, py float64) {
	lat := ClampLat(p.Lat) * math.Pi / 180
	world := WorldSize(zoom)
	px = (p.Lng + 180) / 360 * world
	py = (1 - math.Log(math.Tan(lat)+1/math.Cos(lat))/math.Pi) / 2 * world
	return px, py
}

// WorldPixelToLatLng is the inverse of [LatLngToWorldPixel]. Pixels outside the
// [0, WorldSize) range are silently wrapped/clamped to keep the result valid.
func WorldPixelToLatLng(px, py float64, zoom int) LatLng {
	world := WorldSize(zoom)
	lng := px/world*360 - 180
	n := math.Pi - 2*math.Pi*py/world
	lat := 180 / math.Pi * math.Atan(math.Sinh(n))
	return LatLng{Lat: lat, Lng: NormalizeLng(lng)}
}

// LatLngToTile returns the slippy-map tile coordinates that contain p at the
// given zoom. Tiles are 256×256 pixels.
func LatLngToTile(p LatLng, zoom int) (x, y int) {
	px, py := LatLngToWorldPixel(p, zoom)
	x = int(math.Floor(px / TileSize))
	y = int(math.Floor(py / TileSize))
	maxIdx := int(math.Exp2(float64(zoom))) - 1
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > maxIdx {
		x = maxIdx
	}
	if y > maxIdx {
		y = maxIdx
	}
	return x, y
}

// TileBounds returns the geographic bounding box covered by the slippy-map
// tile (z, x, y). Useful when rendering a tile or determining the area covered
// by a fetched payload.
func TileBounds(zoom, x, y int) BBox {
	nw := WorldPixelToLatLng(float64(x*TileSize), float64(y*TileSize), zoom)
	se := WorldPixelToLatLng(float64((x+1)*TileSize), float64((y+1)*TileSize), zoom)
	return BBox{South: se.Lat, West: nw.Lng, North: nw.Lat, East: se.Lng}
}

// MetersPerPixel returns the ground resolution at a given latitude and zoom,
// in metres per Web-Mercator pixel. Used to compute the in-app scale bar.
func MetersPerPixel(lat float64, zoom int) float64 {
	return math.Cos(lat*math.Pi/180) * 2 * math.Pi * EarthRadiusMeters / WorldSize(zoom)
}
