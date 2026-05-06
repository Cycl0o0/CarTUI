// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package data

import (
	"strings"

	"github.com/cycl0o0/cartui/internal/render"
)

// OSMTags is a tag bag in the OpenStreetMap convention: free-form key/value
// strings whose values typically belong to known enumerations.
type OSMTags map[string]string

// Get returns the value for key, or "" when absent.
func (t OSMTags) Get(key string) string { return t[key] }

// Has reports whether key is set (any non-empty value).
func (t OSMTags) Has(key string) bool { return t[key] != "" }

// RoadClass classifies the road by descending importance — used to pick the
// rendering layer.
type RoadClass uint8

// Road classification, from least to most prominent.
const (
	RoadNone RoadClass = iota
	RoadResidential
	RoadSecondary
	RoadPrimary
	RoadMotorway
)

// Road inspects highway=… and trunk/motorway tags to classify a feature as a
// road. Returns RoadNone when the feature is not a road.
func (t OSMTags) Road() RoadClass {
	switch t.Get("highway") {
	case "motorway", "motorway_link", "trunk", "trunk_link":
		return RoadMotorway
	case "primary", "primary_link":
		return RoadPrimary
	case "secondary", "secondary_link", "tertiary", "tertiary_link":
		return RoadSecondary
	case "residential", "service", "unclassified", "living_street", "pedestrian":
		return RoadResidential
	}
	return RoadNone
}

// IsWater reports whether the feature represents a body of water.
func (t OSMTags) IsWater() bool {
	if t.Has("water") || t.Has("waterway") {
		return true
	}
	switch t.Get("natural") {
	case "water", "bay", "strait", "coastline":
		return true
	}
	return false
}

// IsGreen reports whether the feature is a park, forest or other green area.
func (t OSMTags) IsGreen() bool {
	switch t.Get("leisure") {
	case "park", "garden", "nature_reserve", "common", "pitch":
		return true
	}
	switch t.Get("landuse") {
	case "forest", "grass", "meadow", "village_green", "cemetery", "recreation_ground":
		return true
	}
	switch t.Get("natural") {
	case "wood", "scrub", "grassland", "heath":
		return true
	}
	return false
}

// IsBuilding reports whether the feature is an OSM building footprint.
func (t OSMTags) IsBuilding() bool {
	v := t.Get("building")
	return v != "" && v != "no"
}

// IsBoundary reports whether the feature is an administrative boundary.
func (t OSMTags) IsBoundary() bool {
	return t.Get("boundary") == "administrative"
}

// Layer returns the renderer layer best matching the feature's tags. The
// caller still picks geometry rendering — this only colours.
func (t OSMTags) Layer() render.Layer {
	switch r := t.Road(); r {
	case RoadMotorway:
		return render.LayerRoadMotorway
	case RoadPrimary:
		return render.LayerRoadPrimary
	case RoadSecondary:
		return render.LayerRoadSecondary
	case RoadResidential:
		return render.LayerRoadResidential
	}
	switch {
	case t.IsWater():
		return render.LayerWater
	case t.IsGreen():
		return render.LayerGreen
	case t.IsBuilding():
		return render.LayerBuilding
	case t.IsBoundary():
		return render.LayerBoundary
	case t.Has("amenity"), t.Has("shop"), t.Has("tourism"):
		return render.LayerPOI
	}
	return render.LayerLabel
}

// POICategory groups amenities/shops into the categories the user can browse.
type POICategory uint8

// Categories shown in the POI menu. RawCategory keeps the OSM tag value
// when the category is "other".
const (
	POIOther POICategory = iota
	POIRestaurant
	POICafe
	POIHospital
	POIPharmacy
	POISchool
	POITransport
	POIAccommodation
	POIShopping
	POICulture
	POISport
	POIPublicService
)

// String returns the human label for the category.
func (c POICategory) String() string {
	switch c {
	case POIRestaurant:
		return "Restaurant"
	case POICafe:
		return "Café"
	case POIHospital:
		return "Hôpital"
	case POIPharmacy:
		return "Pharmacie"
	case POISchool:
		return "École"
	case POITransport:
		return "Transport"
	case POIAccommodation:
		return "Hébergement"
	case POIShopping:
		return "Shopping"
	case POICulture:
		return "Culture"
	case POISport:
		return "Sport"
	case POIPublicService:
		return "Service public"
	}
	return "Autre"
}

// Glyph returns a single rune used as a marker on the map for this category.
func (c POICategory) Glyph() rune {
	switch c {
	case POIRestaurant:
		return '🍽'
	case POICafe:
		return '☕'
	case POIHospital:
		return '🏥'
	case POIPharmacy:
		return '💊'
	case POISchool:
		return '🏫'
	case POITransport:
		return '🚉'
	case POIAccommodation:
		return '🏨'
	case POIShopping:
		return '🛒'
	case POICulture:
		return '🏛'
	case POISport:
		return '⚽'
	case POIPublicService:
		return '🏛'
	}
	return '•'
}

// CategorizePOI inspects amenity/shop/tourism/leisure tags and returns the
// matching category.
func CategorizePOI(t OSMTags) POICategory {
	switch t.Get("amenity") {
	case "restaurant", "fast_food", "food_court", "bar", "pub":
		return POIRestaurant
	case "cafe", "biergarten":
		return POICafe
	case "hospital", "clinic", "doctors":
		return POIHospital
	case "pharmacy":
		return POIPharmacy
	case "school", "kindergarten", "university", "college", "library":
		return POISchool
	case "bus_station", "ferry_terminal", "taxi", "bicycle_rental", "car_rental":
		return POITransport
	case "townhall", "courthouse", "post_office", "police", "fire_station", "embassy":
		return POIPublicService
	case "theatre", "cinema", "arts_centre", "museum":
		return POICulture
	}
	switch t.Get("shop") {
	case "":
	default:
		return POIShopping
	}
	switch t.Get("tourism") {
	case "hotel", "hostel", "guest_house", "motel", "apartment", "camp_site":
		return POIAccommodation
	case "museum", "attraction", "gallery", "viewpoint":
		return POICulture
	}
	switch t.Get("leisure") {
	case "stadium", "sports_centre", "fitness_centre", "swimming_pool", "pitch":
		return POISport
	}
	return POIOther
}

// FormatHours normalises the OSM `opening_hours` value for display.
func FormatHours(raw string) string {
	if raw == "" {
		return ""
	}
	return strings.ReplaceAll(raw, ";", "; ")
}
