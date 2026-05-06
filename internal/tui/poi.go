// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/i18n"
	"github.com/cycl0o0/cartui/internal/providers"
)

// poiModel holds the state for the POI category selector.
type poiModel struct {
	categories list.Model
	selected   data.POICategory
	loading    bool
}

// poiCategoryItem is a list item rendered in the POI menu.
type poiCategoryItem struct {
	Cat   data.POICategory
	Label string
}

func (p poiCategoryItem) FilterValue() string { return p.Label }
func (p poiCategoryItem) Title() string       { return string(p.Cat.Glyph()) + " " + p.Label }
func (p poiCategoryItem) Description() string { return string(p.Cat) }

func newPOIModel(t i18n.Strings) poiModel {
	items := []list.Item{
		poiCategoryItem{Cat: data.POIRestaurant, Label: t.Restaurant},
		poiCategoryItem{Cat: data.POICafe, Label: t.Cafe},
		poiCategoryItem{Cat: data.POIHospital, Label: t.Hospital},
		poiCategoryItem{Cat: data.POIPharmacy, Label: t.Pharmacy},
		poiCategoryItem{Cat: data.POISchool, Label: t.School},
		poiCategoryItem{Cat: data.POITransport, Label: t.Transport},
		poiCategoryItem{Cat: data.POIAccommodation, Label: t.Accommodation},
		poiCategoryItem{Cat: data.POIShopping, Label: t.Shopping},
		poiCategoryItem{Cat: data.POICulture, Label: t.Culture},
		poiCategoryItem{Cat: data.POISport, Label: t.Sport},
		poiCategoryItem{Cat: data.POIPublicService, Label: t.PublicService},
	}
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 40, 12)
	l.Title = t.POI
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return poiModel{categories: l}
}

// Update routes messages to the inner list.
func (p *poiModel) Update(msg tea.Msg) tea.Cmd {
	var c tea.Cmd
	p.categories, c = p.categories.Update(msg)
	return c
}

// View renders the POI category overlay.
func (p poiModel) View(width int, t i18n.Strings) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6FB3")).Render(t.POI)
	if p.loading {
		header += "  " + lipgloss.NewStyle().Faint(true).Render(t.Loading)
	}
	p.categories.SetWidth(width)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width).
		Render(header + "\n\n" + p.categories.View())
}

// selectedCategory returns the highlighted category, if any.
func (p poiModel) selectedCategory() (data.POICategory, bool) {
	if it, ok := p.categories.SelectedItem().(poiCategoryItem); ok {
		return it.Cat, true
	}
	return data.POIOther, false
}

// fetchPOIs runs an Overpass POI fetch.
func fetchPOIs(ctx context.Context, o *providers.Overpass, b geo.BBox, cats []data.POICategory) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		fc, err := o.FetchPOIs(ctx, b, cats)
		return poisLoadedMsg{collection: fc, err: err}
	}
}

// formatPOIDetails renders a POI in the side panel.
func formatPOIDetails(p data.POI, t i18n.Strings) string {
	bullet := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6FB3"))
	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Render(p.Name))
	rows = append(rows, fmt.Sprintf("%s %s", bullet.Render("•"), p.Category.String()))
	if p.Address != "" {
		rows = append(rows, fmt.Sprintf("%s %s", bullet.Render("@"), p.Address))
	}
	if p.Phone != "" {
		rows = append(rows, fmt.Sprintf("%s %s", bullet.Render("☎"), p.Phone))
	}
	if p.Website != "" {
		rows = append(rows, fmt.Sprintf("%s %s", bullet.Render("⌘"), p.Website))
	}
	if p.Hours != "" {
		rows = append(rows, fmt.Sprintf("%s %s", bullet.Render("⌚"), data.FormatHours(p.Hours)))
	}
	rows = append(rows, "")
	rows = append(rows, lipgloss.NewStyle().Faint(true).Render(p.Position.String()))
	_ = t
	out := ""
	for _, r := range rows {
		out += r + "\n"
	}
	return out
}
