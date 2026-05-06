// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
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

// renderConfig collects the optional layers requested by the App. Plain
// fields keep the call signature short and reordering-safe.
type renderConfig struct {
	Heatmap   bool
	Incidents []providers.Incident
}

// renderMap composes the canvas for the current viewport.
func renderMap(v Viewport, theme render.Theme, ascii bool, fc data.FeatureCollection, pois data.FeatureCollection, route *data.Route, markers []geo.LatLng, measurePoints []geo.LatLng, cfg renderConfig) string {
	w, h := v.PixelDims()
	c := render.NewCanvas(w, h, theme)
	c.SetASCII(ascii)
	drawFeatures(c, v, fc)
	drawFeatures(c, v, pois)
	drawRoute(c, v, route)
	if len(cfg.Incidents) > 0 {
		drawIncidents(c, v, cfg.Incidents)
	}
	drawMeasurePolyline(c, v, measurePoints)
	drawMarkers(c, v, markers)
	if cfg.Heatmap {
		drawHeatmap(c, v, pois)
	}
	return c.String()
}

// fetchMapLayers schedules an Overpass request for the current viewport.
//
// Below zoom 11 the bbox is too large to query Overpass synchronously
// without rate-limiting both us and the public endpoint, so we return an
// empty result. The bbox at z11 over a temperate latitude is roughly
// 40×30 km — already at the edge of what Overpass treats as "small".
func fetchMapLayers(ctx context.Context, o *providers.Overpass, b geo.BBox, zoom int) tea.Cmd {
	if zoom < 11 {
		return func() tea.Msg {
			return featuresLoadedMsg{collection: data.FeatureCollection{}}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		fc, err := o.FetchMapLayers(ctx, b, zoom)
		return featuresLoadedMsg{collection: fc, err: err}
	}
}
