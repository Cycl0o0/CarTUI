// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"fmt"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
)

// MapSource is the abstraction used by the TUI to fetch map features
// for a given viewport. Both [Overpass] and [PMTilesSource] implement
// it, so the user can switch backends from configuration without
// touching the rendering pipeline.
type MapSource interface {
	FetchMapLayers(ctx context.Context, bbox geo.BBox, zoom int) (data.FeatureCollection, error)
}

// PMTilesSource adapts a [PMTiles] archive to the [MapSource] contract.
// Tiles intersecting the viewport are fetched, decoded and merged into a
// single [data.FeatureCollection]; the schema is mapped from the
// Protomaps basemap convention to CarTUI's [render.Layer] palette.
type PMTilesSource struct {
	archive  *PMTiles
	maxTiles int
}

// NewPMTilesSource wraps a PMTiles archive. maxTiles caps the number of
// tiles fetched per call (zero or negative → 64); it protects against a
// pathologically large viewport.
func NewPMTilesSource(archive *PMTiles, maxTiles int) *PMTilesSource {
	if maxTiles <= 0 {
		maxTiles = 64
	}
	return &PMTilesSource{archive: archive, maxTiles: maxTiles}
}

// FetchMapLayers implements [MapSource].
func (p *PMTilesSource) FetchMapLayers(ctx context.Context, bbox geo.BBox, zoom int) (data.FeatureCollection, error) {
	if p == nil || p.archive == nil {
		return data.FeatureCollection{}, fmt.Errorf("pmtiles: not configured")
	}
	hdr := p.archive.Header()
	z := zoom
	if z < int(hdr.MinZoom) {
		z = int(hdr.MinZoom)
	}
	if z > int(hdr.MaxZoom) {
		z = int(hdr.MaxZoom)
	}

	x0, y0 := geo.LatLngToTile(geo.LatLng{Lat: bbox.North, Lng: bbox.West}, z)
	x1, y1 := geo.LatLngToTile(geo.LatLng{Lat: bbox.South, Lng: bbox.East}, z)
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}

	count := 0
	var fc data.FeatureCollection
	for ty := y0; ty <= y1 && count < p.maxTiles; ty++ {
		for tx := x0; tx <= x1 && count < p.maxTiles; tx++ {
			count++
			blob, err := p.archive.Tile(ctx, z, tx, ty)
			if err != nil {
				continue // ocean / out-of-bounds tiles are silently skipped
			}
			tileFC, err := DecodeMVT(blob, z, tx, ty)
			if err != nil {
				continue
			}
			for _, f := range tileFC.Features {
				if !pmFeatureRenderable(f, z) {
					continue
				}
				rewritePMFeature(&f)
				fc.Features = append(fc.Features, f)
			}
		}
	}
	return fc, nil
}

