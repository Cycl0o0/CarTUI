// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
)

// DefaultOpenFreeMapTileJSON is the public TileJSON endpoint for the
// OpenFreeMap planet dataset. It points at the current daily build —
// the tile URL it advertises includes a date+counter prefix that
// changes every day or two.
const DefaultOpenFreeMapTileJSON = "https://tiles.openfreemap.org/planet"

// OpenFreeMapSource consumes vector tiles from the OpenFreeMap project
// (https://openfreemap.org). OpenFreeMap publishes the planet OSM data
// as OpenMapTiles-schema vector tiles served from Cloudflare R2 — free,
// no API key, no monthly quota, MIT-licensed.
//
// The default endpoint is `https://tiles.openfreemap.org/planet/{z}/{x}/{y}.pbf`.
// The schema is identical to the popular OpenMapTiles standard, so the
// same rewriter we'd use for MapTiler/Stadia/MapLibre demo tiles works
// here too.
type OpenFreeMapSource struct {
	client      *Client
	urlTemplate string // resolved tile URL with {z}/{x}/{y} placeholders
	tileJSONURL string // populated when initial resolution failed; retried on first fetch
	maxTiles    int
}

// NewOpenFreeMapSource builds a source.
//
//   - Empty input: resolves the OpenFreeMap planet TileJSON
//     (https://tiles.openfreemap.org/planet) and uses the dated tile
//     URL pattern it advertises.
//   - Input containing `{z}`/`{x}`/`{y}`: used verbatim — useful when
//     pointing at MapTiler/Stadia/your own OpenMapTiles host.
//   - Input pointing at a TileJSON document (no placeholders): fetched
//     and resolved.
//
// Resolution happens once at construction; failures are deferred to
// the first FetchMapLayers call so the TUI startup never blocks.
func NewOpenFreeMapSource(ctx context.Context, c *Client, urlOrTileJSON string, maxTiles int) *OpenFreeMapSource {
	if maxTiles <= 0 {
		maxTiles = 16
	}
	src := &OpenFreeMapSource{client: c, maxTiles: maxTiles}

	url := strings.TrimSpace(urlOrTileJSON)
	if url == "" {
		url = DefaultOpenFreeMapTileJSON
	}
	if strings.Contains(url, "{z}") {
		src.urlTemplate = url
		return src
	}
	// Treat as TileJSON URL — resolve synchronously.
	tmpl, err := resolveTileJSON(ctx, c, url)
	if err != nil {
		// Fall back to TileJSON-deferred mode: leave urlTemplate empty
		// and try again on the first fetch. The caller will see an
		// error if the resolution still fails at that point.
		src.tileJSONURL = url
		return src
	}
	src.urlTemplate = tmpl
	return src
}

// FetchMapLayers implements [MapSource].
func (o *OpenFreeMapSource) FetchMapLayers(ctx context.Context, bbox geo.BBox, zoom int) (data.FeatureCollection, error) {
	if o == nil {
		return data.FeatureCollection{}, errors.New("openfreemap: not configured")
	}

	// Lazy TileJSON resolution if construction-time fetch failed.
	if o.urlTemplate == "" && o.tileJSONURL != "" {
		tmpl, err := resolveTileJSON(ctx, o.client, o.tileJSONURL)
		if err != nil {
			return data.FeatureCollection{}, fmt.Errorf("openfreemap: tilejson: %w", err)
		}
		o.urlTemplate = tmpl
	}
	if o.urlTemplate == "" {
		return data.FeatureCollection{}, errors.New("openfreemap: no tile URL")
	}

	// OpenMapTiles publishes tiles up to z=14 in the standard schema.
	z := zoom
	if z > 14 {
		z = 14
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
	for ty := y0; ty <= y1 && count < o.maxTiles; ty++ {
		for tx := x0; tx <= x1 && count < o.maxTiles; tx++ {
			count++
			blob, err := o.fetchTile(ctx, z, tx, ty)
			if err != nil {
				continue
			}
			tileFC, err := DecodeMVT(blob, z, tx, ty)
			if err != nil {
				continue
			}
			for _, f := range tileFC.Features {
				if !omtFeatureRenderable(f, z) {
					continue
				}
				rewriteOMTFeature(&f)
				fc.Features = append(fc.Features, f)
			}
		}
	}
	return fc, nil
}

// fetchTile downloads + gunzips a single tile.
func (o *OpenFreeMapSource) fetchTile(ctx context.Context, z, x, y int) ([]byte, error) {
	u := o.urlTemplate
	u = strings.ReplaceAll(u, "{z}", fmt.Sprintf("%d", z))
	u = strings.ReplaceAll(u, "{x}", fmt.Sprintf("%d", x))
	u = strings.ReplaceAll(u, "{y}", fmt.Sprintf("%d", y))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := o.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer drain(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrTileNotFound
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return nil, fmt.Errorf("openfreemap: status %d: %s", resp.StatusCode, string(body))
	}
	const maxBody = 32 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("openfreemap: read body: %w", err)
	}
	if isGzipMagic(body) {
		return gunzip(body)
	}
	return body, nil
}

