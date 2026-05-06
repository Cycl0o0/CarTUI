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

// fetchMapLayers schedules a fetch from the configured [providers.MapSource].
// PMTiles is range-fetched (no minimum-zoom cliff) while Overpass is
// gated below z11 to avoid continent-sized queries that always time out.
func fetchMapLayers(ctx context.Context, src providers.MapSource, b geo.BBox, zoom int, isPMTiles bool) tea.Cmd {
	if src == nil {
		return func() tea.Msg {
			return featuresLoadedMsg{collection: data.FeatureCollection{}}
		}
	}
	if !isPMTiles && zoom < 11 {
		return func() tea.Msg {
			return featuresLoadedMsg{collection: data.FeatureCollection{}}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		fc, err := src.FetchMapLayers(ctx, b, zoom)
		return featuresLoadedMsg{collection: fc, err: err}
	}
}
