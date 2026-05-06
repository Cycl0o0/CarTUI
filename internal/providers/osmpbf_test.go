// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"testing"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
)

func TestGridKeyFor(t *testing.T) {
	// Two close coordinates should land in the same grid cell.
	a := gridKeyFor(44.8378, -0.5792)
	b := gridKeyFor(44.8400, -0.5800)
	assert.Equal(t, a, b)

	// A coordinate one cell east should differ in x.
	c := gridKeyFor(44.8378, -0.5792+pbfGridCellSize+0.001)
	assert.NotEqual(t, a, c)
}

func TestBboxOfPoints(t *testing.T) {
	pts := []geo.LatLng{
		{Lat: 44.8, Lng: -0.6},
		{Lat: 44.9, Lng: -0.5},
		{Lat: 44.85, Lng: -0.55},
	}
	bb := bboxOfPoints(pts)
	assert.InDelta(t, 44.8, bb.South, 1e-9)
	assert.InDelta(t, 44.9, bb.North, 1e-9)
	assert.InDelta(t, -0.6, bb.West, 1e-9)
	assert.InDelta(t, -0.5, bb.East, 1e-9)
}

func TestUnionBBox(t *testing.T) {
	a := geo.BBox{South: 1, West: 1, North: 5, East: 5}
	b := geo.BBox{South: 3, West: -1, North: 7, East: 4}
	u := unionBBox(a, b)
	assert.InDelta(t, 1.0, u.South, 1e-9)
	assert.InDelta(t, -1.0, u.West, 1e-9)
	assert.InDelta(t, 7.0, u.North, 1e-9)
	assert.InDelta(t, 5.0, u.East, 1e-9)
}

func TestIsAreaWay(t *testing.T) {
	pts := []geo.LatLng{
		{Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}, {Lat: 1, Lng: 1}, {Lat: 0, Lng: 0},
	}
	// Building: yes -> area.
	assert.True(t, isAreaWay(data.OSMTags{"building": "yes"}, pts))
	// Open polyline (last point != first) -> not area.
	open := []geo.LatLng{{Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}}
	assert.False(t, isAreaWay(data.OSMTags{"building": "yes"}, open))
	// Closed but no area-like tag -> not area.
	assert.False(t, isAreaWay(data.OSMTags{"barrier": "fence"}, pts))
}

func TestPBFSourceAddAndQuery(t *testing.T) {
	p := &PBFSource{grid: map[pbfGridKey][]int{}}

	// Two points: one in Bordeaux, one in Paris.
	bdx := data.Feature{
		Name:     "Le Cycl0",
		Tags:     data.OSMTags{"amenity": "cafe", "name": "Le Cycl0"},
		Geometry: data.Geometry{Kind: data.GeometryPoint, Points: []geo.LatLng{{Lat: 44.8378, Lng: -0.5792}}},
	}
	paris := data.Feature{
		Name:     "Café Paris",
		Tags:     data.OSMTags{"amenity": "cafe", "name": "Café Paris"},
		Geometry: data.Geometry{Kind: data.GeometryPoint, Points: []geo.LatLng{{Lat: 48.8566, Lng: 2.3522}}},
	}
	p.addFeature(bdx)
	p.addFeature(paris)

	// Bbox over Bordeaux must return only the Bordeaux point.
	bbox := geo.BBox{South: 44.8, West: -0.6, North: 44.9, East: -0.5}
	fc, err := p.FetchMapLayers(t.Context(), bbox, 13)
	assert.NoError(t, err)
	assert.Len(t, fc.Features, 1)
	assert.Equal(t, "Le Cycl0", fc.Features[0].Name)

	// Wide bbox returns both.
	wide := geo.BBox{South: 44, West: -1, North: 50, East: 3}
	fc, err = p.FetchMapLayers(t.Context(), wide, 13)
	assert.NoError(t, err)
	assert.Len(t, fc.Features, 2)
}

func TestPBFFeatureVisibleZoomGating(t *testing.T) {
	// A residential road only renders at z >= 13.
	road := data.Feature{
		Tags:     data.OSMTags{"highway": "residential"},
		Geometry: data.Geometry{Kind: data.GeometryLineString, Points: []geo.LatLng{{}, {}}},
	}
	assert.False(t, pbfFeatureVisible(road, 11))
	assert.True(t, pbfFeatureVisible(road, 13))

	// A POI point hidden at z < 12.
	poi := data.Feature{
		Tags:     data.OSMTags{"amenity": "cafe"},
		Geometry: data.Geometry{Kind: data.GeometryPoint, Points: []geo.LatLng{{}}},
	}
	assert.False(t, pbfFeatureVisible(poi, 10))
	assert.True(t, pbfFeatureVisible(poi, 13))

	// A motorway always visible.
	mw := data.Feature{
		Tags:     data.OSMTags{"highway": "motorway"},
		Geometry: data.Geometry{Kind: data.GeometryLineString, Points: []geo.LatLng{{}, {}}},
	}
	assert.True(t, pbfFeatureVisible(mw, 5))
}
