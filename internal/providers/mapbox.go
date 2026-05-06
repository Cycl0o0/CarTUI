// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
)

// minorRoadClasses are road classes filtered at zoom < 14 to keep the
// canvas legible — Mapbox Streets v8 emits ~4× more road features than
// Protomaps and most of them are unnamed service/path/track entries.
var minorRoadClasses = map[string]struct{}{
	"service":      {},
	"service_link": {},
	"track":        {},
	"path":         {},
	"footway":      {},
	"cycleway":     {},
	"pedestrian":   {},
}

// MapboxSource fetches Mapbox Streets v8 vector tiles via the Mapbox
// Vector Tiles API. The free tier allows 50k requests per month — a
// generous budget given that one viewport refresh typically uses 4-9
// tiles which the in-memory tile pipeline already deduplicates.
//
// Requires a public access token from https://account.mapbox.com.
// Returns nil from [NewMapboxSource] when the token is empty so the
// caller can treat "no key configured" as "Mapbox disabled" without
// branching at every call site.
type MapboxSource struct {
	client   *Client
	urlBase  string // protocol+host, no trailing slash
	tileset  string // e.g. "mapbox.mapbox-streets-v8"
	token    string
	maxTiles int
}

// NewMapboxSource builds the client.
//
// urlBase defaults to `https://api.mapbox.com`, tileset to
// `mapbox.mapbox-streets-v8` (the official basemap). Pass other
// tileset IDs to use Mapbox Terrain, Streets-v6, or your own published
// tileset.
func NewMapboxSource(c *Client, urlBase, tileset, token string, maxTiles int) *MapboxSource {
	if token == "" {
		return nil
	}
	if urlBase == "" {
		urlBase = "https://api.mapbox.com"
	}
	if tileset == "" {
		tileset = "mapbox.mapbox-streets-v8"
	}
	if maxTiles <= 0 {
		maxTiles = 16
	}
	return &MapboxSource{
		client:   c,
		urlBase:  strings.TrimRight(urlBase, "/"),
		tileset:  tileset,
		token:    token,
		maxTiles: maxTiles,
	}
}

// FetchMapLayers implements [MapSource].
func (m *MapboxSource) FetchMapLayers(ctx context.Context, bbox geo.BBox, zoom int) (data.FeatureCollection, error) {
	if m == nil {
		return data.FeatureCollection{}, errors.New("mapbox: not configured")
	}

	// Mapbox Streets v8 publishes tiles up to z=15.
	z := zoom
	if z > 15 {
		z = 15
	}
	if z < 0 {
		z = 0
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
	for ty := y0; ty <= y1 && count < m.maxTiles; ty++ {
		for tx := x0; tx <= x1 && count < m.maxTiles; tx++ {
			count++
			blob, err := m.fetchTile(ctx, z, tx, ty)
			if err != nil {
				continue
			}
			tileFC, err := DecodeMVT(blob, z, tx, ty)
			if err != nil {
				continue
			}
			for _, f := range tileFC.Features {
				if !mapboxFeatureRenderable(f) {
					continue
				}
				// Zoom-aware road thinning: drop minor classes when
				// the viewport is wide enough that they'd just smear.
				if z < 14 && f.Tags["__layer"] == "road" {
					if _, minor := minorRoadClasses[f.Tags["class"]]; minor {
						continue
					}
				}
				rewriteMapboxFeature(&f)
				fc.Features = append(fc.Features, f)
			}
		}
	}
	return fc, nil
}

// fetchTile downloads a single MVT blob and gunzips it when needed.
// Mapbox tiles are typically served already gzipped at the HTTP layer
// (transparent decompression by Go's http client). We also detect
// inline gzip magic bytes for robustness.
func (m *MapboxSource) fetchTile(ctx context.Context, z, x, y int) ([]byte, error) {
	u := fmt.Sprintf("%s/v4/%s/%d/%d/%d.mvt?access_token=%s",
		m.urlBase, m.tileset, z, x, y, url.QueryEscape(m.token))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer drain(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrTileNotFound
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return nil, fmt.Errorf("mapbox: status %d: %s", resp.StatusCode, string(body))
	}
	const maxBody = 32 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("mapbox: read body: %w", err)
	}
	if isGzipMagic(body) {
		return gunzip(body)
	}
	return body, nil
}

