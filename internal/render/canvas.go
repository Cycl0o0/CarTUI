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
