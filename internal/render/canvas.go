// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

// Plain returns the canvas as a plain string with no ANSI escapes. Mostly
// useful for tests and for piping the rendering into a non-TTY sink.
func (c *Canvas) Plain() string {
	var out []byte
	for y := 0; y < c.heightCells; y++ {
		for x := 0; x < c.widthCells; x++ {
			r := c.runeFor(c.cells[y*c.widthCells+x])
			out = append(out, []byte(string(r))...)
		}
		if y < c.heightCells-1 {
			out = append(out, '\n')
		}
	}
	return string(out)
}

// CellAt returns a snapshot of the cell at (cx, cy). Useful for unit tests.
func (c *Canvas) CellAt(cx, cy int) (dots uint8, layer Layer, glyph rune, color Color) {
	if cx < 0 || cx >= c.widthCells || cy < 0 || cy >= c.heightCells {
		return 0, LayerBackground, 0, Color{}
	}
	cl := c.cells[cy*c.widthCells+cx]
	return cl.dots, cl.layer, cl.glyph, cl.color
}

// PaintCell writes a custom dot pattern at (cellX, cellY) with the given
// layer. The colour is taken from the active theme. Unlike [Canvas.FillCell]
// it lets the caller pick the dot density — useful when a single 8-dot block
// is too visually heavy.
//
// The cell is only updated when layer ≥ existing layer; lower-priority
// requests are silently dropped.
func (c *Canvas) PaintCell(cellX, cellY int, dots uint8, layer Layer) {
	if cellX < 0 || cellX >= c.widthCells || cellY < 0 || cellY >= c.heightCells {
		return
	}
	cl := &c.cells[cellY*c.widthCells+cellX]
	if layer < cl.layer {
		return
	}
	cl.dots |= dots
	cl.layer = layer
	cl.color = c.theme.ColorFor(layer)
}

// PaintCellWithColor is like [Canvas.PaintCell] but uses an explicit colour
// instead of the layer's theme colour. Used by colour-gradient renderers
// (heatmaps, traffic flow) where the meaning is encoded in the colour
// itself.
func (c *Canvas) PaintCellWithColor(cellX, cellY int, dots uint8, layer Layer, color Color) {
	if cellX < 0 || cellX >= c.widthCells || cellY < 0 || cellY >= c.heightCells {
		return
	}
	cl := &c.cells[cellY*c.widthCells+cellX]
	if layer < cl.layer {
		return
	}
	cl.dots |= dots
	cl.layer = layer
	cl.color = color
}
