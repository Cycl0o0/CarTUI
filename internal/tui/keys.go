// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tui implements the Bubble Tea models, views and update loop that
// power the CarTUI interactive terminal interface. Every interactive widget
// — map, search, routing, POI list, help — lives here.
package tui

import "github.com/charmbracelet/bubbles/key"

// Keymap centralises every key binding used by the application. Bindings are
// re-used across the help menu and the update loop to keep documentation and
// behaviour in sync.
type Keymap struct {
	// Movement.
	PanLeft  key.Binding
	PanDown  key.Binding
	PanUp    key.Binding
	PanRight key.Binding

	ZoomIn  key.Binding
	ZoomOut key.Binding

	Center key.Binding
	Reset  key.Binding

	// Modes.
	OpenSearch    key.Binding
	OpenRoute     key.Binding
	OpenPOI       key.Binding
	OpenLayers    key.Binding
	ToggleSidebar key.Binding
	Help          key.Binding
	Goto          key.Binding

	// Bookmarks.
	AddBookmark  key.Binding
	ListBookmark key.Binding

	// Generic.
	Confirm key.Binding
	Cancel  key.Binding
	Quit    key.Binding
}

// DefaultKeymap returns the user-visible default key bindings.
func DefaultKeymap() Keymap {
	return Keymap{
		PanLeft:       key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "pan left")),
		PanDown:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "pan down")),
		PanUp:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "pan up")),
		PanRight:      key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "pan right")),
		ZoomIn:        key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "zoom in")),
		ZoomOut:       key.NewBinding(key.WithKeys("-", "_"), key.WithHelp("-", "zoom out")),
		Center:        key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "center")),
		Reset:         key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "reset view")),
		OpenSearch:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		OpenRoute:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "route")),
		OpenPOI:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "POIs")),
		OpenLayers:    key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "layers")),
		ToggleSidebar: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "sidebar")),
		Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Goto:          key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		AddBookmark:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "add bookmark")),
		ListBookmark:  key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "bookmarks")),
		Confirm:       key.NewBinding(key.WithKeys("enter")),
		Cancel:        key.NewBinding(key.WithKeys("esc")),
		Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp returns the bindings to display in the compact one-line help bar.
func (k Keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.PanLeft, k.PanDown, k.PanUp, k.PanRight, k.ZoomIn, k.ZoomOut, k.OpenSearch, k.Help, k.Quit}
}

// FullHelp returns groups of bindings to display in the full-screen help.
func (k Keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.PanLeft, k.PanDown, k.PanUp, k.PanRight, k.ZoomIn, k.ZoomOut, k.Center, k.Reset},
		{k.OpenSearch, k.OpenRoute, k.OpenPOI, k.OpenLayers, k.AddBookmark, k.ListBookmark},
		{k.ToggleSidebar, k.Goto, k.Help, k.Quit},
	}
}
