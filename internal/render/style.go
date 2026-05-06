// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package render contains the Braille-based rasteriser used by CarTUI to
// draw maps inside a terminal grid.
//
// The canvas exposes a sub-cell coordinate system: each terminal cell holds a
// 2×4 Braille bitmap (eight dots, U+2800 .. U+28FF), letting the renderer
// approximate continuous geometry at four times the vertical and twice the
// horizontal resolution of plain ASCII.
package render

// Layer identifies the semantic category of a draw operation. It is consumed
// by [Canvas] when several layers fight for the same Braille cell — only the
// foreground colour of the highest-priority (largest value) layer survives.
type Layer uint8

// Layer constants ordered from background to foreground. Higher values win
// when multiple layers paint the same terminal cell.
const (
	LayerBackground Layer = iota
	LayerHeatmap
	LayerWater
	LayerGreen
	LayerBuilding
	LayerBoundary
	LayerRoadResidential
	LayerRoadSecondary
	LayerRoadPrimary
	LayerRoadMotorway
	LayerLabel
	LayerPOI
	LayerTraffic
	LayerRoute
	LayerMarker
)

// Color is a 24-bit RGB triple. The alpha channel is implicit: an "absent"
// colour is the zero value and is treated as transparent by the canvas.
type Color struct {
	R, G, B uint8
}

// Hex returns the colour in `#RRGGBB` form, ready for Lipgloss styles.
func (c Color) Hex() string {
	const hex = "0123456789ABCDEF"
	out := []byte("#000000")
	out[1] = hex[c.R>>4]
	out[2] = hex[c.R&0x0F]
	out[3] = hex[c.G>>4]
	out[4] = hex[c.G&0x0F]
	out[5] = hex[c.B>>4]
	out[6] = hex[c.B&0x0F]
	return string(out)
}

// IsZero reports whether the colour is the unset zero value.
func (c Color) IsZero() bool { return c == Color{} }

// Theme is the palette used by the renderer. Themes are stateless and safe to
// share across goroutines.
type Theme struct {
	Name string

	Background Color
	Foreground Color

	Water    Color
	Green    Color
	Building Color
	Boundary Color

	RoadMotorway    Color
	RoadPrimary     Color
	RoadSecondary   Color
	RoadResidential Color

	POI     Color
	Route   Color
	Marker  Color
	Label   Color
	Heatmap Color // base hue; rendered code applies a density-derived gradient
	Traffic Color // base hue; rendered code applies a severity gradient
}

// ColorFor returns the dominant colour for the given layer.
func (t Theme) ColorFor(l Layer) Color {
	switch l {
	case LayerWater:
		return t.Water
	case LayerGreen:
		return t.Green
	case LayerBuilding:
		return t.Building
	case LayerBoundary:
		return t.Boundary
	case LayerRoadMotorway:
		return t.RoadMotorway
	case LayerRoadPrimary:
		return t.RoadPrimary
	case LayerRoadSecondary:
		return t.RoadSecondary
	case LayerRoadResidential:
		return t.RoadResidential
	case LayerPOI:
		return t.POI
	case LayerRoute:
		return t.Route
	case LayerMarker:
		return t.Marker
	case LayerLabel:
		return t.Label
	case LayerHeatmap:
		return t.Heatmap
	case LayerTraffic:
		return t.Traffic
	case LayerBackground:
		return t.Background
	default:
		return t.Foreground
	}
}

// DarkTheme is the default high-contrast palette, tuned for dark terminals.
var DarkTheme = Theme{
	Name:            "dark",
	Background:      Color{12, 15, 20},
	Foreground:      Color{220, 220, 220},
	Water:           Color{50, 110, 180},
	Green:           Color{74, 145, 80},
	Building:        Color{120, 120, 120},
	Boundary:        Color{160, 160, 160},
	RoadMotorway:    Color{255, 140, 0},
	RoadPrimary:     Color{255, 200, 0},
	RoadSecondary:   Color{220, 220, 220},
	RoadResidential: Color{170, 170, 170},
	POI:             Color{255, 100, 200},
	Route:           Color{0, 200, 220},
	Marker:          Color{255, 80, 80},
	Label:           Color{240, 240, 240},
	Heatmap:         Color{255, 120, 0},
	Traffic:         Color{255, 60, 60},
}

// LightTheme inverts the brightness for bright terminal backgrounds.
var LightTheme = Theme{
	Name:            "light",
	Background:      Color{248, 248, 245},
	Foreground:      Color{40, 40, 40},
	Water:           Color{120, 170, 220},
	Green:           Color{170, 210, 160},
	Building:        Color{200, 200, 200},
	Boundary:        Color{120, 120, 120},
	RoadMotorway:    Color{220, 100, 0},
	RoadPrimary:     Color{220, 170, 0},
	RoadSecondary:   Color{60, 60, 60},
	RoadResidential: Color{110, 110, 110},
	POI:             Color{200, 50, 150},
	Route:           Color{0, 150, 170},
	Marker:          Color{220, 30, 30},
	Label:           Color{20, 20, 20},
	Heatmap:         Color{220, 120, 0},
	Traffic:         Color{220, 30, 30},
}

// MonoTheme strips colour but keeps layer priorities. Useful with NO_COLOR or
// TERM=dumb.
var MonoTheme = Theme{
	Name:            "mono",
	Foreground:      Color{255, 255, 255},
	Water:           Color{200, 200, 200},
	Green:           Color{200, 200, 200},
	Building:        Color{200, 200, 200},
	Boundary:        Color{200, 200, 200},
	RoadMotorway:    Color{255, 255, 255},
	RoadPrimary:     Color{255, 255, 255},
	RoadSecondary:   Color{200, 200, 200},
	RoadResidential: Color{170, 170, 170},
	POI:             Color{255, 255, 255},
	Route:           Color{255, 255, 255},
	Marker:          Color{255, 255, 255},
	Label:           Color{255, 255, 255},
	Heatmap:         Color{200, 200, 200},
	Traffic:         Color{255, 255, 255},
}

// ThemeByName looks up a built-in theme by case-insensitive name. Falls back
// to [DarkTheme] when the name is unknown.
func ThemeByName(name string) Theme {
	switch name {
	case "light", "Light", "LIGHT":
		return LightTheme
	case "mono", "Mono", "MONO":
		return MonoTheme
	default:
		return DarkTheme
	}
}
