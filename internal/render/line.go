// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

// DrawLine rasterises a line from (x0, y0) to (x1, y1) using Bresenham's
// integer-only algorithm. Coordinates are in Braille pixel space.
//
// The line is drawn at one-pixel thickness — call [DrawThickLine] for wider
// strokes.
func (c *Canvas) DrawLine(x0, y0, x1, y1 int, layer Layer) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx - dy
	x, y := x0, y0
	for {
		c.Set(x, y, layer)
		if x == x1 && y == y1 {
			return
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// DrawThickLine traces the same path as [DrawLine] but with the requested
// thickness in pixels. A thickness of 1 is equivalent to DrawLine. Even
// thicknesses are biased one pixel right/down — this matches conventional
// raster behaviour and is harmless at TUI scales.
func (c *Canvas) DrawThickLine(x0, y0, x1, y1, thickness int, layer Layer) {
	if thickness <= 1 {
		c.DrawLine(x0, y0, x1, y1, layer)
		return
	}
	half := thickness / 2
	for ox := -half; ox < thickness-half; ox++ {
		for oy := -half; oy < thickness-half; oy++ {
			c.DrawLine(x0+ox, y0+oy, x1+ox, y1+oy, layer)
		}
	}
}

// DrawPolyline connects an ordered sequence of pixel coordinates with line
// segments. A polyline of fewer than two points is a no-op. The points slice
// is read but never mutated.
func (c *Canvas) DrawPolyline(points []Point, thickness int, layer Layer) {
	if len(points) < 2 {
		return
	}
	for i := 1; i < len(points); i++ {
		c.DrawThickLine(
			points[i-1].X, points[i-1].Y,
			points[i].X, points[i].Y,
			thickness, layer,
		)
	}
}

// Point is a 2D pixel coordinate used by line and polygon helpers.
type Point struct {
	X, Y int
}
