// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const overpassSample = `{
  "elements": [
    {
      "type": "way",
      "id": 100,
      "tags": {"highway":"primary","name":"Cours de la Marne"},
      "geometry": [
        {"lat":44.83,"lon":-0.58},
        {"lat":44.84,"lon":-0.57},
        {"lat":44.85,"lon":-0.55}
      ]
    },
    {
      "type": "way",
      "id": 200,
      "tags": {"natural":"water"},
      "geometry": [
        {"lat":44.83,"lon":-0.60},
        {"lat":44.83,"lon":-0.59},
        {"lat":44.84,"lon":-0.59},
        {"lat":44.84,"lon":-0.60},
        {"lat":44.83,"lon":-0.60}
      ]
    },
    {
      "type": "node",
      "id": 300,
      "lat": 44.835,
      "lon": -0.572,
      "tags": {"amenity":"cafe","name":"Le Cycl0"}
    }
  ]
}`

func TestOverpassQueryFeatures(t *testing.T) {
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		body, _ := io.ReadAll(r.Body)
		lastBody = string(body)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(overpassSample))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	o := NewOverpass(c, srv.URL)
	bbox := geo.BBox{South: 44.8, West: -0.6, North: 44.9, East: -0.5}
	fc, err := o.QueryFeatures(context.Background(), bbox, `way["highway"];`, 25)
	require.NoError(t, err)
	require.Len(t, fc.Features, 3)

	// First feature is a road LineString.
	assert.Equal(t, data.GeometryLineString, fc.Features[0].Geometry.Kind)
	assert.Equal(t, "Cours de la Marne", fc.Features[0].Name)
	// Second is a closed water polygon.
	assert.Equal(t, data.GeometryPolygon, fc.Features[1].Geometry.Kind)
	// Third is a POI node.
	assert.Equal(t, data.GeometryPoint, fc.Features[2].Geometry.Kind)
	assert.Equal(t, "Le Cycl0", fc.Features[2].Name)

	assert.Contains(t, lastBody, "data=")
	assert.Contains(t, lastBody, "out+body+geom") // form-encoded
}

func TestOverpassFetchMapLayers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// At zoom 16 the layer query should include buildings.
		assert.Contains(t, string(body), "building")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	o := NewOverpass(c, srv.URL)
	bbox := geo.BBox{South: 44.8, West: -0.6, North: 44.9, East: -0.5}
	_, err := o.FetchMapLayers(context.Background(), bbox, 16)
	require.NoError(t, err)
}

func TestOverpassFetchPOIsBuildsRegex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.True(t, strings.Contains(string(body), "amenity") ||
			strings.Contains(string(body), "shop"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	o := NewOverpass(c, srv.URL)
	_, err := o.FetchPOIs(context.Background(),
		geo.BBox{South: 44.8, West: -0.6, North: 44.9, East: -0.5},
		[]data.POICategory{data.POIRestaurant, data.POIShopping})
	require.NoError(t, err)
}
