// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
)

// Overpass is a thin client around the Overpass QL HTTP endpoint. The
// embedded query builder constructs viewport-bounded queries fitting the
// CarTUI layer model (water/green/buildings/roads/POIs).
type Overpass struct {
	client  *Client
	baseURL string
}

// NewOverpass constructs a client. baseURL defaults to the public
// Overpass-API instance when empty.
func NewOverpass(c *Client, baseURL string) *Overpass {
	if baseURL == "" {
		baseURL = "https://overpass-api.de/api/interpreter"
	}
	return &Overpass{client: c, baseURL: baseURL}
}

// QueryFeatures runs an Overpass QL statement and returns the contained
// features in normalised CarTUI form.
//
// The provided query body must omit the leading `[out:json][bbox:...];` —
// it is added by [Overpass]. The bbox is also injected as a global filter so
// callers always operate on the viewport.
func (o *Overpass) QueryFeatures(ctx context.Context, bbox geo.BBox, body string, timeoutSec int) (data.FeatureCollection, error) {
	if timeoutSec <= 0 {
		timeoutSec = 25
	}
	preamble := fmt.Sprintf(
		"[out:json][timeout:%d][bbox:%f,%f,%f,%f];\n",
		timeoutSec, bbox.South, bbox.West, bbox.North, bbox.East,
	)
	full := preamble + strings.TrimSpace(body) + "\nout body geom;\n"

	var raw overpassResponse
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	form := "data=" + escapeForm(full)
	if err := o.client.RequestJSON(ctx, "POST", o.baseURL, headers, io.Reader(bytes.NewBufferString(form)), &raw); err != nil {
		return data.FeatureCollection{}, fmt.Errorf("overpass: %w", err)
	}
	return raw.toFeatures(), nil
}

// FetchMapLayers asks Overpass for everything CarTUI needs to render a
// viewport at the given zoom: roads, water, parks, optionally buildings, and
// POIs. Heavier feature classes are skipped at low zooms to keep responses
// small.
func (o *Overpass) FetchMapLayers(ctx context.Context, bbox geo.BBox, zoom int) (data.FeatureCollection, error) {
	q := buildLayersQuery(zoom)
	return o.QueryFeatures(ctx, bbox, q, 25)
}

// FetchPOIs fetches points of interest by category for a viewport.
func (o *Overpass) FetchPOIs(ctx context.Context, bbox geo.BBox, categories []data.POICategory) (data.FeatureCollection, error) {
	q := buildPOIQuery(categories)
	return o.QueryFeatures(ctx, bbox, q, 25)
}

// buildLayersQuery returns an Overpass QL fragment selecting the geometric
// features required to draw the map.
func buildLayersQuery(zoom int) string {
	parts := []string{
		`way["highway"~"^(motorway|trunk|primary|secondary|tertiary|residential|service|unclassified|living_street|pedestrian)(_link)?$"];`,
		`way["waterway"];`,
		`way["natural"~"^(water|coastline|wood|grassland)$"];`,
		`way["landuse"~"^(forest|grass|meadow|park)$"];`,
		`way["leisure"~"^(park|garden|nature_reserve)$"];`,
		`relation["natural"="water"];`,
	}
	if zoom >= 14 {
		parts = append(parts,
			`way["building"];`,
			`relation["building"];`,
			`way["boundary"="administrative"];`,
		)
	}
	return "(\n  " + strings.Join(parts, "\n  ") + "\n);"
}

// buildPOIQuery builds an Overpass node-filter for the requested categories.
// An empty slice is treated as "all categories".
func buildPOIQuery(cats []data.POICategory) string {
	tags := poiTagFilters(cats)
	statements := make([]string, 0, len(tags))
	for _, f := range tags {
		statements = append(statements, "node"+f+";")
		statements = append(statements, "way"+f+";")
	}
	if len(statements) == 0 {
		statements = append(statements, `node["amenity"];`)
	}
	return "(\n  " + strings.Join(statements, "\n  ") + "\n);"
}

