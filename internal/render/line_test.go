// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// countSetPixels traverses the canvas and counts pixels that are non-empty.
func countSetPixels(c *Canvas) int {
	n := 0
	for y := 0; y < c.HeightPx(); y++ {
		for x := 0; x < c.WidthPx(); x++ {
			if isSet(c, x, y) {
				n++
			}
		}
	}
	return n
}

func isSet(c *Canvas, px, py int) bool {
	cx := px / SubCellWidth
	cy := py / SubCellHeight
	dots, _, _, _ := c.CellAt(cx, cy)
	col := px % SubCellWidth
	row := py % SubCellHeight
	return dots&brailleDotBits[col][row] != 0
}

func TestDrawLineHorizontal(t *testing.T) {
	c := NewCanvas(20, 8, DarkTheme)
	c.DrawLine(0, 4, 19, 4, LayerRoute)
	for x := 0; x < 20; x++ {
		assert.True(t, isSet(c, x, 4), "pixel (%d,4) should be set", x)
	}
}

func TestDrawLineVertical(t *testing.T) {
	c := NewCanvas(8, 16, DarkTheme)
	c.DrawLine(2, 0, 2, 15, LayerRoute)
	for y := 0; y < 16; y++ {
		assert.True(t, isSet(c, 2, y), "pixel (2,%d) should be set", y)
	}
}

func TestDrawLineDiagonal(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	c.DrawLine(0, 0, 10, 10, LayerRoute)
	// Endpoints must be set.
	assert.True(t, isSet(c, 0, 0))
	assert.True(t, isSet(c, 10, 10))
	// Some middle pixel along the diagonal must be set.
	assert.True(t, isSet(c, 5, 5))
}

func TestDrawLineReversed(t *testing.T) {
	c := NewCanvas(10, 10, DarkTheme)
	c.DrawLine(5, 5, 0, 0, LayerRoute)
	assert.True(t, isSet(c, 0, 0))
	assert.True(t, isSet(c, 5, 5))
}

func TestDrawLineSinglePoint(t *testing.T) {
	c := NewCanvas(4, 4, DarkTheme)
	c.DrawLine(1, 1, 1, 1, LayerRoute)
	assert.True(t, isSet(c, 1, 1))
	assert.Equal(t, 1, countSetPixels(c))
}

func TestDrawThickLine(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	c.DrawThickLine(0, 5, 19, 5, 3, LayerRoute)
	// At least 3 pixel rows should be set across the breadth of the line.
	rows := map[int]bool{}
	for y := 0; y < 20; y++ {
		if isSet(c, 10, y) {
			rows[y] = true
		}
	}
	assert.GreaterOrEqual(t, len(rows), 3)
}

func TestDrawThickLineThicknessOne(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	c.DrawThickLine(0, 5, 19, 5, 1, LayerRoute)
	assert.True(t, isSet(c, 10, 5))
	assert.False(t, isSet(c, 10, 4))
	assert.False(t, isSet(c, 10, 6))
}

func TestDrawPolyline(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	pts := []Point{{0, 0}, {10, 0}, {10, 10}}
	c.DrawPolyline(pts, 1, LayerRoute)
	assert.True(t, isSet(c, 0, 0))
	assert.True(t, isSet(c, 10, 0))
	assert.True(t, isSet(c, 10, 10))
	assert.True(t, isSet(c, 5, 0))
	assert.True(t, isSet(c, 10, 5))
}

func TestDrawPolylineEmpty(t *testing.T) {
	c := NewCanvas(20, 20, DarkTheme)
	c.DrawPolyline(nil, 1, LayerRoute)
	c.DrawPolyline([]Point{{0, 0}}, 1, LayerRoute)
	assert.Equal(t, 0, countSetPixels(c))
}
