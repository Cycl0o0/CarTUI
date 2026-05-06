// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import "sort"

// FillPolygon rasterises and fills the polygon defined by `points` (a closed
// ring; the last vertex need not duplicate the first). The fill uses the
// even-odd rule via the classic scanline algorithm.
//
// For polygons that should also have a visible outline, call DrawPolyline on
// the same points with the desired thickness.
//
// A polygon of fewer than three points is a no-op.
func (c *Canvas) FillPolygon(points []Point, layer Layer) {
	if len(points) < 3 {
		return
	}

	minY, maxY := points[0].Y, points[0].Y
	for _, p := range points[1:] {
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	if minY < 0 {
		minY = 0
	}
	if maxY >= c.heightPx {
		maxY = c.heightPx - 1
	}

	// Per-scanline crossings buffer, reused across rows.
	xs := make([]int, 0, len(points))
	for y := minY; y <= maxY; y++ {
		xs = xs[:0]
		for i := 0; i < len(points); i++ {
			a := points[i]
			b := points[(i+1)%len(points)]
			if a.Y == b.Y {
				continue // ignore horizontal edges
			}
			yMin, yMax := a.Y, b.Y
			if yMin > yMax {
				yMin, yMax = yMax, yMin
			}
			// Standard rule: include the lower endpoint, exclude the
			// upper one. Avoids double-counting at vertices.
			if y < yMin || y >= yMax {
				continue
			}
			x := a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
			xs = append(xs, x)
		}
		if len(xs) < 2 {
			continue
		}
		sort.Ints(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			x1 := xs[i]
			x2 := xs[i+1]
			if x2 < 0 || x1 >= c.widthPx {
				continue
			}
			if x1 < 0 {
				x1 = 0
			}
			if x2 >= c.widthPx {
				x2 = c.widthPx - 1
			}
			for x := x1; x <= x2; x++ {
				c.Set(x, y, layer)
			}
		}
	}
}

// FillRect rasterises a filled rectangle with corners (x0,y0)–(x1,y1) inclusive.
// Used for opaque polygon fills where a 4-vertex polygon is overkill.
func (c *Canvas) FillRect(x0, y0, x1, y1 int, layer Layer) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c.Set(x, y, layer)
		}
	}
}