// isGzipMagic reports whether b starts with the gzip magic bytes
// `0x1f 0x8b`.
func isGzipMagic(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

// mapboxFeatureRenderable filters the same dominating-fill features
// that we drop from Protomaps, but expressed in Mapbox Streets v8
// vocabulary.
func mapboxFeatureRenderable(f data.Feature) bool {
	switch f.Tags["__layer"] {
	case "landuse":
		switch f.Tags["class"] {
		case "residential", "commercial", "industrial":
			return false
		}
	case "structure":
		// Bridges, tunnels, retaining walls, dams. At TUI resolution
		// they cluster into noisy polygons that hide the road network
		// without adding signal. Drop them all.
		return false
	case "housenum_label":
		// House numbers are too dense to render usefully in a TUI.
		return false
	case "transit_stop_label":
		// Mapbox emits one feature per bus/tram stop — at z>=14 that's
		// hundreds per viewport. Render only rail stations (kept by
		// rewriteMapboxFeature → railway=station).
		switch f.Tags["class"] {
		case "rail", "rail_metro", "ferry":
			return true
		}
		return false
	}
	return true
}

// rewriteMapboxFeature normalises Mapbox Streets v8 features into the
// OSM-shaped tags CarTUI's renderer / heuristics already understand.
//
// Reference:
//
//	https://docs.mapbox.com/data/tilesets/reference/mapbox-streets-v8/
func rewriteMapboxFeature(f *data.Feature) {
	layer := f.Tags["__layer"]
	class := f.Tags["class"]

	switch layer {
	case "water":
		f.Tags["natural"] = "water"
	case "waterway":
		f.Tags["waterway"] = stringOr(class, "river")
	case "landuse":
		switch class {
		case "park", "cemetery", "grass", "scrub":
			f.Tags["leisure"] = "park"
		case "wood":
			f.Tags["natural"] = "wood"
		case "agriculture":
			f.Tags["landuse"] = "farmland"
		case "hospital", "school":
			f.Tags["amenity"] = class
		case "airport":
			f.Tags["aeroway"] = "aerodrome"
		}
	case "landuse_overlay":
		switch class {
		case "wetland", "wetland_noveg":
			f.Tags["natural"] = "wetland"
		case "national_park":
			f.Tags["leisure"] = "nature_reserve"
		}
	case "building":
		f.Tags["building"] = stringOr(f.Tags["type"], "yes")
	case "admin":
		f.Tags["boundary"] = "administrative"
	case "road":
		applyMapboxRoadClass(f, class)
	case "road_label":
		// Mapbox emits road centrelines a second time in this layer
		// alongside the `name` attribute. We render them as roads too
		// — the geometry overlaps the `road` layer feature but at the
		// same colour, which the layer-priority logic resolves into
		// a single visible line.
		applyMapboxRoadClass(f, class)
	case "place_label":
		// `type` is the place hierarchy in Mapbox Streets v8.
		t := f.Tags["type"]
		if t == "" {
			t = class
		}
		f.Tags["place"] = stringOr(t, "locality")
	case "poi_label":
		applyMapboxPOIClass(f, class)
	case "transit_stop_label":
		f.Tags["public_transport"] = "stop_position"
		switch class {
		case "rail", "rail_metro":
			f.Tags["railway"] = "station"
		default:
			f.Tags["amenity"] = "bus_station"
		}
	case "natural_label":
		switch class {
		case "peak", "volcano":
			f.Tags["natural"] = "peak"
		case "bay", "sea", "ocean":
			f.Tags["natural"] = "bay"
		}
	}
}

// applyMapboxRoadClass sets `highway=` based on the Mapbox Streets v8
// road class.
func applyMapboxRoadClass(f *data.Feature, class string) {
	switch class {
	case "motorway", "motorway_link", "trunk", "trunk_link":
		f.Tags["highway"] = "motorway"
	case "primary", "primary_link":
		f.Tags["highway"] = "primary"
	case "secondary", "secondary_link", "tertiary", "tertiary_link":
		f.Tags["highway"] = "secondary"
	case "street", "street_limited":
		f.Tags["highway"] = "residential"
	case "service", "service_link", "track":
		f.Tags["highway"] = "service"
	case "pedestrian", "path", "footway", "cycleway":
		f.Tags["highway"] = "pedestrian"
	}
}

// applyMapboxPOIClass routes Mapbox POI categories to OSM amenity /
// shop / tourism / leisure tags so [data.OSMTags.Layer] returns the
// expected layer and [data.CategorizePOI] picks the right glyph.
func applyMapboxPOIClass(f *data.Feature, class string) {
	switch class {
	case "food_and_drink":
		f.Tags["amenity"] = "restaurant"
	case "food_and_drink_stores":
		f.Tags["shop"] = "convenience"
	case "lodging":
		f.Tags["tourism"] = "hotel"
	case "education":
		f.Tags["amenity"] = "school"
	case "medical":
		f.Tags["amenity"] = "hospital"
	case "shopping":
		f.Tags["shop"] = "yes"
	case "arts_and_entertainment":
		f.Tags["tourism"] = "attraction"
	case "park_like":
		f.Tags["leisure"] = "park"
	case "general":
		// Mapbox uses "general" for catch-all POIs — keep them as
		// generic amenities so OSMTags.Layer() returns LayerPOI.
		f.Tags["amenity"] = "yes"
	case "religion":
		f.Tags["amenity"] = "place_of_worship"
	case "sports_and_leisure":
		f.Tags["leisure"] = "sports_centre"
	case "transportation":
		f.Tags["amenity"] = "bus_station"
	case "public_facilities":
		f.Tags["amenity"] = "townhall"
	case "industry":
		f.Tags["man_made"] = "works"
	default:
		if f.Tags["amenity"] == "" && f.Tags["shop"] == "" && f.Tags["tourism"] == "" {
			f.Tags["amenity"] = "yes"
		}
	}
}
