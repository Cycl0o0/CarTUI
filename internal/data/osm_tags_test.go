// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package data

import (
	"testing"

	"github.com/cycl0o0/cartui/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestRoadClassification(t *testing.T) {
	assert.Equal(t, RoadMotorway, OSMTags{"highway": "motorway"}.Road())
	assert.Equal(t, RoadMotorway, OSMTags{"highway": "trunk"}.Road())
	assert.Equal(t, RoadPrimary, OSMTags{"highway": "primary"}.Road())
	assert.Equal(t, RoadSecondary, OSMTags{"highway": "secondary"}.Road())
	assert.Equal(t, RoadResidential, OSMTags{"highway": "residential"}.Road())
	assert.Equal(t, RoadNone, OSMTags{}.Road())
	assert.Equal(t, RoadNone, OSMTags{"foo": "bar"}.Road())
}

func TestIsWater(t *testing.T) {
	assert.True(t, OSMTags{"natural": "water"}.IsWater())
	assert.True(t, OSMTags{"waterway": "river"}.IsWater())
	assert.False(t, OSMTags{}.IsWater())
}

func TestIsGreen(t *testing.T) {
	assert.True(t, OSMTags{"leisure": "park"}.IsGreen())
	assert.True(t, OSMTags{"landuse": "forest"}.IsGreen())
	assert.True(t, OSMTags{"natural": "wood"}.IsGreen())
	assert.False(t, OSMTags{}.IsGreen())
}

func TestIsBuilding(t *testing.T) {
	assert.True(t, OSMTags{"building": "yes"}.IsBuilding())
	assert.True(t, OSMTags{"building": "house"}.IsBuilding())
	assert.False(t, OSMTags{"building": "no"}.IsBuilding())
	assert.False(t, OSMTags{}.IsBuilding())
}

func TestLayer(t *testing.T) {
	assert.Equal(t, render.LayerRoadMotorway, OSMTags{"highway": "motorway"}.Layer())
	assert.Equal(t, render.LayerWater, OSMTags{"natural": "water"}.Layer())
	assert.Equal(t, render.LayerGreen, OSMTags{"leisure": "park"}.Layer())
	assert.Equal(t, render.LayerBuilding, OSMTags{"building": "yes"}.Layer())
	assert.Equal(t, render.LayerPOI, OSMTags{"amenity": "cafe"}.Layer())
}

func TestCategorizePOI(t *testing.T) {
	assert.Equal(t, POIRestaurant, CategorizePOI(OSMTags{"amenity": "restaurant"}))
	assert.Equal(t, POICafe, CategorizePOI(OSMTags{"amenity": "cafe"}))
	assert.Equal(t, POIHospital, CategorizePOI(OSMTags{"amenity": "hospital"}))
	assert.Equal(t, POIPharmacy, CategorizePOI(OSMTags{"amenity": "pharmacy"}))
	assert.Equal(t, POISchool, CategorizePOI(OSMTags{"amenity": "school"}))
	assert.Equal(t, POIShopping, CategorizePOI(OSMTags{"shop": "convenience"}))
	assert.Equal(t, POIAccommodation, CategorizePOI(OSMTags{"tourism": "hotel"}))
	assert.Equal(t, POIOther, CategorizePOI(OSMTags{}))
}

func TestPOICategoryStringAndGlyph(t *testing.T) {
	assert.NotEmpty(t, POIRestaurant.String())
	assert.NotEqual(t, rune(0), POIRestaurant.Glyph())
	assert.Equal(t, "Autre", POIOther.String())
	assert.Equal(t, '•', POIOther.Glyph())
}

func TestFormatHours(t *testing.T) {
	assert.Equal(t, "", FormatHours(""))
	assert.Equal(t, "Mo 08:00-18:00; Tu 08:00-18:00", FormatHours("Mo 08:00-18:00;Tu 08:00-18:00"))
}

func TestOSMTagsHelpers(t *testing.T) {
	tags := OSMTags{"name": "test"}
	assert.Equal(t, "test", tags.Get("name"))
	assert.Equal(t, "", tags.Get("missing"))
	assert.True(t, tags.Has("name"))
	assert.False(t, tags.Has("missing"))
}
