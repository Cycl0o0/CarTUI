// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	paris = LatLng{48.8566, 2.3522}
	nyc   = LatLng{40.7128, -74.0060}
	la    = LatLng{34.0522, -118.2437}
)

func TestHaversineKnownDistances(t *testing.T) {
	// Reference values computed against the equatorial sphere (R = 6378137 m,
	// matching [EarthRadiusMeters]). Mean-radius implementations give ~6 km
	// less; both are acceptable for in-app distance display.
	d := Haversine(paris, nyc) / 1000
	assert.InDelta(t, 5840, d, 10)

	d = Haversine(nyc, la) / 1000
	assert.InDelta(t, 3940, d, 10)

	assert.Equal(t, 0.0, Haversine(paris, paris))
}

func TestVincentyAgreesWithHaversine(t *testing.T) {
	// Within ~0.5% over typical distances.
	d1 := Haversine(paris, nyc)
	d2 := Vincenty(paris, nyc)
	assert.InEpsilon(t, d1, d2, 0.005)
}

func TestVincentyZero(t *testing.T) {
	assert.Equal(t, 0.0, Vincenty(paris, paris))
}

func TestVincentyShortDistance(t *testing.T) {
	// Pont de Pierre, Bordeaux -> Place de la Bourse, Bordeaux ~ 350 m.
	a := LatLng{44.8388, -0.5675}
	b := LatLng{44.8413, -0.5703}
	d := Vincenty(a, b)
	assert.Greater(t, d, 200.0)
	assert.Less(t, d, 500.0)
}

func TestPathLength(t *testing.T) {
	assert.Equal(t, 0.0, PathLength(nil))
	assert.Equal(t, 0.0, PathLength([]LatLng{paris}))

	d := PathLength([]LatLng{paris, nyc, la})
	expected := Haversine(paris, nyc) + Haversine(nyc, la)
	assert.InDelta(t, expected, d, 1.0)
}
