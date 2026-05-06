// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/i18n"
	"github.com/cycl0o0/cartui/internal/version"
)

// renderHeader builds the persistent top bar.
func (a *App) renderHeader() string {
	mode := modeLabel(a.mode, a.t)
	left := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFA500")).
		Render(a.t.AppName + " ")
	left += lipgloss.NewStyle().
		Background(lipgloss.Color("#222")).
		Foreground(lipgloss.Color("#EEE")).
		Padding(0, 1).
		Render(mode)

	right := ""
	if a.notification != "" {
		right = lipgloss.NewStyle().Foreground(lipgloss.Color("#5DE0A0")).Render(a.notification)
	}
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderFooter builds the bottom status line.
func (a *App) renderFooter() string {
	mpc := a.viewport.MetersPerCell()
	scale := fmt.Sprintf("%s ~%.0fm", a.t.StatusScale, mpc)
	if mpc > 1000 {
		scale = fmt.Sprintf("%s ~%.1fkm", a.t.StatusScale, mpc/1000)
	}
	left := fmt.Sprintf("%s %s | %s %d | %s",
		a.t.StatusCenter, a.viewport.Center.String(),
		a.t.StatusZoom, a.viewport.Zoom,
		scale,
	)
	right := compactKeyHelp(a.keys)
	pad := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1c1c1c")).
		Foreground(lipgloss.Color("#888")).
		Render(left + strings.Repeat(" ", pad) + right)
}

// renderSidebar builds the right-hand information panel.
func (a *App) renderSidebar(height, width int) string {
	if width == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("CarTUI"))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("v" + version.Version))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("%s: %s\n", a.t.StatusCenter, a.viewport.Center.String()))
	sb.WriteString(fmt.Sprintf("%s: %d\n", a.t.StatusZoom, a.viewport.Zoom))
	sb.WriteString("\n")

	if a.route != nil {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00C8DC")).Render(a.t.Route))
		sb.WriteString(fmt.Sprintf("\n%.1f km — %s\n\n", a.route.Distance/1000, formatDuration(a.route.Duration)))
	}

	if len(a.pois.Features) > 0 {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6FB3")).Render(a.t.POI))
		sb.WriteString("\n")
		limit := 8
		if len(a.pois.Features) < limit {
			limit = len(a.pois.Features)
		}
		for _, f := range a.pois.Features[:limit] {
			cat := data.CategorizePOI(f.Tags)
			name := f.Name
			if name == "" {
				name = "—"
			}
			sb.WriteString(fmt.Sprintf("%s %s\n", string(cat.Glyph()), truncate(name, width-4)))
		}
		if len(a.pois.Features) > limit {
			sb.WriteString(lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("(+%d)\n", len(a.pois.Features)-limit)))
		}
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#333")).
		Padding(0, 1).
		Width(width).
		Height(height)
	return style.Render(sb.String())
}

// modeLabel maps a [Mode] to its localised display string.
func modeLabel(m Mode, t i18n.Strings) string {
	switch m {
	case ModeSearch:
		return t.ModeSearch
	case ModeRoute:
		return t.ModeRoute
	case ModePOI:
		return t.ModePOI
	case ModeHelp:
		return t.ModeHelp
	default:
		return t.ModeNormal
	}
}

// compactKeyHelp builds a one-line help string for the footer.
func compactKeyHelp(k Keymap) string {
	short := k.ShortHelp()
	parts := make([]string, 0, len(short))
	for _, b := range short {
		h := b.Help()
		if h.Key == "" {
			continue
		}
		parts = append(parts, h.Key+":"+h.Desc)
	}
	return strings.Join(parts, " · ")
}

// overlay centres an overlay box on top of an existing rendering. The
// implementation is intentionally simple — Bubble Tea's terminal renderer
// already handles the underlying clearing.
func overlay(base, box string) string {
	// Centred via JoinVertical — the box is opaque so it covers the body.
	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		box,
		"",
		lipgloss.NewStyle().Faint(true).Render("─── overlay (esc to close) ───"),
		"",
		base,
	)
}

// keyMatch reports whether msg activates the binding.
func keyMatch(b key.Binding, msg tea.KeyMsg) bool { return key.Matches(msg, b) }

// truncate clips s to maxLen runes, appending an ellipsis when truncation
// occurred.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-1]) + "…"
}
