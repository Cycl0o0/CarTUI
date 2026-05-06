// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/render"
)

// drawLabels overlays text labels on top of the canvas: street names,
// place names (cities, towns) and named POIs. A simple greedy collision
// tracker prevents text from stomping over previously-placed labels.
//
// Labels are drawn at [render.LayerLabel] which sits above water/green/
// roads but below POI markers, route polylines and traffic glyphs —
// keeping the visual hierarchy intuitive: "where am I" beats "what is
// this", which beats "what's nearby".
func drawLabels(c *render.Canvas, v Viewport, fc data.FeatureCollection) {
	if c == nil || len(fc.Features) == 0 {
		return
	}
	tracker := newLabelTracker(c.Width(), c.Height())

	// Pass 1: places (cities, towns, neighbourhoods). They are big
	// targets and worth keeping when the viewport is busy.
	for _, f := range fc.Features {
		if f.Geometry.Kind != data.GeometryPoint {
			continue
		}
		if f.Tags["__layer"] != "places" {
			continue
		}
		if f.Name == "" {
			continue
		}
		placePointLabel(c, v, tracker, f, 24)
	}

	// Pass 2: major roads. Only highway/primary at low zoom, broaden
	// at higher zooms.
	for _, f := range fc.Features {
		if f.Geometry.Kind != data.GeometryLineString {
			continue
		}
		if f.Name == "" {
			continue
		}
		if !isLabellableRoad(f.Tags, v.Zoom) {
			continue
		}
		placeRoadLabel(c, v, tracker, f)
	}

	// Pass 3: POIs. Limit to keep the viewport readable.
	const maxPOILabels = 24
	poiLabels := 0
	for _, f := range fc.Features {
		if f.Geometry.Kind != data.GeometryPoint {
			continue
		}
		if f.Tags["__layer"] != "pois" {
			continue
		}
		if f.Name == "" {
			continue
		}
		if poiLabels >= maxPOILabels {
			break
		}
		if placePOILabel(c, v, tracker, f) {
			poiLabels++
		}
	}
}

// isLabellableRoad reports whether a road is significant enough to
// label at the current zoom. Smaller roads only get labelled when the
// user has zoomed in enough that there is canvas room.
func isLabellableRoad(t data.OSMTags, zoom int) bool {
	switch t.Road() {
	case data.RoadMotorway, data.RoadPrimary:
		return true
	case data.RoadSecondary:
		return zoom >= 13
	case data.RoadResidential:
		return zoom >= 15
	}
	return false
}

// placeRoadLabel drops the road's name near the geometric centre of its
// visible polyline.
func placeRoadLabel(c *render.Canvas, v Viewport, t *labelTracker, f data.Feature) {
	pts := projectPoints(v, f.Geometry.Points)
	if len(pts) == 0 {
		return
	}
	mid := pts[len(pts)/2]
	cx := mid.X / render.SubCellWidth
	cy := mid.Y / render.SubCellHeight
	label := truncate(f.Name, 18)
	if t.tryReserve(cx, cy, label) {
		c.PutString(cx, cy, label, render.LayerLabel)
	}
}

// placePointLabel drops the feature's name centred on its point. Lat/Lng
// projection lands on a single cell — we centre the string horizontally
// around it.
func placePointLabel(c *render.Canvas, v Viewport, t *labelTracker, f data.Feature, maxLen int) {
	if len(f.Geometry.Points) == 0 {
		return
	}
	cx, cy := v.LatLngToCell(f.Geometry.Points[0])
	if cx < 0 {
		return
	}
	label := truncate(f.Name, maxLen)
	startX := cx - len(label)/2
	if startX < 0 {
		startX = 0
	}
	if t.tryReserve(startX, cy, label) {
		c.PutString(startX, cy, label, render.LayerLabel)
	}
}

// placePOILabel drops the POI name to the right of its glyph (or to the
// left when the right is full). Returns true on success.
func placePOILabel(c *render.Canvas, v Viewport, t *labelTracker, f data.Feature) bool {
	if len(f.Geometry.Points) == 0 {
		return false
	}
	cx, cy := v.LatLngToCell(f.Geometry.Points[0])
	if cx < 0 {
		return false
	}
	label := " " + truncate(f.Name, 14)
	if t.tryReserve(cx+1, cy, label) {
		c.PutString(cx+1, cy, label, render.LayerLabel)
		return true
	}
	leftLabel := truncate(f.Name, 14) + " "
	startX := cx - len(leftLabel)
	if startX >= 0 && t.tryReserve(startX, cy, leftLabel) {
		c.PutString(startX, cy, leftLabel, render.LayerLabel)
		return true
	}
	return false
}

// labelTracker is a tiny per-frame anti-collision buffer. Each cell is
// tracked once; reservation is greedy (first-come, first-served).
type labelTracker struct {
	width, height int
	cells         []bool
}

func newLabelTracker(width, height int) *labelTracker {
	return &labelTracker{
		width:  width,
		height: height,
		cells:  make([]bool, width*height),
	}
}

// tryReserve attempts to claim `len(label)` cells starting at (cx, cy).
// Returns true on success and marks those cells (plus a 1-cell margin
// on each side) as occupied. Returns false when any of the requested
// cells is already taken or out of bounds.
func (t *labelTracker) tryReserve(cx, cy int, label string) bool {
	n := lenRunes(label)
	if cy < 0 || cy >= t.height {
		return false
	}
	if cx < 0 || cx+n > t.width {
		return false
	}
	for x := cx; x < cx+n; x++ {
		if t.cells[cy*t.width+x] {
			return false
		}
	}
	for x := cx; x < cx+n; x++ {
		t.cells[cy*t.width+x] = true
	}
	return true
}

// lenRunes returns the rune count of s — the canvas measures cells in
// runes, not bytes.
func lenRunes(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
