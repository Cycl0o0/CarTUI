// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/providers"
	"github.com/cycl0o0/cartui/internal/render"
)

// trafficState owns runtime traffic data: incidents currently fetched and
// whether the overlay is enabled.
type trafficState struct {
	enabled   bool
	incidents []providers.Incident
	updatedAt time.Time
}

// trafficLoadedMsg lands when the TomTom incidents fetch completes.
type trafficLoadedMsg struct {
	incidents []providers.Incident
	err       error
}

// trafficTickMsg requests the next incidents poll.
type trafficTickMsg struct{}

// fetchIncidents schedules a background incidents fetch.
func fetchIncidents(ctx context.Context, t *providers.TomTom, b geo.BBox, lang string) tea.Cmd {
	if t == nil {
		return func() tea.Msg {
			return trafficLoadedMsg{err: fmt.Errorf("TomTom non configuré (api_key vide)")}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		inc, err := t.Incidents(ctx, b, lang)
		return trafficLoadedMsg{incidents: inc, err: err}
	}
}

// rescheduleTraffic arms the next refresh while the overlay is enabled.
// 60 seconds is a sensible cadence — TomTom updates flow data once per
// minute on the free tier.
func (a *App) rescheduleTraffic() tea.Cmd {
	if !a.traffic.enabled {
		return nil
	}
	return tea.Tick(60*time.Second, func(_ time.Time) tea.Msg { return trafficTickMsg{} })
}

// drawIncidents renders incidents on the canvas: polylines (work zones,
// closures) as thick coloured strokes, points as glyphs whose colour and
// shape track the severity.
func drawIncidents(c *render.Canvas, v Viewport, incidents []providers.Incident) {
	for _, in := range incidents {
		col := severityColor(in.Severity)
		if len(in.Geometry) >= 2 {
			pts := projectPoints(v, in.Geometry)
			drawColoredPolyline(c, pts, col)
		}
		cx, cy := v.LatLngToCell(in.Position)
		if cx < 0 {
			continue
		}
		c.PaintCellWithColor(cx, cy, 0xFF, render.LayerTraffic, col)
		c.PutGlyph(cx, cy, providers.IncidentGlyph(in.Severity), render.LayerTraffic)
	}
}

// drawColoredPolyline overlays a polyline on the canvas with an explicit
// colour at the traffic layer.
func drawColoredPolyline(c *render.Canvas, pts []render.Point, col render.Color) {
	if len(pts) < 2 {
		return
	}
	for i := 1; i < len(pts); i++ {
		drawColoredLine(c, pts[i-1].X, pts[i-1].Y, pts[i].X, pts[i].Y, col)
	}
}

// drawColoredLine is a Bresenham-style line painted via PaintCellWithColor
// per pixel. Slightly slower than Canvas.DrawLine but lets us control the
// colour explicitly.
func drawColoredLine(c *render.Canvas, x0, y0, x1, y1 int, col render.Color) {
	dx, dy := x1-x0, y1-y0
	if dx < 0 {
		dx = -dx
	}
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
		paintPixel(c, x, y, col)
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

// paintPixel paints a single Braille pixel at (px, py) with explicit colour
// and the traffic layer.
func paintPixel(c *render.Canvas, px, py int, col render.Color) {
	if px < 0 || py < 0 {
		return
	}
	cx := px / render.SubCellWidth
	cy := py / render.SubCellHeight
	colIdx := px % render.SubCellWidth
	rowIdx := py % render.SubCellHeight
	dot := brailleDotMask(colIdx, rowIdx)
	c.PaintCellWithColor(cx, cy, dot, render.LayerTraffic, col)
}

// brailleDotMask mirrors the bit layout of [render.brailleDotBits]
// without exporting it. (col, row) ∈ ([0,1], [0,3]).
func brailleDotMask(col, row int) uint8 {
	bits := [2][4]uint8{
		{0x01, 0x02, 0x04, 0x40},
		{0x08, 0x10, 0x20, 0x80},
	}
	return bits[col][row]
}

// severityColor maps an incident severity to an RGB colour.
func severityColor(s providers.IncidentSeverity) render.Color {
	switch s {
	case providers.SeverityClosure:
		return render.Color{R: 200, G: 0, B: 200} // magenta — full closure
	case providers.SeverityMajor:
		return render.Color{R: 230, G: 40, B: 40} // red — major delay
	case providers.SeverityModerate:
		return render.Color{R: 230, G: 150, B: 0} // orange — moderate
	case providers.SeverityMinor:
		return render.Color{R: 230, G: 220, B: 40} // yellow — minor
	}
	return render.Color{R: 200, G: 200, B: 200} // grey — unknown
}
