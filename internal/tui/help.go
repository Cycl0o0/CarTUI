// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/i18n"
)

// helpView renders the full-screen help overlay listing every key binding.
func helpView(width int, t i18n.Strings, k Keymap) string {
	title := lipgloss.NewStyle().Bold(true).Render(t.HelpTitle)

	groups := k.FullHelp()
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")
	for i, group := range groups {
		for _, b := range group {
			help := b.Help()
			if help.Key == "" && help.Desc == "" {
				continue
			}
			line := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFA500")).
				Render(padRight(help.Key, 12)) +
				"  " + help.Desc
			sb.WriteString("  " + line + "\n")
		}
		if i < len(groups)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press ? or esc to close"))
	if width > 0 {
		return lipgloss.NewStyle().Padding(1, 2).Width(width).Render(sb.String())
	}
	return sb.String()
}

// padRight returns s right-padded with spaces to the given width.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
