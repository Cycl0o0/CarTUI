// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cycl0o0/cartui/internal/geo"
)

// TomTom is a thin client over the public TomTom Traffic APIs. The free
// developer tier requires an API key (https://developer.tomtom.com).
//
// CarTUI uses two endpoints:
//
//   - Traffic Incidents Details v5 — point + polyline incidents for a
//     bbox.
//   - Traffic Flow Segment Data v4 — current vs free-flow speed for a
//     single point. Used to colour-code roads on demand.
type TomTom struct {
	client  *Client
	baseURL string
	apiKey  string
}

// NewTomTom builds a client. An empty baseURL defaults to the public
// `https://api.tomtom.com` endpoint.
//
// Returns nil when apiKey is empty — the caller should treat that as
// "traffic disabled" rather than crashing.
func NewTomTom(c *Client, baseURL, apiKey string) *TomTom {
	if apiKey == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = "https://api.tomtom.com"
	}
	return &TomTom{
		client:  c,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

// IncidentSeverity classifies an incident by user impact, mapped from
// TomTom's `magnitudeOfDelay` field.
type IncidentSeverity uint8

// Severity values, lowest to highest.
const (
	SeverityUnknown IncidentSeverity = iota
	SeverityMinor
	SeverityModerate
	SeverityMajor
	SeverityClosure
)

// String returns a short label for the severity.
func (s IncidentSeverity) String() string {
	switch s {
	case SeverityClosure:
		return "fermeture"
	case SeverityMajor:
		return "ralentissement majeur"
	case SeverityModerate:
		return "ralentissement modéré"
	case SeverityMinor:
		return "incident mineur"
	}
	return "inconnu"
}

// Incident is a normalised traffic event ready to display.
type Incident struct {
	Position    geo.LatLng
	Geometry    []geo.LatLng // polyline; empty when only a point is known
	Description string
	IconCat     int
	Severity    IncidentSeverity
}

// Incidents fetches every traffic incident in the bbox.
func (t *TomTom) Incidents(ctx context.Context, b geo.BBox, lang string) ([]Incident, error) {
	if t == nil {
		return nil, errors.New("tomtom: not configured")
	}
	q := url.Values{}
	q.Set("key", t.apiKey)
	q.Set("bbox", fmt.Sprintf("%f,%f,%f,%f", b.West, b.South, b.East, b.North))
	q.Set("fields", "{incidents{type,geometry{type,coordinates},properties{events{description},magnitudeOfDelay,iconCategory}}}")
	if lang != "" {
		q.Set("language", lang)
	}
	u := t.baseURL + "/traffic/services/5/incidentDetails?" + q.Encode()

	var raw incidentsResponse
	if err := t.client.GetJSON(ctx, u, &raw); err != nil {
		return nil, fmt.Errorf("tomtom incidents: %w", err)
	}
	out := make([]Incident, 0, len(raw.Incidents))
	for _, in := range raw.Incidents {
		inc := Incident{
			IconCat:  in.Properties.IconCategory,
			Severity: severityFromMagnitude(in.Properties.MagnitudeOfDelay),
		}
		if len(in.Properties.Events) > 0 {
			inc.Description = in.Properties.Events[0].Description
		}
		switch in.Geometry.Type {
		case "Point":
			// GeoJSON: coordinates is a single [lng, lat] pair.
			if c, ok := pointFromAny(any(in.Geometry.Coordinates)); ok {
				inc.Position = c
			}
		case "LineString", "MultiLineString":
			line := flattenLine(in.Geometry.Coordinates)
			inc.Geometry = line
			if len(line) > 0 {
				inc.Position = line[len(line)/2]
			}
		}
		if inc.Position.Lat == 0 && inc.Position.Lng == 0 {
			continue
		}
		out = append(out, inc)
	}
	return out, nil
}

// Flow describes the current vs free-flow conditions at a single point —
// the response shape of [TomTom.FlowAtPoint].
type Flow struct {
	CurrentSpeed  float64 // km/h
	FreeFlowSpeed float64 // km/h
	Confidence    float64 // 0..1
	RoadClosure   bool
}

// FlowAtPoint queries the Flow Segment Data API.
func (t *TomTom) FlowAtPoint(ctx context.Context, p geo.LatLng) (Flow, error) {
	if t == nil {
		return Flow{}, errors.New("tomtom: not configured")
	}
	q := url.Values{}
	q.Set("key", t.apiKey)
	q.Set("point", strconv.FormatFloat(p.Lat, 'f', 6, 64)+","+strconv.FormatFloat(p.Lng, 'f', 6, 64))
	u := t.baseURL + "/traffic/services/4/flowSegmentData/relative0/10/json?" + q.Encode()

	var raw flowResponse
	if err := t.client.GetJSON(ctx, u, &raw); err != nil {
		return Flow{}, fmt.Errorf("tomtom flow: %w", err)
	}
	return Flow{
		CurrentSpeed:  float64(raw.FlowSegmentData.CurrentSpeed),
		FreeFlowSpeed: float64(raw.FlowSegmentData.FreeFlowSpeed),
		Confidence:    raw.FlowSegmentData.Confidence,
		RoadClosure:   raw.FlowSegmentData.RoadClosure,
	}, nil
}

// IncidentGlyph returns the user-visible glyph for an incident severity.
func IncidentGlyph(s IncidentSeverity) rune {
	switch s {
	case SeverityClosure:
		return '⛔'
	case SeverityMajor:
		return '⚠'
	case SeverityModerate:
		return '!'
	case SeverityMinor:
		return '·'
	}
	return '?'
}

// severityFromMagnitude maps TomTom's 0..4 magnitudeOfDelay to our enum.
func severityFromMagnitude(m int) IncidentSeverity {
	switch m {
	case 4:
		return SeverityClosure
	case 3:
		return SeverityMajor
	case 2:
		return SeverityModerate
	case 1:
		return SeverityMinor
	}
	return SeverityUnknown
}

// pointFromAny accepts either []float64 (the GeoJSON shape) or a generic
// [interface{}] slice (decoded by the client).
func pointFromAny(p any) (geo.LatLng, bool) {
	switch v := p.(type) {
	case []any:
		if len(v) < 2 {
			return geo.LatLng{}, false
		}
		lng, ok1 := v[0].(float64)
		lat, ok2 := v[1].(float64)
		if !ok1 || !ok2 {
			return geo.LatLng{}, false
		}
		return geo.LatLng{Lat: lat, Lng: lng}, true
	case []float64:
		if len(v) < 2 {
			return geo.LatLng{}, false
		}
		return geo.LatLng{Lat: v[1], Lng: v[0]}, true
	}
	return geo.LatLng{}, false
}

// flattenLine accepts either a LineString ([]Point) or a MultiLineString
// ([][]Point) and returns a flat polyline.
func flattenLine(raw []any) []geo.LatLng {
	out := make([]geo.LatLng, 0, len(raw))
	for _, item := range raw {
		v, ok := item.([]any)
		if !ok {
			continue
		}
		// Could be a Point or another nested line.
		if p, ok := pointFromAny(v); ok {
			out = append(out, p)
			continue
		}
		for _, p := range v {
			if pt, ok := pointFromAny(p); ok {
				out = append(out, pt)
			}
		}
	}
	return out
}

// incidentsResponse mirrors the upstream JSON.
type incidentsResponse struct {
	Incidents []incidentItem `json:"incidents"`
}

type incidentItem struct {
	Type       string             `json:"type"`
	Geometry   incidentGeometry   `json:"geometry"`
	Properties incidentProperties `json:"properties"`
}

type incidentGeometry struct {
	Type        string `json:"type"`
	Coordinates []any  `json:"coordinates"`
}

type incidentProperties struct {
	Events           []incidentEvent `json:"events"`
	MagnitudeOfDelay int             `json:"magnitudeOfDelay"`
	IconCategory     int             `json:"iconCategory"`
}

type incidentEvent struct {
	Description string `json:"description"`
	Code        int    `json:"code"`
}

type flowResponse struct {
	FlowSegmentData struct {
		CurrentSpeed  int     `json:"currentSpeed"`
		FreeFlowSpeed int     `json:"freeFlowSpeed"`
		Confidence    float64 `json:"confidence"`
		RoadClosure   bool    `json:"roadClosure"`
	} `json:"flowSegmentData"`
}
