// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/i18n"
)

// bookmarkModel manages the bookmark-list overlay.
type bookmarkModel struct {
	list list.Model
}

// bookmarkItem wraps a stored bookmark for use by the bubbles list.
type bookmarkItem struct {
	Bookmark data.Bookmark
}

func (b bookmarkItem) FilterValue() string { return b.Bookmark.Name }
func (b bookmarkItem) Title() string {
	if b.Bookmark.Name != "" {
		return b.Bookmark.Name
	}
	return b.Bookmark.Position.String()
}
func (b bookmarkItem) Description() string {
	return b.Bookmark.CreatedAt.Format("2006-01-02 15:04") + " · " + b.Bookmark.Position.String()
}

func newBookmarkModel(t i18n.Strings) bookmarkModel {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 50, 16)
	l.Title = t.Bookmarks
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return bookmarkModel{list: l}
}

func (b *bookmarkModel) refresh(items []data.Bookmark) {
	mapped := make([]list.Item, len(items))
	for i, bm := range items {
		mapped[i] = bookmarkItem{Bookmark: bm}
	}
	b.list.SetItems(mapped)
}

func (b *bookmarkModel) Update(msg tea.Msg) tea.Cmd {
	var c tea.Cmd
	b.list, c = b.list.Update(msg)
	return c
}

func (b bookmarkModel) View(width int, t i18n.Strings) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5DE0A0")).Render(t.Bookmarks)
	if len(b.list.Items()) == 0 {
		body := lipgloss.NewStyle().Faint(true).Render("(aucun favori — appuyez sur f pour en ajouter)")
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(width).
			Render(header + "\n\n" + body + "\n\n" +
				lipgloss.NewStyle().Faint(true).Render("Esc : fermer"))
	}
	b.list.SetWidth(width)
	hint := lipgloss.NewStyle().Faint(true).Render("Enter : centrer · d : supprimer · Esc : fermer")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width).
		Render(header + "\n\n" + b.list.View() + "\n" + hint)
}

func (b bookmarkModel) selected() (data.Bookmark, bool) {
	if it, ok := b.list.SelectedItem().(bookmarkItem); ok {
		return it.Bookmark, true
	}
	return data.Bookmark{}, false
}

// handleBookmarkKey routes keys received while in the bookmarks overlay.
func (a *App) handleBookmarkKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.mode = ModeNormal
		return a, nil
	case "enter":
		if bm, ok := a.bookmarks.selected(); ok {
			a.viewport.Center = bm.Position
			a.markers = []geo.LatLng{bm.Position}
			a.mode = ModeNormal
			return a, a.refreshLayers()
		}
		return a, nil
	case "d":
		if bm, ok := a.bookmarks.selected(); ok && a.deps.Store != nil {
			_ = a.deps.Store.DeleteBookmark(bm.ID)
			items, _ := a.deps.Store.ListBookmarks()
			a.bookmarks.refresh(items)
			return a, a.notify(a.t.BookmarkRemoved)
		}
		return a, nil
	}
	return a, a.bookmarks.Update(msg)
}
