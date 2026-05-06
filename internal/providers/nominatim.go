// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
)

// Nominatim is the OpenStreetMap geocoding service client. The hosted
// instance at `https://nominatim.openstreetmap.org/` is heavily rate-limited
// (1 req/s) and requires an identifiable User-Agent — the shared [Client]
// already takes care of both.
type Nominatim struct {
	client  *Client
	baseURL string
}

// NewNominatim builds a client. Passing an empty baseURL falls back to the
// public OSM-hosted endpoint.
func NewNominatim(c *Client, baseURL string) *Nominatim {
	if baseURL == "" {
		baseURL = "https://nominatim.openstreetmap.org/"
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Nominatim{client: c, baseURL: baseURL}
}

// SearchOptions tunes a [Nominatim.Search] call.
type SearchOptions struct {
	Limit         int     // max results (1..50, default 10)
	CountryCode   string  // ISO 3166-1 alpha2, comma-separated for multiple
	Language      string  // RFC 1766 lang tag for `accept-language` header
	BoundedToBBox *geo.BBox
}

// SearchResult is a single Nominatim hit, normalised to a CarTUI shape.
type SearchResult struct {
	DisplayName string
	Position    geo.LatLng
	Class       string // top-level OSM class (`amenity`, `place`, …)
	Type        string // sub-type within the class (`cafe`, `city`, …)
	Importance  float64
	BBox        *geo.BBox // available for areas (cities, regions, etc.)
}

// Search performs a forward geocoding query.
func (n *Nominatim) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("nominatim: empty query")
	}
	limit := opts.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	q := url.Values{}
	q.Set("q", query)
	q.Set("format", "jsonv2")
	q.Set("addressdetails", "1")
	q.Set("limit", strconv.Itoa(limit))
	if opts.CountryCode != "" {
		q.Set("countrycodes", opts.CountryCode)
	}
	if opts.BoundedToBBox != nil {
		b := *opts.BoundedToBBox
		// Nominatim viewbox is "left,top,right,bottom" = "west,north,east,south".
		q.Set("viewbox", fmt.Sprintf("%f,%f,%f,%f", b.West, b.North, b.East, b.South))
		q.Set("bounded", "1")
	}

	url := n.baseURL + "search?" + q.Encode()
	headers := map[string]string{}
	if opts.Language != "" {
		headers["Accept-Language"] = opts.Language
	}

	var raw []nominatimResult
	if err := n.client.RequestJSON(ctx, "GET", url, headers, nil, &raw); err != nil {
		return nil, fmt.Errorf("nominatim search: %w", err)
	}

	out := make([]SearchResult, 0, len(raw))
	for _, r := range raw {
		lat, errLat := strconv.ParseFloat(r.Lat, 64)
		lon, errLon := strconv.ParseFloat(r.Lon, 64)
		if errLat != nil || errLon != nil {
			continue
		}
		res := SearchResult{
			DisplayName: r.DisplayName,
			Position:    geo.LatLng{Lat: lat, Lng: lon},
			Class:       r.Class,
			Type:        r.Type,
			Importance:  r.Importance,
		}
		if len(r.BoundingBox) == 4 {
			south, errS := strconv.ParseFloat(r.BoundingBox[0], 64)
			north, errN := strconv.ParseFloat(r.BoundingBox[1], 64)
			west, errW := strconv.ParseFloat(r.BoundingBox[2], 64)
			east, errE := strconv.ParseFloat(r.BoundingBox[3], 64)
			if errS == nil && errN == nil && errW == nil && errE == nil {
				bb := geo.BBox{South: south, West: west, North: north, East: east}
				res.BBox = &bb
			}
		}
		out = append(out, res)
	}
	return out, nil
}

// Reverse maps a coordinate back to a human-readable address.
func (n *Nominatim) Reverse(ctx context.Context, p geo.LatLng, lang string) (SearchResult, error) {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(p.Lat, 'f', 6, 64))
	q.Set("lon", strconv.FormatFloat(p.Lng, 'f', 6, 64))
	q.Set("format", "jsonv2")
	q.Set("addressdetails", "1")

	headers := map[string]string{}
	if lang != "" {
		headers["Accept-Language"] = lang
	}

	var raw nominatimResult
	if err := n.client.RequestJSON(ctx, "GET", n.baseURL+"reverse?"+q.Encode(), headers, nil, &raw); err != nil {
		return SearchResult{}, fmt.Errorf("nominatim reverse: %w", err)
	}
	lat, _ := strconv.ParseFloat(raw.Lat, 64)
	lon, _ := strconv.ParseFloat(raw.Lon, 64)
	return SearchResult{
		DisplayName: raw.DisplayName,
		Position:    geo.LatLng{Lat: lat, Lng: lon},
		Class:       raw.Class,
		Type:        raw.Type,
		Importance:  raw.Importance,
	}, nil
}

// ToBookmark converts a search result into a default-named bookmark.
func (r SearchResult) ToBookmark() data.Bookmark {
	return data.Bookmark{
		Name:     r.DisplayName,
		Position: r.Position,
	}
}

// nominatimResult mirrors the wire shape of a single Nominatim JSON result.
// All numeric fields come down as strings — that's the API behaviour.
type nominatimResult struct {
	PlaceID     int64    `json:"place_id"`
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	DisplayName string   `json:"display_name"`
	Class       string   `json:"class"`
	Type        string   `json:"type"`
	Importance  float64  `json:"importance"`
	BoundingBox []string `json:"boundingbox"`
}
