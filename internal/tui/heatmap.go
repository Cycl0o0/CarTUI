// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/render"
)

// drawHeatmap projects every Point feature in `pois` to canvas cells, sums
// the per-cell counts, and paints a density-coloured background only on
// cells that no map feature has already coloured.
//
// The colour gradient is the classic blue → cyan → green → yellow → red
// scale; sparse areas stay dim. The dot pattern density also tracks the
// count, so a single POI shows as a discreet single-dot mark while a
// dense cluster fills the cell.
func drawHeatmap(c *render.Canvas, v Viewport, pois data.FeatureCollection) {
	if c == nil || len(pois.Features) == 0 {
		return
	}

	w, h := c.Width(), c.Height()
	if w == 0 || h == 0 {
		return
	}
	counts := make([]int, w*h)
	maxCount := 0
	for _, f := range pois.Features {
		if f.Geometry.Kind != data.GeometryPoint || len(f.Geometry.Points) == 0 {
			continue
		}
		cx, cy := v.LatLngToCell(f.Geometry.Points[0])
		if cx < 0 || cy < 0 || cx >= w || cy >= h {
			continue
		}
		// Diffuse a small portion to the four orthogonal neighbours so
		// adjacent clusters merge visually.
		counts[cy*w+cx] += 4
		incr(counts, w, h, cx-1, cy, 1)
		incr(counts, w, h, cx+1, cy, 1)
		incr(counts, w, h, cx, cy-1, 1)
		incr(counts, w, h, cx, cy+1, 1)
		if v := counts[cy*w+cx]; v > maxCount {
			maxCount = v
		}
	}
	if maxCount == 0 {
		return
	}

	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			n := counts[cy*w+cx]
			if n == 0 {
				continue
			}
			dots, _, glyph, _ := c.CellAt(cx, cy)
			if dots != 0 || glyph != 0 {
				continue // already painted by map features
			}
			t := float64(n) / float64(maxCount)
			c.PaintCellWithColor(cx, cy, dotsForIntensity(t), render.LayerHeatmap, gradientHeatmap(t))
		}
	}
}

// incr safely increments counts[cy*w+cx] when (cx, cy) is in bounds.
func incr(counts []int, w, h, cx, cy, by int) {
	if cx < 0 || cy < 0 || cx >= w || cy >= h {
		return
	}
	counts[cy*w+cx] += by
}

// dotsForIntensity picks a Braille dot pattern whose dot count grows with
// t ∈ [0, 1].
func dotsForIntensity(t float64) uint8 {
	switch {
	case t > 0.85:
		return 0xFF
	case t > 0.6:
		return 0x77 // 6 dots, balanced top/bottom
	case t > 0.35:
		return 0x33 // 4 dots, two columns
	case t > 0.15:
		return 0x09 // 2 dots
	default:
		return 0x01 // single dot
	}
}

// gradientHeatmap maps t ∈ [0, 1] to a 5-stop blue → cyan → green → yellow
// → red gradient.
func gradientHeatmap(t float64) render.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	stops := [5]render.Color{
		{R: 40, G: 60, B: 200},  // blue
		{R: 50, G: 200, B: 220}, // cyan
		{R: 80, G: 220, B: 80},  // green
		{R: 255, G: 220, B: 0},  // yellow
		{R: 255, G: 60, B: 40},  // red
	}
	pos := t * 4
	idx := int(pos)
	if idx >= 4 {
		return stops[4]
	}
	frac := pos - float64(idx)
	return lerpColor(stops[idx], stops[idx+1], frac)
}

// lerpColor linearly interpolates between two RGB colours by frac ∈ [0, 1].
func lerpColor(a, b render.Color, frac float64) render.Color {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return render.Color{
		R: uint8(float64(a.R) + frac*(float64(b.R)-float64(a.R))),
		G: uint8(float64(a.G) + frac*(float64(b.G)-float64(a.G))),
		B: uint8(float64(a.B) + frac*(float64(b.B)-float64(a.B))),
	}
}
