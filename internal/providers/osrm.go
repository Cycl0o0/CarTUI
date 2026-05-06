// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
)

// OSRM wraps the OpenStreetMap Routing Machine HTTP API. Compatible
// alternatives (GraphHopper, Valhalla) speak a similar dialect; only the URL
// path and a few field names differ — implement them as separate types if
// the need arises.
type OSRM struct {
	client  *Client
	baseURL string
}

// NewOSRM builds a client. baseURL defaults to the public demo server when
// empty.
func NewOSRM(c *Client, baseURL string) *OSRM {
	if baseURL == "" {
		baseURL = "https://router.project-osrm.org/"
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &OSRM{client: c, baseURL: baseURL}
}

// Route asks for a route between the given waypoints, in order. At least two
// points are required.
func (o *OSRM) Route(ctx context.Context, profile data.RouteProfile, waypoints []geo.LatLng) (data.Route, error) {
	if len(waypoints) < 2 {
		return data.Route{}, fmt.Errorf("osrm: need at least two waypoints")
	}
	prof := osrmProfile(profile)
	coords := make([]string, len(waypoints))
	for i, w := range waypoints {
		coords[i] = strconv.FormatFloat(w.Lng, 'f', 6, 64) + "," +
			strconv.FormatFloat(w.Lat, 'f', 6, 64)
	}
	q := url.Values{}
	q.Set("overview", "full")
	q.Set("geometries", "geojson")
	q.Set("steps", "true")

	endpoint := fmt.Sprintf("%sroute/v1/%s/%s?%s",
		o.baseURL, prof, strings.Join(coords, ";"), q.Encode())

	var raw osrmRouteResponse
	if err := o.client.GetJSON(ctx, endpoint, &raw); err != nil {
		return data.Route{}, fmt.Errorf("osrm route: %w", err)
	}
	if raw.Code != "Ok" {
		return data.Route{}, fmt.Errorf("osrm: %s: %s", raw.Code, raw.Message)
	}
	if len(raw.Routes) == 0 {
		return data.Route{}, fmt.Errorf("osrm: no route returned")
	}
	r := raw.Routes[0]
	return data.Route{
		Distance: r.Distance,
		Duration: time.Duration(r.Duration * float64(time.Second)),
		Geometry: geometryFromGeoJSON(r.Geometry),
		Steps:    flattenSteps(r.Legs),
		Profile:  profile,
	}, nil
}

// ToGPX serialises a route as a minimal GPX 1.1 document.
//
// The output is suitable for opening in any GPS application or website that
// accepts the format.
func ToGPX(r data.Route, name string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<gpx version="1.1" creator="CarTUI" xmlns="http://www.topografix.com/GPX/1/1">` + "\n")
	b.WriteString("  <trk>\n")
	b.WriteString("    <name>")
	b.WriteString(escapeXML(name))
	b.WriteString("</name>\n")
	b.WriteString("    <trkseg>\n")
	for _, p := range r.Geometry {
		fmt.Fprintf(&b, "      <trkpt lat=\"%.6f\" lon=\"%.6f\"></trkpt>\n", p.Lat, p.Lng)
	}
	b.WriteString("    </trkseg>\n")
	b.WriteString("  </trk>\n")
	b.WriteString("</gpx>\n")
	return b.String()
}

func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

func osrmProfile(p data.RouteProfile) string {
	switch p {
	case data.ProfileCycling:
		return "cycling"
	case data.ProfileWalking:
		return "foot"
	default:
		return "driving"
	}
}

func geometryFromGeoJSON(g osrmGeometry) []geo.LatLng {
	out := make([]geo.LatLng, len(g.Coordinates))
	for i, c := range g.Coordinates {
		if len(c) < 2 {
			continue
		}
		out[i] = geo.LatLng{Lng: c[0], Lat: c[1]}
	}
	return out
}

func flattenSteps(legs []osrmLeg) []data.RouteStep {
	var total int
	for _, l := range legs {
		total += len(l.Steps)
	}
	out := make([]data.RouteStep, 0, total)
	for _, l := range legs {
		for _, s := range l.Steps {
			out = append(out, data.RouteStep{
				Instruction: s.Maneuver.instruction(),
				Distance:    s.Distance,
				Duration:    time.Duration(s.Duration * float64(time.Second)),
				Geometry:    geometryFromGeoJSON(s.Geometry),
			})
		}
	}
	return out
}

type osrmRouteResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message,omitempty"`
	Routes  []osrmRoute `json:"routes"`
}

type osrmRoute struct {
	Distance float64      `json:"distance"`
	Duration float64      `json:"duration"`
	Geometry osrmGeometry `json:"geometry"`
	Legs     []osrmLeg    `json:"legs"`
}

type osrmGeometry struct {
	Coordinates [][]float64 `json:"coordinates"`
}

type osrmLeg struct {
	Steps []osrmStep `json:"steps"`
}

type osrmStep struct {
	Distance float64      `json:"distance"`
	Duration float64      `json:"duration"`
	Geometry osrmGeometry `json:"geometry"`
	Maneuver osrmManeuver `json:"maneuver"`
	Name     string       `json:"name"`
}

type osrmManeuver struct {
	Type     string `json:"type"`
	Modifier string `json:"modifier,omitempty"`
}

func (m osrmManeuver) instruction() string {
	t := m.Type
	if m.Modifier != "" {
		t += " " + m.Modifier
	}
	return t
}