func poiTagFilters(cats []data.POICategory) []string {
	if len(cats) == 0 {
		return []string{
			`["amenity"]`,
			`["shop"]`,
			`["tourism"]`,
			`["leisure"]`,
		}
	}
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		switch c {
		case data.POIRestaurant:
			out = append(out, `["amenity"~"^(restaurant|fast_food|food_court|bar|pub)$"]`)
		case data.POICafe:
			out = append(out, `["amenity"~"^(cafe|biergarten)$"]`)
		case data.POIHospital:
			out = append(out, `["amenity"~"^(hospital|clinic|doctors)$"]`)
		case data.POIPharmacy:
			out = append(out, `["amenity"="pharmacy"]`)
		case data.POISchool:
			out = append(out, `["amenity"~"^(school|kindergarten|university|college|library)$"]`)
		case data.POITransport:
			out = append(out, `["amenity"~"^(bus_station|ferry_terminal|taxi)$"]`,
				`["public_transport"]`, `["railway"="station"]`)
		case data.POIAccommodation:
			out = append(out, `["tourism"~"^(hotel|hostel|guest_house|motel|apartment|camp_site)$"]`)
		case data.POIShopping:
			out = append(out, `["shop"]`)
		case data.POICulture:
			out = append(out, `["tourism"~"^(museum|attraction|gallery|viewpoint)$"]`,
				`["amenity"~"^(theatre|cinema|arts_centre|museum)$"]`)
		case data.POISport:
			out = append(out, `["leisure"~"^(stadium|sports_centre|fitness_centre|swimming_pool|pitch)$"]`)
		case data.POIPublicService:
			out = append(out, `["amenity"~"^(townhall|courthouse|post_office|police|fire_station|embassy)$"]`)
		}
	}
	return out
}

// escapeForm URL-encodes a value for x-www-form-urlencoded bodies.
func escapeForm(s string) string {
	return urlValuesEscape(s)
}

// urlValuesEscape mirrors net/url.QueryEscape but avoids the import cycle
// with the test helpers that share the same name.
func urlValuesEscape(s string) string {
	const upperhex = "0123456789ABCDEF"
	count := 0
	for i := 0; i < len(s); i++ {
		if !shouldEscape(s[i]) {
			continue
		}
		count++
	}
	if count == 0 {
		return s
	}
	out := make([]byte, 0, len(s)+2*count)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			out = append(out, '+')
		case shouldEscape(c):
			out = append(out, '%', upperhex[c>>4], upperhex[c&15])
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

func shouldEscape(c byte) bool {
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' {
		return false
	}
	switch c {
	case '-', '_', '.', '~':
		return false
	case ' ':
		return true
	}
	return true
}

// overpassResponse mirrors the JSON returned by `out body geom;`.
type overpassResponse struct {
	Elements []overpassElement `json:"elements"`
}

// overpassElement covers nodes, ways and relations. Only the fields we use
// are decoded; the rest is ignored.
type overpassElement struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Lat      float64           `json:"lat,omitempty"`
	Lon      float64           `json:"lon,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	Geometry []overpassPoint   `json:"geometry,omitempty"`
	Members  []overpassMember  `json:"members,omitempty"`
}

type overpassPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type overpassMember struct {
	Type     string          `json:"type"`
	Ref      int64           `json:"ref"`
	Role     string          `json:"role"`
	Geometry []overpassPoint `json:"geometry,omitempty"`
}

// toFeatures normalises an Overpass payload into a [data.FeatureCollection].
func (r overpassResponse) toFeatures() data.FeatureCollection {
	out := data.FeatureCollection{Features: make([]data.Feature, 0, len(r.Elements))}
	for _, el := range r.Elements {
		f := data.Feature{
			ID:   fmt.Sprintf("%s/%d", el.Type, el.ID),
			Tags: data.OSMTags(el.Tags),
			Name: el.Tags["name"],
		}
		switch el.Type {
		case "node":
			f.Geometry = data.Geometry{
				Kind:   data.GeometryPoint,
				Points: []geo.LatLng{{Lat: el.Lat, Lng: el.Lon}},
			}
		case "way":
			pts := pointsFrom(el.Geometry)
			if len(pts) == 0 {
				continue
			}
			kind := data.GeometryLineString
			if isClosedRing(pts) && (data.OSMTags(el.Tags).IsBuilding() ||
				data.OSMTags(el.Tags).IsWater() ||
				data.OSMTags(el.Tags).IsGreen()) {
				kind = data.GeometryPolygon
			}
			f.Geometry = data.Geometry{Kind: kind, Points: pts}
		case "relation":
			// Render outer rings only — sufficient for visual fidelity in
			// a TUI without a full topology engine.
			for _, m := range el.Members {
				if m.Role != "outer" {
					continue
				}
				pts := pointsFrom(m.Geometry)
				if len(pts) < 3 {
					continue
				}
				f.Geometry = data.Geometry{Kind: data.GeometryPolygon, Points: pts}
				break
			}
		}
		if f.Geometry.Kind == data.GeometryUnknown {
			continue
		}
		out.Features = append(out.Features, f)
	}
	return out
}

func pointsFrom(in []overpassPoint) []geo.LatLng {
	out := make([]geo.LatLng, len(in))
	for i, p := range in {
		out[i] = geo.LatLng{Lat: p.Lat, Lng: p.Lon}
	}
	return out
}

func isClosedRing(p []geo.LatLng) bool {
	return len(p) >= 4 && p[0] == p[len(p)-1]
}
