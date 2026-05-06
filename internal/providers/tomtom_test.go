// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tomtomIncidentsSample = `{
  "incidents": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [-0.5750, 44.8400]
      },
      "properties": {
        "events": [{"description":"Accident","code":401}],
        "magnitudeOfDelay": 3,
        "iconCategory": 1
      }
    },
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [[-0.5800,44.8300],[-0.5780,44.8320],[-0.5760,44.8340]]
      },
      "properties": {
        "events": [{"description":"Travaux","code":701}],
        "magnitudeOfDelay": 2,
        "iconCategory": 2
      }
    }
  ]
}`

func TestTomTomIncidents(t *testing.T) {
	var calledKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledKey = r.URL.Query().Get("key")
		assert.Contains(t, r.URL.Path, "/traffic/services/5/incidentDetails")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(tomtomIncidentsSample))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	tt := NewTomTom(c, srv.URL, "test-key")
	require.NotNil(t, tt)

	bbox := geo.BBox{South: 44.80, West: -0.60, North: 44.90, East: -0.50}
	got, err := tt.Incidents(context.Background(), bbox, "fr-FR")
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, "test-key", calledKey)
	assert.Equal(t, SeverityMajor, got[0].Severity)
	assert.Equal(t, "Accident", got[0].Description)
	assert.InDelta(t, 44.84, got[0].Position.Lat, 1e-6)
	assert.InDelta(t, -0.575, got[0].Position.Lng, 1e-6)

	assert.Equal(t, SeverityModerate, got[1].Severity)
	assert.NotEmpty(t, got[1].Geometry, "linestring should be preserved")
}

func TestTomTomDisabledWithoutKey(t *testing.T) {
	c := NewClient(ClientOptions{})
	tt := NewTomTom(c, "https://example.invalid", "")
	assert.Nil(t, tt, "no key -> nil client")
}

func TestTomTomFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"flowSegmentData":{"currentSpeed":35,"freeFlowSpeed":50,"confidence":0.95,"roadClosure":false}}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	tt := NewTomTom(c, srv.URL, "k")
	flow, err := tt.FlowAtPoint(context.Background(), geo.LatLng{Lat: 44.84, Lng: -0.57})
	require.NoError(t, err)
	assert.InDelta(t, 35, flow.CurrentSpeed, 0.001)
	assert.InDelta(t, 50, flow.FreeFlowSpeed, 0.001)
	assert.InDelta(t, 0.95, flow.Confidence, 0.001)
	assert.False(t, flow.RoadClosure)
}

func TestSeverityMapping(t *testing.T) {
	assert.Equal(t, SeverityClosure, severityFromMagnitude(4))
	assert.Equal(t, SeverityMajor, severityFromMagnitude(3))
	assert.Equal(t, SeverityModerate, severityFromMagnitude(2))
	assert.Equal(t, SeverityMinor, severityFromMagnitude(1))
	assert.Equal(t, SeverityUnknown, severityFromMagnitude(0))
}

func TestIncidentGlyphCoversEverySeverity(t *testing.T) {
	for s := SeverityUnknown; s <= SeverityClosure; s++ {
		assert.NotEqual(t, rune(0), IncidentGlyph(s))
	}
}
