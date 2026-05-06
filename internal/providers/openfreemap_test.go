// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenFreeMapWithExplicitURL(t *testing.T) {
	// Pass a URL with placeholders directly — no TileJSON resolution.
	c := NewClient(ClientOptions{})
	src := NewOpenFreeMapSource(context.Background(), c,
		"https://example/{z}/{x}/{y}.pbf", 0)
	require.NotNil(t, src)
	assert.Contains(t, src.urlTemplate, "{z}")
}

func TestOpenFreeMapTileJSONResolution(t *testing.T) {
	var calledTileJSON, calledTile bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/tilejson"):
			calledTileJSON = true
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"tilejson":"3.0.0","tiles":["%s/v123/{z}/{x}/{y}.pbf"]}`, "http://"+r.Host)
		default:
			calledTile = true
			feat := buildLineFeature(t)
			layer := buildLayer(t, "transportation", feat, "class", "primary")
			_, _ = w.Write(buildTile(t, layer))
		}
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	src := NewOpenFreeMapSource(context.Background(), c, srv.URL+"/tilejson", 0)
	require.NotNil(t, src)
	assert.True(t, calledTileJSON, "TileJSON should be fetched at construction")
	assert.Contains(t, src.urlTemplate, "v123")

	bbox := geo.BBox{South: 44.83, West: -0.59, North: 44.85, East: -0.56}
	fc, err := src.FetchMapLayers(context.Background(), bbox, 12)
	require.NoError(t, err)
	assert.NotEmpty(t, fc.Features)
	assert.True(t, calledTile)
}

func TestOMTRoadClassRewrite(t *testing.T) {
	cases := []struct {
		class string
		want  string
	}{
		{"motorway", "motorway"},
		{"trunk", "motorway"},
		{"primary", "primary"},
		{"secondary", "secondary"},
		{"tertiary", "secondary"},
		{"minor", "residential"},
		{"service", "service"},
		{"track", "track"},
		{"path", "pedestrian"},
		{"footway", "pedestrian"},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			f := data.Feature{Tags: data.OSMTags{"__layer": "transportation", "class": tc.class}}
			rewriteOMTFeature(&f)
			assert.Equal(t, tc.want, f.Tags["highway"])
		})
	}
}

func TestOMTPOIClassRewrite(t *testing.T) {
	cases := []struct {
		class string
		key   string
		want  string
	}{
		{"restaurant", "amenity", "restaurant"},
		{"cafe", "amenity", "cafe"},
		{"hospital", "amenity", "hospital"},
		{"hotel", "tourism", "hotel"},
		{"museum", "tourism", "museum"},
		{"shop", "shop", "yes"},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			f := data.Feature{Tags: data.OSMTags{"__layer": "poi", "class": tc.class}}
			rewriteOMTFeature(&f)
			assert.Equal(t, tc.want, f.Tags[tc.key])
		})
	}
}

func TestOMTZoomFiltering(t *testing.T) {
	building := data.Feature{Tags: data.OSMTags{"__layer": "building"}}
	assert.False(t, omtFeatureRenderable(building, 12))
	assert.True(t, omtFeatureRenderable(building, 14))

	city := data.Feature{Tags: data.OSMTags{"__layer": "place", "class": "city"}}
	assert.True(t, omtFeatureRenderable(city, 7))

	// Neighbourhood / suburb dropped — they're encoded as landuse classes.
	suburb := data.Feature{Tags: data.OSMTags{"__layer": "landuse", "class": "suburb"}}
	assert.False(t, omtFeatureRenderable(suburb, 14))

	primary := data.Feature{Tags: data.OSMTags{"__layer": "transportation", "class": "primary"}}
	assert.True(t, omtFeatureRenderable(primary, 8))
	tertiary := data.Feature{Tags: data.OSMTags{"__layer": "transportation", "class": "tertiary"}}
	assert.False(t, omtFeatureRenderable(tertiary, 8))
}

func TestOMTLandcoverRewrite(t *testing.T) {
	f := data.Feature{Tags: data.OSMTags{"__layer": "landcover", "class": "wood"}}
	rewriteOMTFeature(&f)
	assert.Equal(t, "wood", f.Tags["natural"])

	f = data.Feature{Tags: data.OSMTags{"__layer": "landcover", "class": "grass"}}
	rewriteOMTFeature(&f)
	assert.Equal(t, "grassland", f.Tags["natural"])
}
