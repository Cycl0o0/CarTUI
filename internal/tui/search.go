// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/i18n"
	"github.com/cycl0o0/cartui/internal/providers"
)

const debounceWindow = 300 * time.Millisecond

// searchModel is the view-state for the search overlay.
type searchModel struct {
	input    textinput.Model
	results  list.Model
	loading  bool
	debounceID int // monotonic id used to ignore stale debounced results
}

// searchItem wraps a Nominatim hit so it can be displayed inside a bubbles
// list. The list package wants a small `Title()` and `Description()`.
type searchItem struct {
	Result providers.SearchResult
}

func (s searchItem) FilterValue() string { return s.Result.DisplayName }
func (s searchItem) Title() string {
	parts := strings.SplitN(s.Result.DisplayName, ",", 2)
	return parts[0]
}
func (s searchItem) Description() string { return s.Result.DisplayName }

// newSearchModel builds the overlay components.
func newSearchModel(t i18n.Strings) searchModel {
	in := textinput.New()
	in.Placeholder = t.SearchHint
	in.Prompt = "/ "
	in.CharLimit = 256
	in.Width = 40

	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 40, 12)
	l.Title = t.Search
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	return searchModel{input: in, results: l}
}

// Focus activates the text input.
func (s *searchModel) Focus() tea.Cmd { return s.input.Focus() }

// Blur deactivates the text input and resets state for the next session.
func (s *searchModel) Blur() {
	s.input.Blur()
	s.input.SetValue("")
	s.results.SetItems(nil)
	s.loading = false
}

// View renders the overlay.
func (s searchModel) View(width int, t i18n.Strings) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5DADE2")).Render(t.Search)
	body := s.input.View()
	if s.loading {
		body += "  " + lipgloss.NewStyle().Faint(true).Render(t.Loading)
	}
	if len(s.results.Items()) > 0 {
		s.results.SetWidth(width)
		body += "\n\n" + s.results.View()
	} else if !s.loading && s.input.Value() != "" {
		body += "\n\n" + lipgloss.NewStyle().Faint(true).Render(t.NoResults)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(width).
		Render(header + "\n\n" + body)
}

// Update routes Bubble Tea messages to the input and list components.
func (s *searchModel) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var c tea.Cmd
	s.input, c = s.input.Update(msg)
	cmds = append(cmds, c)

	s.results, c = s.results.Update(msg)
	cmds = append(cmds, c)
	return tea.Batch(cmds...)
}

// debounce returns a Cmd that triggers a search after the debounce window
// elapses, as long as the user hasn't typed again in the meantime.
func (s *searchModel) debounce() tea.Cmd {
	s.debounceID++
	id := s.debounceID
	q := s.input.Value()
	return tea.Tick(debounceWindow, func(_ time.Time) tea.Msg {
		return debounceSearchMsg{query: q, id: id}
	})
}

// fetchResults runs a Nominatim search asynchronously.
func fetchResults(ctx context.Context, n *providers.Nominatim, query, lang string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		res, err := n.Search(ctx, query, providers.SearchOptions{Limit: 10, Language: lang})
		return searchResultsMsg{results: res, err: err}
	}
}

// applyResults populates the results list.
func (s *searchModel) applyResults(results []providers.SearchResult) {
	items := make([]list.Item, len(results))
	for i, r := range results {
		items[i] = searchItem{Result: r}
	}
	s.results.SetItems(items)
}

// selected returns the currently highlighted result, if any.
func (s searchModel) selected() (providers.SearchResult, bool) {
	if it, ok := s.results.SelectedItem().(searchItem); ok {
		return it.Result, true
	}
	return providers.SearchResult{}, false
}
