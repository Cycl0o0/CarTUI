// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/i18n"
	"github.com/cycl0o0/cartui/internal/render"
)

// measureModel records the user's measurement polyline. Distances are
// displayed in metres / kilometres using the haversine formula on the
// great-circle path through every recorded point.
type measureModel struct {
	points []geo.LatLng
}

// addPoint stores the given coordinate at the end of the polyline.
func (m *measureModel) addPoint(p geo.LatLng) {
	m.points = append(m.points, p)
}

// undoPoint pops the last point.
func (m *measureModel) undoPoint() {
	if len(m.points) == 0 {
		return
	}
	m.points = m.points[:len(m.points)-1]
}

// reset clears every recorded point.
func (m *measureModel) reset() { m.points = nil }

// totalMeters returns the cumulated great-circle distance.
func (m measureModel) totalMeters() float64 { return geo.PathLength(m.points) }

// View renders the measurement HUD.
func (m measureModel) View(width int, t i18n.Strings) string {
	var sb strings.Builder
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD580")).Render("📏 " + t.Distance)
	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Points: %d\n", len(m.points)))
	if len(m.points) >= 2 {
		dist := m.totalMeters()
		if dist >= 1000 {
			sb.WriteString(fmt.Sprintf("%s totale: %.2f km\n", t.Distance, dist/1000))
		} else {
			sb.WriteString(fmt.Sprintf("%s totale: %.0f m\n", t.Distance, dist))
		}
	}
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render(
		"Espace : ajouter le point central · u : annuler · c : effacer · Esc : quitter",
	))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width).
		Render(sb.String())
}

// drawMeasurePolyline renders the active measurement on the canvas. The
// polyline uses the route layer so it lands above the map but below
// markers.
func drawMeasurePolyline(c *render.Canvas, v Viewport, points []geo.LatLng) {
	if len(points) < 2 {
		// A lone point is still drawn as a marker.
		for _, p := range points {
			cx, cy := v.LatLngToCell(p)
			if cx >= 0 {
				c.PutGlyph(cx, cy, '•', render.LayerMarker)
			}
		}
		return
	}
	pts := projectPoints(v, points)
	c.DrawPolyline(pts, 1, render.LayerRoute)
	for _, p := range pts {
		c.Set(p.X, p.Y, render.LayerMarker)
	}
}

// handleMeasureKey routes keys received while in measurement mode.
func (a *App) handleMeasureKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = ModeNormal
		return a, nil
	case " ":
		a.measure.addPoint(a.viewport.Center)
		return a, nil
	case "u":
		a.measure.undoPoint()
		return a, nil
	case "c":
		a.measure.reset()
		return a, nil
	}
	return a, nil
}
