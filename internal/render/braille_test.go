// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCanvasRoundsToCellGranularity(t *testing.T) {
	c := NewCanvas(5, 5, DarkTheme)
	assert.Equal(t, 6, c.WidthPx(), "width rounded up to multiple of 2")
	assert.Equal(t, 8, c.HeightPx(), "height rounded up to multiple of 4")
	assert.Equal(t, 3, c.Width())
	assert.Equal(t, 2, c.Height())
}

func TestCanvasMinimumSize(t *testing.T) {
	c := NewCanvas(0, 0, DarkTheme)
	assert.Equal(t, 1, c.Width())
	assert.Equal(t, 1, c.Height())
}

func TestSetSinglePixel(t *testing.T) {
	c := NewCanvas(2, 4, DarkTheme)
	c.Set(0, 0, LayerRoute)
	dots, layer, _, color := c.CellAt(0, 0)
	assert.Equal(t, uint8(0x01), dots)
	assert.Equal(t, LayerRoute, layer)
	assert.Equal(t, DarkTheme.Route, color)
}

func TestSetAllEightDots(t *testing.T) {
	c := NewCanvas(2, 4, DarkTheme)
	for x := 0; x < 2; x++ {
		for y := 0; y < 4; y++ {
			c.Set(x, y, LayerRoadPrimary)
		}
	}
	dots, _, _, _ := c.CellAt(0, 0)
	assert.Equal(t, uint8(0xFF), dots, "all eight dots should be set")
}

func TestLayerPriorityWins(t *testing.T) {
	c := NewCanvas(2, 4, DarkTheme)
	// Background -> water -> route. Route wins.
	c.Set(0, 0, LayerWater)
	c.Set(0, 0, LayerRoute)
	_, _, _, color := c.CellAt(0, 0)
	assert.Equal(t, DarkTheme.Route, color)
}

func TestLowerLayerKeepsHigherColor(t *testing.T) {
	c := NewCanvas(2, 4, DarkTheme)
	c.Set(0, 0, LayerRoute)
	c.Set(0, 0, LayerWater) // lower priority
	_, _, _, color := c.CellAt(0, 0)
	assert.Equal(t, DarkTheme.Route, color, "higher layer should keep its colour")
}

func TestSetOutOfBounds(t *testing.T) {
	c := NewCanvas(4, 4, DarkTheme)
	c.Set(-1, 0, LayerRoute)
	c.Set(0, -1, LayerRoute)
	c.Set(99, 0, LayerRoute)
	c.Set(0, 99, LayerRoute)
	dots, _, _, _ := c.CellAt(0, 0)
	assert.Equal(t, uint8(0), dots)
}

func TestClear(t *testing.T) {
	c := NewCanvas(4, 4, DarkTheme)
	c.Set(0, 0, LayerRoute)
	c.Clear()
	dots, layer, _, _ := c.CellAt(0, 0)
	assert.Equal(t, uint8(0), dots)
	assert.Equal(t, LayerBackground, layer)
}

func TestPlainStringEmpty(t *testing.T) {
	// 4×8 pixels -> 2×2 cell grid, all spaces with one newline.
	c := NewCanvas(4, 8, DarkTheme)
	out := c.Plain()
	assert.Equal(t, "  \n  ", out)
}

func TestStringContainsBraille(t *testing.T) {
	c := NewCanvas(4, 4, DarkTheme)
	c.Set(0, 0, LayerRoute)
	out := c.Plain()
	require.NotEmpty(t, out)
	// 0x01 -> U+2801
	assert.True(t, strings.ContainsRune(out, '⠁'))
}

func TestPutGlyph(t *testing.T) {
	c := NewCanvas(4, 4, DarkTheme)
	c.PutGlyph(0, 0, '★', LayerMarker)
	out := c.Plain()
	assert.True(t, strings.ContainsRune(out, '★'))
}

func TestPutString(t *testing.T) {
	c := NewCanvas(20, 4, DarkTheme)
	c.PutString(0, 0, "Hi", LayerLabel)
	out := c.Plain()
	assert.True(t, strings.HasPrefix(out, "Hi"))
}

func TestASCIIFallback(t *testing.T) {
	c := NewCanvas(2, 4, DarkTheme)
	c.SetASCII(true)
	for x := 0; x < 2; x++ {
		for y := 0; y < 4; y++ {
			c.Set(x, y, LayerRoute)
		}
	}
	out := c.Plain()
	// 8 dots set -> '█'
	assert.True(t, strings.ContainsRune(out, '█'))
}

func TestSetTheme(t *testing.T) {
	c := NewCanvas(2, 4, DarkTheme)
	c.Set(0, 0, LayerRoute)
	c.SetTheme(LightTheme)
	_, _, _, color := c.CellAt(0, 0)
	assert.Equal(t, LightTheme.Route, color)
}

func TestColorHex(t *testing.T) {
	assert.Equal(t, "#000000", Color{0, 0, 0}.Hex())
	assert.Equal(t, "#FFFFFF", Color{255, 255, 255}.Hex())
	assert.Equal(t, "#FF8000", Color{255, 128, 0}.Hex())
	assert.True(t, Color{}.IsZero())
	assert.False(t, Color{1, 0, 0}.IsZero())
}

func TestThemeByName(t *testing.T) {
	assert.Equal(t, "dark", ThemeByName("dark").Name)
	assert.Equal(t, "light", ThemeByName("light").Name)
	assert.Equal(t, "mono", ThemeByName("mono").Name)
	assert.Equal(t, "dark", ThemeByName("zzz").Name)
}

func TestThemeColorFor(t *testing.T) {
	assert.Equal(t, DarkTheme.Water, DarkTheme.ColorFor(LayerWater))
	assert.Equal(t, DarkTheme.Route, DarkTheme.ColorFor(LayerRoute))
	assert.Equal(t, DarkTheme.Foreground, DarkTheme.ColorFor(Layer(99)))
}

func TestFillCell(t *testing.T) {
	c := NewCanvas(4, 4, DarkTheme)
	c.FillCell(0, 0, LayerWater)
	dots, layer, _, _ := c.CellAt(0, 0)
	assert.Equal(t, uint8(0xFF), dots)
	assert.Equal(t, LayerWater, layer)
}
