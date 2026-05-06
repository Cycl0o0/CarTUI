// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapboxDisabledWithoutToken(t *testing.T) {
	c := NewClient(ClientOptions{})
	src := NewMapboxSource(c, "https://example.invalid", "", "", 0)
	assert.Nil(t, src)
}

func TestMapboxFetchTileBuildsCorrectURL(t *testing.T) {
	var calledPath, calledQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path
		calledQuery = r.URL.RawQuery
		// Return a minimal MVT (one road feature).
		feat := buildLineFeature(t)
		layer := buildLayer(t, "road", feat, "class", "primary")
		_, _ = w.Write(buildTile(t, layer))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	src := NewMapboxSource(c, srv.URL, "mapbox.mapbox-streets-v8", "test-token", 0)
	require.NotNil(t, src)

	bbox := geo.BBox{South: 44.83, West: -0.59, North: 44.85, East: -0.56}
	fc, err := src.FetchMapLayers(context.Background(), bbox, 12)
	require.NoError(t, err)

	assert.Contains(t, calledPath, "/v4/mapbox.mapbox-streets-v8/12/")
	assert.Contains(t, calledQuery, "access_token=test-token")
	require.NotEmpty(t, fc.Features)
}

func TestMapboxRoadClassRewrite(t *testing.T) {
	cases := []struct {
		class string
		want  string
	}{
		{"motorway", "motorway"},
		{"motorway_link", "motorway"},
		{"trunk", "motorway"},
		{"primary", "primary"},
		{"secondary", "secondary"},
		{"tertiary", "secondary"},
		{"street", "residential"},
		{"service", "service"},
		{"pedestrian", "pedestrian"},
		{"footway", "pedestrian"},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			f := data.Feature{Tags: data.OSMTags{"__layer": "road", "class": tc.class}}
			rewriteMapboxFeature(&f)
			assert.Equal(t, tc.want, f.Tags["highway"])
		})
	}
}

func TestMapboxPOIClassRewrite(t *testing.T) {
	cases := []struct {
		class string
		key   string
		want  string
	}{
		{"food_and_drink", "amenity", "restaurant"},
		{"lodging", "tourism", "hotel"},
		{"education", "amenity", "school"},
		{"medical", "amenity", "hospital"},
		{"shopping", "shop", "yes"},
		{"arts_and_entertainment", "tourism", "attraction"},
		{"park_like", "leisure", "park"},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			f := data.Feature{Tags: data.OSMTags{"__layer": "poi_label", "class": tc.class}}
			rewriteMapboxFeature(&f)
			assert.Equal(t, tc.want, f.Tags[tc.key])
		})
	}
}

func TestMapboxFeatureRenderableFiltersUrbanLanduse(t *testing.T) {
	for _, class := range []string{"residential", "commercial", "industrial"} {
		f := data.Feature{Tags: data.OSMTags{"__layer": "landuse", "class": class}}
		assert.False(t, mapboxFeatureRenderable(f))
	}

	// `park` survives.
	f := data.Feature{Tags: data.OSMTags{"__layer": "landuse", "class": "park"}}
	assert.True(t, mapboxFeatureRenderable(f))
}

func TestMapboxHouseNumLabelDropped(t *testing.T) {
	f := data.Feature{Tags: data.OSMTags{"__layer": "housenum_label"}}
	assert.False(t, mapboxFeatureRenderable(f))
}

func TestMapboxRoadLabelKeepsName(t *testing.T) {
	// road_label features carry both class + name, and we render them
	// as roads so labels can be picked up by drawLabels.
	f := data.Feature{
		Name: "Cours de la Marne",
		Tags: data.OSMTags{"__layer": "road_label", "class": "primary"},
	}
	rewriteMapboxFeature(&f)
	assert.Equal(t, "primary", f.Tags["highway"])
	assert.Equal(t, "Cours de la Marne", f.Name)
}

func TestMapboxFetchTileTokenURLEncoded(t *testing.T) {
	// Token characters that need URL escaping should arrive intact.
	tokenWithSpecial := "abc/+xyz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// http.Request.URL.Query() automatically decodes — perfect for
		// asserting that we sent the right thing.
		assert.Equal(t, tokenWithSpecial, r.URL.Query().Get("access_token"))
		_, _ = w.Write([]byte{}) // empty MVT is OK
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	src := NewMapboxSource(c, srv.URL, "", tokenWithSpecial, 0)
	require.NotNil(t, src)
	_, _ = src.FetchMapLayers(context.Background(),
		geo.BBox{South: 44.83, West: -0.59, North: 44.85, East: -0.56}, 10)
}

func TestMapboxIsGzipMagic(t *testing.T) {
	assert.True(t, isGzipMagic([]byte{0x1f, 0x8b, 0x08}))
	assert.False(t, isGzipMagic([]byte{0x1f}))
	assert.False(t, isGzipMagic([]byte{0x00, 0x00}))
	assert.False(t, isGzipMagic(nil))
}

// nameForTest exists only to silence "unused" linter complaints if
// strings is removed in a future refactor.
var _ = strings.TrimSpace