// omtFeatureRenderable applies the same zoom-aware tier rules we already
// use for Mapbox/Protomaps, but expressed in OpenMapTiles vocabulary.
//
// Reference schema:
//
//	https://openmaptiles.org/schema/
func omtFeatureRenderable(f data.Feature, zoom int) bool {
	layer := f.Tags["__layer"]
	class := f.Tags["class"]

	switch layer {
	case "housenumber":
		return false
	case "landuse":
		switch class {
		case "residential", "commercial", "industrial", "suburb",
			"neighbourhood":
			return false
		}
		if zoom < 13 {
			switch class {
			case "school", "hospital", "cemetery":
				return false
			}
		}
	case "building":
		if zoom < 14 {
			return false
		}
	case "transportation":
		if _, minor := minorRoadClasses[class]; minor && zoom < 14 {
			return false
		}
		if zoom < 11 {
			switch class {
			case "motorway", "trunk", "primary":
				return true
			}
			return false
		}
	case "transportation_name":
		// Road-label features mirror transportation; same gating.
		if _, minor := minorRoadClasses[class]; minor && zoom < 14 {
			return false
		}
	case "place":
		t := stringOr(class, f.Tags["type"])
		if zoom < 7 {
			switch t {
			case "country", "continent", "ocean":
				return true
			}
			return false
		}
		if zoom < 10 {
			switch t {
			case "country", "continent", "state", "city":
				return true
			}
			return false
		}
		if zoom < 12 {
			switch t {
			case "country", "state", "city", "town":
				return true
			}
			return false
		}
		if zoom < 13 {
			switch t {
			case "country", "state", "city", "town", "village":
				return true
			}
			return false
		}
	case "poi":
		if zoom < 12 {
			return false
		}
	case "transit":
		if zoom < 11 {
			return false
		}
	case "boundary":
		if zoom < 8 {
			return false
		}
	case "mountain_peak", "water_name", "aerodrome_label":
		if zoom < 11 {
			return false
		}
	case "landcover":
		if zoom < 9 {
			switch class {
			case "grass", "wood", "scrub":
				return false
			}
		}
	case "park":
		// Always interesting, no gating.
	case "water", "waterway":
		// Always interesting.
	case "aeroway":
		if zoom < 11 {
			return false
		}
	}
	return true
}

