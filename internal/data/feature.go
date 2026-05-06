// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package data contains the in-memory domain types shared between providers,
// the renderer and the TUI. None of these types should depend on any specific
// upstream API — providers normalise their payloads into these shapes.
package data

import "github.com/cycl0o0/cartui/internal/geo"

// Geometry is a tagged union covering the geometries we care about in a TUI
// renderer: a single point, a polyline, or a closed polygon (single ring).
//
// The empty Geometry is invalid — callers should always set [Geometry.Kind].
type Geometry struct {
	Kind   GeometryKind
	Points []geo.LatLng // for Point: 1 element; for LineString/Polygon: ordered ring
}

// GeometryKind enumerates the geometry types CarTUI renders.
type GeometryKind uint8

// Geometry kinds, matching a sub-set of the GeoJSON RFC 7946 set.
const (
	GeometryUnknown GeometryKind = iota
	GeometryPoint
	GeometryLineString
	GeometryPolygon
)

// Feature is a rendered map element: a geometry plus a typed tag bag.
//
// Tags follow the OpenStreetMap convention (key/value strings such as
// `highway=primary` or `amenity=cafe`). Helper methods on [OSMTags] interpret
// them.
type Feature struct {
	ID       string
	Geometry Geometry
	Tags     OSMTags
	Name     string
}

// FeatureCollection is an ordered set of features sharing the same coordinate
// reference system (always WGS84 in CarTUI).
type FeatureCollection struct {
	Features []Feature
}
