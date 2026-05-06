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

const nominatimSample = `[
  {
    "place_id": 1,
    "lat": "44.8378",
    "lon": "-0.5792",
    "display_name": "Bordeaux, Gironde, France",
    "class": "place",
    "type": "city",
    "importance": 0.9,
    "boundingbox": ["44.8000", "44.9000", "-0.6500", "-0.5000"]
  }
]`

func TestNominatimSearch(t *testing.T) {
	var calledPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path
		w.WriteHeader(200)
		_, _ = w.Write([]byte(nominatimSample))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	n := NewNominatim(c, srv.URL)
	results, err := n.Search(context.Background(), "Bordeaux", SearchOptions{Limit: 3})
	require.NoError(t, err)
	require.Len(t, results, 1)

	r := results[0]
	assert.Equal(t, "Bordeaux, Gironde, France", r.DisplayName)
	assert.InDelta(t, 44.8378, r.Position.Lat, 1e-6)
	assert.InDelta(t, -0.5792, r.Position.Lng, 1e-6)
	assert.Equal(t, "place", r.Class)
	assert.Equal(t, "city", r.Type)
	require.NotNil(t, r.BBox)
	assert.InDelta(t, 44.80, r.BBox.South, 1e-6)
	assert.InDelta(t, 44.90, r.BBox.North, 1e-6)
	assert.Contains(t, calledPath, "/search")
}

func TestNominatimSearchEmptyQuery(t *testing.T) {
	c := NewClient(ClientOptions{})
	n := NewNominatim(c, "https://example.invalid/")
	_, err := n.Search(context.Background(), "  ", SearchOptions{})
	require.Error(t, err)
}

func TestNominatimReverse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/reverse")
		assert.Equal(t, "44.837800", r.URL.Query().Get("lat"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"lat":"44.8378","lon":"-0.5792","display_name":"Bordeaux"}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	n := NewNominatim(c, srv.URL)
	res, err := n.Reverse(context.Background(), geo.LatLng{Lat: 44.8378, Lng: -0.5792}, "")
	require.NoError(t, err)
	assert.Equal(t, "Bordeaux", res.DisplayName)
}
