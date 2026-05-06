// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/providers"
	"github.com/cycl0o0/cartui/internal/render"
)

// renderMap composes the canvas for the current viewport.
func renderMap(v Viewport, theme render.Theme, ascii bool, fc data.FeatureCollection, route *data.Route, markers []geo.LatLng) string {
	w, h := v.PixelDims()
	c := render.NewCanvas(w, h, theme)
	c.SetASCII(ascii)
	drawFeatures(c, v, fc)
	drawRoute(c, v, route)
	drawMarkers(c, v, markers)
	return c.String()
}

// fetchMapLayers schedules an Overpass request for the current viewport.
func fetchMapLayers(ctx context.Context, o *providers.Overpass, b geo.BBox, zoom int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		fc, err := o.FetchMapLayers(ctx, b, zoom)
		return featuresLoadedMsg{collection: fc, err: err}
	}
}
