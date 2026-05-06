// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Sub-cell layout. Each terminal cell maps to a 2×4 dot grid; the bit values
// match the Unicode Braille pattern offsets relative to U+2800.
//
//	dot column / dot row -> bit offset
//	col 0, row 0 -> 0  (0x01)
//	col 0, row 1 -> 1  (0x02)
//	col 0, row 2 -> 2  (0x04)
//	col 1, row 0 -> 3  (0x08)
//	col 1, row 1 -> 4  (0x10)
//	col 1, row 2 -> 5  (0x20)
//	col 0, row 3 -> 6  (0x40)
//	col 1, row 3 -> 7  (0x80)
const (
	// SubCellWidth is the number of Braille dots per terminal cell column.
	SubCellWidth = 2
	// SubCellHeight is the number of Braille dots per terminal cell row.
	SubCellHeight = 4

	brailleBase rune = 0x2800
)

// brailleDotBits maps (col, row) to the bit offset of the corresponding dot
// in the U+2800 Braille pattern code point.
var brailleDotBits = [SubCellWidth][SubCellHeight]uint8{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

// cell holds the rasterised state of a single terminal cell.
type cell struct {
	dots  uint8 // bitmask of Braille dots (0 means empty -> rendered as space)
	color Color // fg colour at the highest layer painted so far
	layer Layer // priority of `color`
	glyph rune  // override: when non-zero, replace the Braille pattern (used for POI markers)
}

// Canvas is a sub-cell rasteriser sized in pixels. The terminal grid it
// produces is widthCells = ceil(widthPx / 2), heightCells = ceil(heightPx / 4).
//
// All Set/SetIf operations operate in pixel space (sub-cell). String() builds
// the final styled output.
type Canvas struct {
	widthPx  int
	heightPx int

	widthCells  int
	heightCells int

	cells []cell

	theme Theme
	ascii bool // when true, render with block characters instead of Braille
}

// NewCanvas allocates an empty canvas of the given pixel dimensions. The
// dimensions are rounded up to a multiple of the sub-cell granularity so
// callers can pass arbitrary pixel sizes.
func NewCanvas(widthPx, heightPx int, theme Theme) *Canvas {
	if widthPx < 1 {
		widthPx = 1
	}
	if heightPx < 1 {
		heightPx = 1
	}
	wc := (widthPx + SubCellWidth - 1) / SubCellWidth
	hc := (heightPx + SubCellHeight - 1) / SubCellHeight
	return &Canvas{
		widthPx:     wc * SubCellWidth,
		heightPx:    hc * SubCellHeight,
		widthCells:  wc,
		heightCells: hc,
		cells:       make([]cell, wc*hc),
		theme:       theme,
	}
}

// SetASCII toggles the ASCII fallback rendering mode (no Braille). Enable when
// the user passes --ascii, NO_COLOR is set with a hostile TERM, or the locale
// cannot encode U+2800.
func (c *Canvas) SetASCII(on bool) { c.ascii = on }

// SetTheme swaps the active theme. The dot layer information already painted
// on the canvas is preserved; only the colour mapping changes.
func (c *Canvas) SetTheme(t Theme) {
	c.theme = t
	for i := range c.cells {
		if c.cells[i].layer != LayerBackground {
			c.cells[i].color = t.ColorFor(c.cells[i].layer)
		}
	}
}

// Width returns the canvas width in terminal cells.
func (c *Canvas) Width() int { return c.widthCells }

// Height returns the canvas height in terminal cells.
func (c *Canvas) Height() int { return c.heightCells }

// WidthPx returns the canvas width in Braille pixels.
func (c *Canvas) WidthPx() int { return c.widthPx }

// HeightPx returns the canvas height in Braille pixels.
func (c *Canvas) HeightPx() int { return c.heightPx }

// Theme returns the active theme.
func (c *Canvas) Theme() Theme { return c.theme }

// Clear resets the canvas to the background colour.
func (c *Canvas) Clear() {
	z := cell{}
	for i := range c.cells {
		c.cells[i] = z
	}
}

// inBounds reports whether (px, py) is a valid pixel coordinate.
func (c *Canvas) inBounds(px, py int) bool {
	return px >= 0 && px < c.widthPx && py >= 0 && py < c.heightPx
}

// cellIndex returns the cells slice index for the cell that contains (px, py).
func (c *Canvas) cellIndex(px, py int) int {
	cx := px / SubCellWidth
	cy := py / SubCellHeight
	return cy*c.widthCells + cx
}

// Set turns on the Braille dot at pixel (px, py) and tags the underlying cell
// with the given layer. If the cell already carries a higher-priority layer
// the dot is added but the colour is not overridden.
func (c *Canvas) Set(px, py int, layer Layer) {
	if !c.inBounds(px, py) {
		return
	}
	idx := c.cellIndex(px, py)
	col := px % SubCellWidth
	row := py % SubCellHeight
	cl := &c.cells[idx]
	cl.dots |= brailleDotBits[col][row]
	if layer >= cl.layer {
		cl.layer = layer
		cl.color = c.theme.ColorFor(layer)
	}
}

// FillCell paints the entire 2×4 cell at (cellX, cellY) with the layer
// background. Used by polygon scanline fills which prefer cell granularity to
// keep colours coherent.
func (c *Canvas) FillCell(cellX, cellY int, layer Layer) {
	if cellX < 0 || cellX >= c.widthCells || cellY < 0 || cellY >= c.heightCells {
		return
	}
	cl := &c.cells[cellY*c.widthCells+cellX]
	if layer < cl.layer {
		return
	}
	cl.dots = 0xFF
	cl.layer = layer
	cl.color = c.theme.ColorFor(layer)
}

// PutGlyph stamps a single rune at the given cell, with the layer's colour.
// It overrides the Braille pattern but is itself overridden by later glyphs
// of equal or higher layer.
func (c *Canvas) PutGlyph(cellX, cellY int, glyph rune, layer Layer) {
	if cellX < 0 || cellX >= c.widthCells || cellY < 0 || cellY >= c.heightCells {
		return
	}
	cl := &c.cells[cellY*c.widthCells+cellX]
	if layer < cl.layer {
		return
	}
	cl.glyph = glyph
	cl.layer = layer
	cl.color = c.theme.ColorFor(layer)
}

// PutString writes a string of glyphs starting at the given cell, advancing
// horizontally. Characters past the right edge are clipped.
func (c *Canvas) PutString(cellX, cellY int, s string, layer Layer) {
	for i, r := range s {
		c.PutGlyph(cellX+i, cellY, r, layer)
	}
}

// String renders the canvas to a styled multi-line string. The output is
// suitable for printing directly to a terminal that supports ANSI styling.
//
// When the active theme has no colour set for a cell, the cell is emitted
// without a Lipgloss style — the surrounding terminal styling wins.
func (c *Canvas) String() string {
	var b strings.Builder
	b.Grow(c.widthCells * c.heightCells * 2)

	for y := 0; y < c.heightCells; y++ {
		for x := 0; x < c.widthCells; x++ {
			cl := c.cells[y*c.widthCells+x]
			r := c.runeFor(cl)
			if cl.color.IsZero() {
				b.WriteRune(r)
				continue
			}
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(cl.color.Hex())).
				Render(string(r)))
		}
		if y < c.heightCells-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// runeFor selects the glyph for a cell, accounting for explicit glyph overrides
// and the ASCII fallback mode.
func (c *Canvas) runeFor(cl cell) rune {
	if cl.glyph != 0 {
		return cl.glyph
	}
	if cl.dots == 0 {
		return ' '
	}
	if c.ascii {
		return asciiShade(cl.dots)
	}
	return brailleBase + rune(cl.dots)
}

// asciiShade picks a block-element rune approximating the dot density.
func asciiShade(dots uint8) rune {
	count := 0
	for d := dots; d != 0; d &= d - 1 {
		count++
	}
	switch {
	case count >= 7:
		return '█' // █
	case count >= 5:
		return '▓' // ▓
	case count >= 3:
		return '▒' // ▒
	default:
		return '░' // ░
	}
}