// pmFeatureRenderable filters out Protomaps features that would
// visually dominate the canvas without conveying useful information
// at TUI resolution. The rules are zoom-aware: small features that
// are meaningful at z=15 become noise at z=8 because they overlap
// each other and the rest of the map.
//
// In particular the `earth` layer is a single polygon covering the
// entire tile — when painted on top of everything else it blanks the
// whole map. `landuse` polygons with continent-spanning categories
// (residential / commercial / industrial) have the same effect at low
// zoom and are dropped too.
func pmFeatureRenderable(f data.Feature, zoom int) bool {
	layer := f.Tags["__layer"]
	kind := f.Tags["kind"]

	switch layer {
	case "earth":
		return false
	case "places":
		if f.Geometry.Kind == data.GeometryPolygon {
			return false
		}
		// At low zoom, only render major place markers.
		if zoom < 7 {
			switch kind {
			case "country", "region", "state":
				return true
			}
			return false
		}
		if zoom < 10 {
			switch kind {
			case "country", "region", "state", "city":
				return true
			}
			return false
		}
		if zoom < 12 {
			switch kind {
			case "country", "region", "state", "city", "macrohood",
				"town":
				return true
			}
			return false
		}
		if zoom < 13 {
			switch kind {
			case "country", "region", "state", "city", "macrohood",
				"town", "village":
				return true
			}
			return false
		}
		// z >= 13: also neighbourhood, suburb, locality.
	case "landuse":
		switch kind {
		case "residential", "commercial", "industrial", "urban_area":
			return false
		}
		// Small institutional landuse is a dot at low zoom.
		if zoom < 13 {
			switch kind {
			case "school", "hospital", "cemetery":
				return false
			}
		}
	case "buildings":
		// Building footprints are unreadable at TUI zoom-out.
		if zoom < 14 {
			return false
		}
	case "roads":
		// Protomaps uses the `kind` attribute for road class.
		if zoom < 11 {
			switch kind {
			case "highway", "major_road":
				return true
			}
			return false
		}
		if zoom < 13 {
			switch kind {
			case "highway", "major_road", "medium_road":
				return true
			}
			return false
		}
		// z >= 13: include minor_road, path, rail.
	case "pois":
		if zoom < 12 {
			return false
		}
	case "transit":
		if zoom < 11 {
			return false
		}
	case "natural":
		// At low zoom we keep natural=water but drop tiny green blobs.
		if zoom < 11 {
			switch kind {
			case "grass", "scrub":
				return false
			}
		}
	case "boundaries":
		// Admin boundaries at low zoom = country borders only.
		if zoom < 8 {
			return false
		}
	}
	return true
}

// rewritePMFeature injects the OSM-shaped tags that CarTUI's layer logic
// expects, based on the Protomaps basemap layer/attribute conventions.
//
// Reference:
//
//	https://github.com/protomaps/basemaps/blob/main/styles/src/layers.ts
//
// The mapping is intentionally conservative — when in doubt we keep the
// raw tag and let the existing [data.OSMTags] heuristics decide.
func rewritePMFeature(f *data.Feature) {
	switch f.Tags["__layer"] {
	case "water":
		f.Tags["natural"] = "water"
	case "natural":
		// Protomaps uses kind=forest, kind=grass, etc. Map to OSM-style.
		switch f.Tags["kind"] {
		case "forest", "wood":
			f.Tags["natural"] = "wood"
		case "grass":
			f.Tags["natural"] = "grassland"
		}
	case "landuse":
		switch f.Tags["kind"] {
		case "park", "garden":
			f.Tags["leisure"] = "park"
		case "forest":
			f.Tags["landuse"] = "forest"
		case "grass", "meadow":
			f.Tags["landuse"] = "grass"
		case "residential":
			f.Tags["landuse"] = "residential"
		case "commercial", "industrial":
			f.Tags["landuse"] = f.Tags["kind"]
		}
	case "buildings":
		f.Tags["building"] = "yes"
	case "boundaries":
		f.Tags["boundary"] = "administrative"
	case "roads":
		switch f.Tags["kind"] {
		case "highway":
			f.Tags["highway"] = "motorway"
		case "major_road":
			f.Tags["highway"] = "primary"
		case "medium_road":
			f.Tags["highway"] = "secondary"
		case "minor_road":
			f.Tags["highway"] = "residential"
		case "path":
			f.Tags["highway"] = "pedestrian"
		case "rail":
			f.Tags["highway"] = "" // no road; routed through transit layer
			f.Tags["railway"] = "rail"
		}
	case "places":
		// Place names are rendered as labels — no specific layer.
		f.Tags["place"] = stringOr(f.Tags["kind"], "locality")
	case "pois":
		// Surface in the OSM amenity space so [data.OSMTags.Layer]
		// returns LayerPOI. The exact category is best-effort.
		switch f.Tags["kind"] {
		case "restaurant", "cafe", "bar", "fast_food":
			f.Tags["amenity"] = f.Tags["kind"]
		case "hospital", "pharmacy", "school", "library", "hotel":
			f.Tags["amenity"] = f.Tags["kind"]
		default:
			if f.Tags["amenity"] == "" {
				f.Tags["amenity"] = "yes"
			}
		}
	}
}

// stringOr returns s when non-empty, otherwise fallback.
func stringOr(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
