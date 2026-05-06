// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorldSize(t *testing.T) {
	assert.Equal(t, 256.0, WorldSize(0))
	assert.Equal(t, 512.0, WorldSize(1))
	assert.Equal(t, 65536.0, WorldSize(8))
}

func TestLatLngTileRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		ll   LatLng
		zoom int
	}{
		{"bordeaux z13", LatLng{44.8378, -0.5792}, 13},
		{"new york z10", LatLng{40.7128, -74.0060}, 10},
		{"sydney z8", LatLng{-33.8688, 151.2093}, 8},
		{"equator origin z5", LatLng{0, 0}, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			px, py := LatLngToWorldPixel(tc.ll, tc.zoom)
			back := WorldPixelToLatLng(px, py, tc.zoom)
			assert.InDelta(t, tc.ll.Lat, back.Lat, 1e-6)
			assert.InDelta(t, tc.ll.Lng, back.Lng, 1e-6)
		})
	}
}

func TestLatLngToTileKnownValues(t *testing.T) {
	// Reference values from
	// https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames#Implementations
	// for (51.50, -0.12) at zoom 9 -> (255, 170).
	x, y := LatLngToTile(LatLng{51.50, -0.12}, 9)
	assert.Equal(t, 255, x)
	assert.Equal(t, 170, y)

	// Origin tile at zoom 0 is (0,0).
	x, y = LatLngToTile(LatLng{0, 0}, 0)
	assert.Equal(t, 0, x)
	assert.Equal(t, 0, y)
}

func TestTileBoundsCoversCenter(t *testing.T) {
	zoom := 12
	ll := LatLng{44.8378, -0.5792}
	tx, ty := LatLngToTile(ll, zoom)
	b := TileBounds(zoom, tx, ty)
	assert.True(t, b.Contains(ll), "tile bounds should contain the originating point")
}

func TestMetersPerPixel(t *testing.T) {
	// At z=0 / equator: ~156543 m/px (well-known value).
	mpp := MetersPerPixel(0, 0)
	assert.InDelta(t, 156543.0, mpp, 1.0)

	// At z=10 / equator: ~152.87 m/px.
	mpp = MetersPerPixel(0, 10)
	assert.InDelta(t, 152.87, mpp, 0.1)

	// At z=10 / 60° lat: half the equatorial value.
	mpp60 := MetersPerPixel(60, 10)
	assert.InDelta(t, mpp/2, mpp60, 0.5)
}

func TestClampZoom(t *testing.T) {
	assert.Equal(t, MinZoom, ClampZoom(-1))
	assert.Equal(t, MaxZoom, ClampZoom(99))
	assert.Equal(t, 5, ClampZoom(5))
}
