// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFillPolygonRectangle(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	pts := []Point{{2, 2}, {10, 2}, {10, 8}, {2, 8}}
	c.FillPolygon(pts, LayerWater)

	for y := 2; y <= 7; y++ {
		for x := 2; x < 10; x++ {
			assert.True(t, isSet(c, x, y), "interior pixel (%d,%d) should be set", x, y)
		}
	}
	// Just outside the polygon: not set.
	assert.False(t, isSet(c, 1, 5))
	assert.False(t, isSet(c, 11, 5))
	assert.False(t, isSet(c, 5, 1))
	assert.False(t, isSet(c, 5, 9))
}

func TestFillPolygonTriangle(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	pts := []Point{{0, 0}, {10, 0}, {5, 10}}
	c.FillPolygon(pts, LayerGreen)
	// Centroid of the triangle is roughly (5, 3).
	assert.True(t, isSet(c, 5, 3))
	// Outside the triangle (below its tip).
	assert.False(t, isSet(c, 5, 11))
}

func TestFillPolygonTooFewPoints(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	c.FillPolygon(nil, LayerWater)
	c.FillPolygon([]Point{{0, 0}}, LayerWater)
	c.FillPolygon([]Point{{0, 0}, {1, 1}}, LayerWater)
	assert.Equal(t, 0, countSetPixels(c))
}

func TestFillPolygonOutOfBounds(t *testing.T) {
	c := NewCanvas(10, 10, DarkTheme)
	pts := []Point{{-5, -5}, {15, -5}, {15, 15}, {-5, 15}}
	c.FillPolygon(pts, LayerWater)
	// Every pixel inside the canvas should be set.
	for y := 0; y < c.HeightPx(); y++ {
		for x := 0; x < c.WidthPx(); x++ {
			assert.True(t, isSet(c, x, y))
		}
	}
}

func TestFillRect(t *testing.T) {
	c := NewCanvas(10, 10, DarkTheme)
	c.FillRect(2, 2, 5, 5, LayerWater)
	for y := 2; y <= 5; y++ {
		for x := 2; x <= 5; x++ {
			assert.True(t, isSet(c, x, y))
		}
	}
	assert.False(t, isSet(c, 1, 1))
}

func TestFillRectReversed(t *testing.T) {
	c := NewCanvas(10, 10, DarkTheme)
	c.FillRect(5, 5, 2, 2, LayerWater)
	assert.True(t, isSet(c, 3, 3))
}
