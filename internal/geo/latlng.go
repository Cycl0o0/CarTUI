// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package geo provides geographic primitives: coordinates, bounding boxes,
// projections (Web Mercator), and great-circle distance computations.
package geo

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// LatLng is a WGS84 geographic coordinate expressed in decimal degrees.
//
// Latitude is bounded to [-90, +90] and longitude to [-180, +180]. Values
// outside these ranges are accepted by the type but rejected by [LatLng.Valid].
type LatLng struct {
	Lat float64
	Lng float64
}

// NewLatLng constructs a LatLng and returns an error when the coordinates lie
// outside their geographic ranges.
func NewLatLng(lat, lng float64) (LatLng, error) {
	ll := LatLng{Lat: lat, Lng: lng}
	if !ll.Valid() {
		return LatLng{}, fmt.Errorf("invalid coordinates: lat=%v lng=%v", lat, lng)
	}
	return ll, nil
}

// Valid reports whether the coordinate falls inside the WGS84 ranges and is
// finite (not NaN or ±Inf).
func (l LatLng) Valid() bool {
	if math.IsNaN(l.Lat) || math.IsNaN(l.Lng) {
		return false
	}
	if math.IsInf(l.Lat, 0) || math.IsInf(l.Lng, 0) {
		return false
	}
	return l.Lat >= -90 && l.Lat <= 90 && l.Lng >= -180 && l.Lng <= 180
}

// String formats the coordinate with six decimal digits — about 11cm precision
// at the equator, sufficient for any UI need.
func (l LatLng) String() string {
	return fmt.Sprintf("%.6f,%.6f", l.Lat, l.Lng)
}

// ParseLatLng parses a "lat,lng" string. Whitespace around the comma is
// tolerated. The function rejects malformed input or out-of-range values.
func ParseLatLng(s string) (LatLng, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return LatLng{}, errors.New("expected \"lat,lng\" with a single comma")
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return LatLng{}, fmt.Errorf("parse lat: %w", err)
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return LatLng{}, fmt.Errorf("parse lng: %w", err)
	}
	return NewLatLng(lat, lng)
}

// ClampLat constrains a latitude to the Web Mercator usable band
// [-MaxMercatorLat, MaxMercatorLat] (~±85.05113°). Past this band the
// projection becomes singular.
func ClampLat(lat float64) float64 {
	if lat > MaxMercatorLat {
		return MaxMercatorLat
	}
	if lat < -MaxMercatorLat {
		return -MaxMercatorLat
	}
	return lat
}

// NormalizeLng wraps a longitude into [-180, 180].
func NormalizeLng(lng float64) float64 {
	for lng > 180 {
		lng -= 360
	}
	for lng < -180 {
		lng += 360
	}
	return lng
}