// rewriteOMTFeature normalises OpenMapTiles features into OSM-style tags
// so the existing renderer / heuristics work without special-casing.
func rewriteOMTFeature(f *data.Feature) {
	layer := f.Tags["__layer"]
	class := f.Tags["class"]

	switch layer {
	case "water":
		f.Tags["natural"] = "water"
	case "waterway":
		f.Tags["waterway"] = stringOr(class, "river")
	case "landuse":
		switch class {
		case "park", "cemetery":
			f.Tags["leisure"] = "park"
		case "wood":
			f.Tags["natural"] = "wood"
		case "agriculture", "farmland":
			f.Tags["landuse"] = "farmland"
		case "school", "hospital":
			f.Tags["amenity"] = class
		case "stadium", "pitch":
			f.Tags["leisure"] = "sports_centre"
		}
	case "landcover":
		switch class {
		case "wood", "forest":
			f.Tags["natural"] = "wood"
		case "grass":
			f.Tags["natural"] = "grassland"
		case "scrub":
			f.Tags["natural"] = "scrub"
		case "wetland":
			f.Tags["natural"] = "wetland"
		case "ice":
			f.Tags["natural"] = "glacier"
		}
	case "park":
		f.Tags["leisure"] = "park"
	case "building":
		f.Tags["building"] = "yes"
	case "boundary":
		f.Tags["boundary"] = "administrative"
	case "transportation":
		applyOMTRoadClass(f, class)
	case "transportation_name":
		applyOMTRoadClass(f, class)
	case "place":
		t := stringOr(f.Tags["type"], class)
		f.Tags["place"] = stringOr(t, "locality")
	case "poi":
		applyOMTPOIClass(f, class)
	case "aeroway":
		f.Tags["aeroway"] = stringOr(class, "aerodrome")
	case "mountain_peak":
		f.Tags["natural"] = "peak"
	case "water_name":
		f.Tags["natural"] = "water"
	case "aerodrome_label":
		f.Tags["aeroway"] = "aerodrome"
	}
}

// applyOMTRoadClass mirrors the Mapbox helper but for OpenMapTiles
// road class values.
func applyOMTRoadClass(f *data.Feature, class string) {
	switch class {
	case "motorway", "trunk":
		f.Tags["highway"] = "motorway"
	case "primary":
		f.Tags["highway"] = "primary"
	case "secondary", "tertiary":
		f.Tags["highway"] = "secondary"
	case "minor":
		f.Tags["highway"] = "residential"
	case "service":
		f.Tags["highway"] = "service"
	case "track":
		f.Tags["highway"] = "track"
	case "path", "footway", "pedestrian":
		f.Tags["highway"] = "pedestrian"
	case "rail":
		f.Tags["railway"] = "rail"
	}
}

// resolveTileJSON fetches a TileJSON document and returns its first
// `tiles` URL (which contains the `{z}/{x}/{y}` placeholders). The
// TileJSON spec guarantees at least one tile URL per source.
func resolveTileJSON(ctx context.Context, c *Client, url string) (string, error) {
	var doc struct {
		Tiles []string `json:"tiles"`
	}
	if err := c.GetJSON(ctx, url, &doc); err != nil {
		return "", err
	}
	if len(doc.Tiles) == 0 {
		return "", errors.New("tilejson: no tiles URL")
	}
	return doc.Tiles[0], nil
}

// applyOMTPOIClass routes OpenMapTiles POI classes to OSM tags so
// CategorizePOI returns the right category and Layer() returns LayerPOI.
func applyOMTPOIClass(f *data.Feature, class string) {
	switch class {
	case "restaurant", "fast_food", "cafe", "bar", "pub", "food_court":
		f.Tags["amenity"] = class
	case "hospital", "clinic", "doctors", "pharmacy":
		f.Tags["amenity"] = class
	case "school", "kindergarten", "library":
		f.Tags["amenity"] = class
	case "bus", "bus_stop", "bus_station":
		f.Tags["amenity"] = "bus_station"
	case "railway", "rail":
		f.Tags["railway"] = "station"
	case "lodging", "hotel":
		f.Tags["tourism"] = "hotel"
	case "shop":
		f.Tags["shop"] = "yes"
	case "art_gallery", "gallery":
		f.Tags["tourism"] = "gallery"
	case "museum":
		f.Tags["tourism"] = "museum"
	case "attraction", "viewpoint":
		f.Tags["tourism"] = class
	case "place_of_worship":
		f.Tags["amenity"] = "place_of_worship"
	case "stadium", "sports_centre", "fitness_centre":
		f.Tags["leisure"] = "sports_centre"
	case "park":
		f.Tags["leisure"] = "park"
	case "police", "fire_station", "post_office", "townhall":
		f.Tags["amenity"] = class
	default:
		// Generic fallback so OSMTags.Layer() returns LayerPOI.
		if f.Tags["amenity"] == "" && f.Tags["shop"] == "" && f.Tags["tourism"] == "" {
			f.Tags["amenity"] = "yes"
		}
	}
}
