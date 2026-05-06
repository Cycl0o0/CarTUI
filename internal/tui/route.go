// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/i18n"
	"github.com/cycl0o0/cartui/internal/providers"
)

// routeModel tracks the routing wizard state.
type routeModel struct {
	start   *geo.LatLng
	end     *geo.LatLng
	profile data.RouteProfile
	route   *data.Route
	loading bool
	err     error
}

func newRouteModel() routeModel {
	return routeModel{profile: data.ProfileDriving}
}

// View renders the route panel.
func (r routeModel) View(width int, t i18n.Strings) string {
	var sb strings.Builder
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00C8DC")).Render(t.Route)
	sb.WriteString(header)
	sb.WriteString("\n\n")

	sb.WriteString(formatPoint(t.StartPoint, r.start))
	sb.WriteString("\n")
	sb.WriteString(formatPoint(t.EndPoint, r.end))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-10s %s\n", t.Profile, r.profile))

	if r.loading {
		sb.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render(t.Loading))
	}
	if r.err != nil {
		sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6060")).Render(r.err.Error()))
	}
	if r.route != nil {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("%-10s %.1f km\n", t.Distance, r.route.Distance/1000))
		sb.WriteString(fmt.Sprintf("%-10s %s\n", t.Duration, formatDuration(r.route.Duration)))
		sb.WriteString("\nÉtapes:\n")
		for i, step := range r.route.Steps {
			if i >= 8 {
				sb.WriteString(fmt.Sprintf("  … (+%d)\n", len(r.route.Steps)-i))
				break
			}
			sb.WriteString(fmt.Sprintf("  %d. %s — %.0fm\n", i+1, step.Instruction, step.Distance))
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width).
		Render(sb.String())
}

func formatPoint(label string, p *geo.LatLng) string {
	if p == nil {
		return fmt.Sprintf("%-10s %s", label, lipgloss.NewStyle().Faint(true).Render("non défini — espace pour fixer le centre"))
	}
	return fmt.Sprintf("%-10s %s", label, p.String())
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) - h*60
	if h > 0 {
		return fmt.Sprintf("%dh %02dmin", h, m)
	}
	return fmt.Sprintf("%dmin", m)
}

// routeProfilesCycle returns the next profile in the order driving →
// cycling → walking → driving.
func routeProfilesCycle(p data.RouteProfile) data.RouteProfile {
	switch p {
	case data.ProfileDriving:
		return data.ProfileCycling
	case data.ProfileCycling:
		return data.ProfileWalking
	default:
		return data.ProfileDriving
	}
}

// computeRoute kicks off an OSRM call asynchronously.
func computeRoute(ctx context.Context, o *providers.OSRM, prof data.RouteProfile, start, end geo.LatLng) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		route, err := o.Route(ctx, prof, []geo.LatLng{start, end})
		return routeLoadedMsg{route: route, err: err}
	}
}

// exportGPX writes the current route to disk in GPX 1.1.
func exportGPX(r *data.Route, name, path string) error {
	if r == nil {
		return fmt.Errorf("no active route")
	}
	if path == "" {
		path = "cartui-route.gpx"
	}
	gpx := providers.ToGPX(*r, name)
	return os.WriteFile(path, []byte(gpx), 0o644)
}
