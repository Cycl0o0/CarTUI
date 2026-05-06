// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import (
	"errors"
	"fmt"
)

// BBox is an axis-aligned latitude/longitude bounding box.
//
// The "South" and "West" fields are inclusive; "North" and "East" are also
// inclusive. The box is always normalised so South ≤ North; East may wrap
// around the antimeridian when West > East.
type BBox struct {
	South, West, North, East float64
}

// NewBBox builds a bounding box and returns an error if the latitudes are
// inverted or the coordinates are outside WGS84 ranges.
func NewBBox(south, west, north, east float64) (BBox, error) {
	b := BBox{South: south, West: west, North: north, East: east}
	if !b.Valid() {
		return BBox{}, fmt.Errorf("invalid bbox: %v", b)
	}
	return b, nil
}

// FromCenter returns a BBox centred on c with the given full-width spans in
// degrees. Useful when computing a viewport from a single point.
func FromCenter(c LatLng, latSpan, lngSpan float64) BBox {
	return BBox{
		South: c.Lat - latSpan/2,
		West:  c.Lng - lngSpan/2,
		North: c.Lat + latSpan/2,
		East:  c.Lng + lngSpan/2,
	}
}

// Valid reports whether the box has plausible coordinates.
func (b BBox) Valid() bool {
	if b.South < -90 || b.North > 90 || b.South > b.North {
		return false
	}
	if b.West < -180 || b.East > 180 {
		return false
	}
	return true
}

// Contains reports whether p lies within b. Points on the boundary are
// considered inside.
func (b BBox) Contains(p LatLng) bool {
	if p.Lat < b.South || p.Lat > b.North {
		return false
	}
	if b.West <= b.East {
		return p.Lng >= b.West && p.Lng <= b.East
	}
	// Wraps the antimeridian: covers [West, 180] ∪ [-180, East].
	return p.Lng >= b.West || p.Lng <= b.East
}

// Center returns the geometric centre of the box.
func (b BBox) Center() LatLng {
	lng := (b.West + b.East) / 2
	if b.West > b.East {
		lng = b.West + (360+b.East-b.West)/2
		lng = NormalizeLng(lng)
	}
	return LatLng{Lat: (b.South + b.North) / 2, Lng: lng}
}

// Span returns the (latitude, longitude) extent of the box in degrees.
func (b BBox) Span() (latSpan, lngSpan float64) {
	latSpan = b.North - b.South
	if b.West <= b.East {
		lngSpan = b.East - b.West
	} else {
		lngSpan = 360 - b.West + b.East
	}
	return latSpan, lngSpan
}

// Expand inflates the box by latPad and lngPad on each side and returns the
// resulting box. Useful when fetching tiles slightly beyond the visible area.
func (b BBox) Expand(latPad, lngPad float64) BBox {
	return BBox{
		South: math_max(-90, b.South-latPad),
		North: math_min(90, b.North+latPad),
		West:  NormalizeLng(b.West - lngPad),
		East:  NormalizeLng(b.East + lngPad),
	}
}

// String renders the box in "south,west,north,east" Overpass/OSM order.
func (b BBox) String() string {
	return fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", b.South, b.West, b.North, b.East)
}

// ParseBBox parses "south,west,north,east".
func ParseBBox(s string) (BBox, error) {
	var south, west, north, east float64
	n, err := fmt.Sscanf(s, "%f,%f,%f,%f", &south, &west, &north, &east)
	if err != nil || n != 4 {
		return BBox{}, errors.New("expected \"south,west,north,east\"")
	}
	return NewBBox(south, west, north, east)
}

// math_min/math_max — small helpers using the float built-ins, kept private to
// avoid clashing with the package-level builtins on older Go versions.
func math_min(a, b float64) float64 { //revive:disable-line:var-naming
	if a < b {
		return a
	}
	return b
}

func math_max(a, b float64) float64 { //revive:disable-line:var-naming
	if a > b {
		return a
	}
	return b
}
