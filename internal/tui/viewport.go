// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"math"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/render"
)

// Viewport describes the camera: where the map is centred, at which zoom
// level, and how many terminal cells the canvas spans.
type Viewport struct {
	Center      geo.LatLng
	Zoom        int
	WidthCells  int
	HeightCells int
}

// PixelDims returns the canvas size in Braille pixels.
func (v Viewport) PixelDims() (w, h int) {
	return v.WidthCells * render.SubCellWidth, v.HeightCells * render.SubCellHeight
}

// BBox returns the geographic bounding box covered by the viewport.
func (v Viewport) BBox() geo.BBox {
	w, h := v.PixelDims()
	cx, cy := geo.LatLngToWorldPixel(v.Center, v.Zoom)
	nw := geo.WorldPixelToLatLng(cx-float64(w)/2, cy-float64(h)/2, v.Zoom)
	se := geo.WorldPixelToLatLng(cx+float64(w)/2, cy+float64(h)/2, v.Zoom)
	return geo.BBox{
		South: math.Min(nw.Lat, se.Lat),
		North: math.Max(nw.Lat, se.Lat),
		West:  math.Min(nw.Lng, se.Lng),
		East:  math.Max(nw.Lng, se.Lng),
	}
}

// Pan shifts the centre by (dxCells, dyCells) terminal cells. Positive values
// move the map content the same direction (e.g. dx=+1 reveals more to the
// right by panning the world to the left).
func (v *Viewport) Pan(dxCells, dyCells int) {
	if dxCells == 0 && dyCells == 0 {
		return
	}
	cx, cy := geo.LatLngToWorldPixel(v.Center, v.Zoom)
	cx += float64(dxCells * render.SubCellWidth)
	cy += float64(dyCells * render.SubCellHeight)
	v.Center = geo.WorldPixelToLatLng(cx, cy, v.Zoom)
}

// SetZoom adjusts the zoom level, clamped to the OSM range. The centre is
// preserved.
func (v *Viewport) SetZoom(z int) { v.Zoom = geo.ClampZoom(z) }

// LatLngToCell projects a geographic coordinate to canvas cell coordinates.
// Returns (-1, -1) when the projection lands outside the viewport — callers
// can use that as a "skip" sentinel.
func (v Viewport) LatLngToCell(p geo.LatLng) (int, int) {
	x, y := v.LatLngToPixel(p)
	if x < 0 || y < 0 {
		return -1, -1
	}
	return x / render.SubCellWidth, y / render.SubCellHeight
}

// LatLngToPixel projects a coordinate to the canvas pixel grid.
// Returns negative coordinates when the point lies above/left of the
// viewport; coordinates ≥ width/height when below/right of it.
func (v Viewport) LatLngToPixel(p geo.LatLng) (int, int) {
	w, h := v.PixelDims()
	cx, cy := geo.LatLngToWorldPixel(v.Center, v.Zoom)
	px, py := geo.LatLngToWorldPixel(p, v.Zoom)
	x := int(math.Round(px - cx + float64(w)/2))
	y := int(math.Round(py - cy + float64(h)/2))
	return x, y
}

// MetersPerCell returns the on-ground horizontal distance covered by a single
// terminal cell, used to draw the scale bar.
func (v Viewport) MetersPerCell() float64 {
	return geo.MetersPerPixel(v.Center.Lat, v.Zoom) * float64(render.SubCellWidth)
}

// drawFeatures rasterises a feature collection onto the canvas, picking the
// right colour layer per feature and switching between line/polygon
// rendering based on geometry kind.
func drawFeatures(c *render.Canvas, v Viewport, fc data.FeatureCollection) {
	// Two passes: areas first (water/green/buildings), then linear features
	// on top (roads/boundaries). This avoids fills overpainting roads.
	for _, f := range fc.Features {
		if f.Geometry.Kind != data.GeometryPolygon {
			continue
		}
		layer := f.Tags.Layer()
		pts := projectPoints(v, f.Geometry.Points)
		c.FillPolygon(pts, layer)
	}
	for _, f := range fc.Features {
		switch f.Geometry.Kind {
		case data.GeometryLineString:
			layer := f.Tags.Layer()
			th := strokeWidthFor(f.Tags.Road())
			pts := projectPoints(v, f.Geometry.Points)
			c.DrawPolyline(pts, th, layer)
		case data.GeometryPoint:
			if len(f.Geometry.Points) == 0 {
				continue
			}
			cx, cy := v.LatLngToCell(f.Geometry.Points[0])
			if cx < 0 {
				continue
			}
			cat := data.CategorizePOI(f.Tags)
			c.PutGlyph(cx, cy, cat.Glyph(), render.LayerPOI)
		}
	}
}

// drawRoute paints a polyline for the active route (if any) over everything
// else on the map.
func drawRoute(c *render.Canvas, v Viewport, route *data.Route) {
	if route == nil || len(route.Geometry) == 0 {
		return
	}
	pts := projectPoints(v, route.Geometry)
	c.DrawPolyline(pts, 2, render.LayerRoute)
}

// drawMarkers paints user-placed markers (search results, bookmarks).
func drawMarkers(c *render.Canvas, v Viewport, markers []geo.LatLng) {
	for _, m := range markers {
		cx, cy := v.LatLngToCell(m)
		if cx < 0 {
			continue
		}
		c.PutGlyph(cx, cy, '★', render.LayerMarker)
	}
}

func projectPoints(v Viewport, pts []geo.LatLng) []render.Point {
	out := make([]render.Point, len(pts))
	for i, p := range pts {
		x, y := v.LatLngToPixel(p)
		out[i] = render.Point{X: x, Y: y}
	}
	return out
}

func strokeWidthFor(c data.RoadClass) int {
	switch c {
	case data.RoadMotorway:
		return 2
	case data.RoadPrimary:
		return 2
	case data.RoadSecondary:
		return 1
	default:
		return 1
	}
}
