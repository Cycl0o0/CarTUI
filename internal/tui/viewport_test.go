// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"testing"

	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
)

func TestViewportBBoxContainsCenter(t *testing.T) {
	v := Viewport{
		Center:      geo.LatLng{Lat: 44.8378, Lng: -0.5792},
		Zoom:        13,
		WidthCells:  80,
		HeightCells: 24,
	}
	assert.True(t, v.BBox().Contains(v.Center))
}

func TestViewportPanShiftsCenter(t *testing.T) {
	v := Viewport{
		Center:      geo.LatLng{Lat: 44.8378, Lng: -0.5792},
		Zoom:        13,
		WidthCells:  80,
		HeightCells: 24,
	}
	before := v.Center
	v.Pan(2, 0)
	assert.NotEqual(t, before.Lng, v.Center.Lng)
	v.Pan(0, 2)
	assert.NotEqual(t, before.Lat, v.Center.Lat)
}

func TestViewportSetZoomClamps(t *testing.T) {
	v := Viewport{}
	v.SetZoom(99)
	assert.Equal(t, geo.MaxZoom, v.Zoom)
	v.SetZoom(-5)
	assert.Equal(t, geo.MinZoom, v.Zoom)
	v.SetZoom(10)
	assert.Equal(t, 10, v.Zoom)
}

func TestViewportLatLngToCellCenter(t *testing.T) {
	v := Viewport{
		Center:      geo.LatLng{Lat: 44.8378, Lng: -0.5792},
		Zoom:        13,
		WidthCells:  80,
		HeightCells: 24,
	}
	cx, cy := v.LatLngToCell(v.Center)
	// Centre coordinate must land near the middle of the canvas.
	assert.InDelta(t, v.WidthCells/2, cx, 2)
	assert.InDelta(t, v.HeightCells/2, cy, 2)
}

func TestViewportLatLngToCellOutOfBounds(t *testing.T) {
	v := Viewport{
		Center:      geo.LatLng{Lat: 0, Lng: 0},
		Zoom:        13,
		WidthCells:  20,
		HeightCells: 8,
	}
	// A point on the opposite side of the world is far outside the
	// viewport at z=13.
	cx, cy := v.LatLngToCell(geo.LatLng{Lat: 60, Lng: 60})
	assert.Equal(t, -1, cx)
	assert.Equal(t, -1, cy)
}

func TestViewportMetersPerCellDecreasesWithZoom(t *testing.T) {
	v := Viewport{Center: geo.LatLng{Lat: 0, Lng: 0}, Zoom: 5, WidthCells: 80, HeightCells: 24}
	a := v.MetersPerCell()
	v.Zoom = 10
	b := v.MetersPerCell()
	assert.Less(t, b, a)
}
