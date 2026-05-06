// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"fmt"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/pbf"
)

// Mapbox Vector Tile spec v2:
//   https://github.com/mapbox/vector-tile-spec/tree/master/2.1
//
// CarTUI only consumes a small subset of the format:
//   - Tile { repeated Layer layers = 3 }
//   - Layer { name=1, features=2, keys=3, values=4, extent=5, version=15 }
//   - Feature { id=1, tags=2 (packed varint), type=3, geometry=4 (packed) }
//   - Value { string=1, float=2, double=3, int=4, uint=5, sint=6, bool=7 }
//
// The "extent" is the resolution of the tile-local coordinate system
// (default 4096 × 4096); top-left is (0,0).

// MVT geometry types.
const (
	mvtGeomUnknown    = 0
	mvtGeomPoint      = 1
	mvtGeomLineString = 2
	mvtGeomPolygon    = 3
)

// MVT geometry commands.
const (
	cmdMoveTo    = 1
	cmdLineTo    = 2
	cmdClosePath = 7
)

// DecodeMVT decodes a Mapbox Vector Tile blob and returns the contained
// features in CarTUI's normalised shape, projected back to WGS84 using
// the tile's slippy-map address.
func DecodeMVT(blob []byte, z, x, y int) (data.FeatureCollection, error) {
	r := pbf.New(blob)
	var fc data.FeatureCollection

	for !r.Done() {
		field, wire, err := r.Tag()
		if err != nil {
			return fc, fmt.Errorf("mvt: tag: %w", err)
		}
		if field == 3 && wire == pbf.WireBytes {
			layerBlob, err := r.Bytes()
			if err != nil {
				return fc, fmt.Errorf("mvt: layer bytes: %w", err)
			}
			if err := decodeMVTLayer(layerBlob, z, x, y, &fc); err != nil {
				return fc, err
			}
			continue
		}
		if err := r.Skip(wire); err != nil {
			return fc, err
		}
	}
	return fc, nil
}

// decodeMVTLayer parses a single Layer message and appends its features
// to fc.
func decodeMVTLayer(blob []byte, z, x, y int, fc *data.FeatureCollection) error {
	r := pbf.New(blob)

	var (
		name     string
		extent   uint32 = 4096
		keys     []string
		values   []any
		features [][]byte
	)

	for !r.Done() {
		field, wire, err := r.Tag()
		if err != nil {
			return fmt.Errorf("mvt layer: %w", err)
		}
		switch {
		case field == 1 && wire == pbf.WireBytes:
			name, err = r.String()
			if err != nil {
				return err
			}
		case field == 2 && wire == pbf.WireBytes:
			b, err := r.Bytes()
			if err != nil {
				return err
			}
			features = append(features, append([]byte(nil), b...))
		case field == 3 && wire == pbf.WireBytes:
			s, err := r.String()
			if err != nil {
				return err
			}
			keys = append(keys, s)
		case field == 4 && wire == pbf.WireBytes:
			vb, err := r.Bytes()
			if err != nil {
				return err
			}
			v, err := decodeMVTValue(vb)
			if err != nil {
				return err
			}
			values = append(values, v)
		case field == 5 && wire == pbf.WireVarint:
			v, err := r.Varint()
			if err != nil {
				return err
			}
			extent = uint32(v)
		default:
			if err := r.Skip(wire); err != nil {
				return err
			}
		}
	}

	for _, fbuf := range features {
		f, err := decodeMVTFeature(fbuf, z, x, y, extent, keys, values, name)
		if err != nil {
			continue // tolerate per-feature decoding errors
		}
		if f.Geometry.Kind != data.GeometryUnknown {
			fc.Features = append(fc.Features, f)
		}
	}
	return nil
}

func decodeMVTValue(blob []byte) (any, error) {
	r := pbf.New(blob)
	for !r.Done() {
		field, wire, err := r.Tag()
		if err != nil {
			return nil, err
		}
		switch field {
		case 1:
			return r.String()
		case 2:
			return r.Float()
		case 3:
			return r.Double()
		case 4:
			v, err := r.Varint()
			return int64(v), err
		case 5:
			return r.Varint()
		case 6:
			return r.Sint()
		case 7:
			return r.Bool()
		default:
			if err := r.Skip(wire); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

func decodeMVTFeature(blob []byte, z, x, y int, extent uint32, keys []string, values []any, layerName string) (data.Feature, error) {
	r := pbf.New(blob)

	var (
		tags    []uint64
		geomCmd []uint64
		gType   uint32
	)

	for !r.Done() {
		field, wire, err := r.Tag()
		if err != nil {
			return data.Feature{}, err
		}
		switch {
		case field == 2:
			tags, err = r.PackedVarint()
			if err != nil {
				return data.Feature{}, err
			}
		case field == 3 && wire == pbf.WireVarint:
			v, err := r.Varint()
			if err != nil {
				return data.Feature{}, err
			}
			gType = uint32(v)
		case field == 4:
			geomCmd, err = r.PackedVarint()
			if err != nil {
				return data.Feature{}, err
			}
		default:
			if err := r.Skip(wire); err != nil {
				return data.Feature{}, err
			}
		}
	}

	osmTags := make(data.OSMTags, len(tags)/2)
	osmTags["__layer"] = layerName // surface the MVT layer name as a synthetic tag
	for i := 0; i+1 < len(tags); i += 2 {
		ki := int(tags[i])
		vi := int(tags[i+1])
		if ki >= len(keys) || vi >= len(values) {
			continue
		}
		osmTags[keys[ki]] = stringOf(values[vi])
	}

	rings := decodeGeometry(geomCmd)
	geom := geometryFromRings(rings, gType, z, x, y, extent)

	feat := data.Feature{
		ID:       fmt.Sprintf("%s/%d/%d/%d", layerName, z, x, y),
		Geometry: geom,
		Tags:     osmTags,
		Name:     stringOf(osmTags["name"]),
	}
	return feat, nil
}

// decodeGeometry walks the command stream and returns rings: each
// ring is a slice of integer (x, y) tile-local coordinates. Multiple
// MoveTo commands open new rings.
func decodeGeometry(cmd []uint64) [][][2]int32 {
	var (
		rings   [][][2]int32
		current [][2]int32
		x, y    int32
	)

	i := 0
	for i < len(cmd) {
		header := cmd[i]
		i++
		op := header & 0x7
		count := int(header >> 3)
		switch op {
		case cmdMoveTo:
			for k := 0; k < count && i+1 < len(cmd); k++ {
				dx := int32(pbf.ZigZagDecode(cmd[i]))
				dy := int32(pbf.ZigZagDecode(cmd[i+1]))
				i += 2
				x += dx
				y += dy
				if len(current) > 0 {
					rings = append(rings, current)
				}
				current = [][2]int32{{x, y}}
			}
		case cmdLineTo:
			for k := 0; k < count && i+1 < len(cmd); k++ {
				dx := int32(pbf.ZigZagDecode(cmd[i]))
				dy := int32(pbf.ZigZagDecode(cmd[i+1]))
				i += 2
				x += dx
				y += dy
				current = append(current, [2]int32{x, y})
			}
		case cmdClosePath:
			if len(current) > 0 {
				current = append(current, current[0]) // close ring explicitly
			}
		}
	}
	if len(current) > 0 {
		rings = append(rings, current)
	}
	return rings
}

// geometryFromRings projects tile-local coordinates back to WGS84 and
// builds a [data.Geometry] consistent with the MVT geometry type.
func geometryFromRings(rings [][][2]int32, gType uint32, z, x, y int, extent uint32) data.Geometry {
	if len(rings) == 0 || extent == 0 {
		return data.Geometry{}
	}
	switch gType {
	case mvtGeomPoint:
		// Each "ring" of one element is a separate point. We emit only
		// the first; multipoints are rare in MVT.
		if len(rings[0]) == 0 {
			return data.Geometry{}
		}
		p := tileCoordToLatLng(rings[0][0][0], rings[0][0][1], z, x, y, extent)
		return data.Geometry{Kind: data.GeometryPoint, Points: []geo.LatLng{p}}
	case mvtGeomLineString:
		// Concatenate every ring into a single polyline (acceptable for
		// rendering — keeps the feature count low).
		var pts []geo.LatLng
		for _, ring := range rings {
			for _, c := range ring {
				pts = append(pts, tileCoordToLatLng(c[0], c[1], z, x, y, extent))
			}
		}
		if len(pts) < 2 {
			return data.Geometry{}
		}
		return data.Geometry{Kind: data.GeometryLineString, Points: pts}
	case mvtGeomPolygon:
		// Use the largest ring (outer); skip holes — fine for a TUI.
		var biggest [][2]int32
		for _, ring := range rings {
			if len(ring) > len(biggest) {
				biggest = ring
			}
		}
		if len(biggest) < 3 {
			return data.Geometry{}
		}
		pts := make([]geo.LatLng, len(biggest))
		for i, c := range biggest {
			pts[i] = tileCoordToLatLng(c[0], c[1], z, x, y, extent)
		}
		return data.Geometry{Kind: data.GeometryPolygon, Points: pts}
	}
	return data.Geometry{}
}

// tileCoordToLatLng converts a (tx, ty) coordinate inside the tile (in
// extent units) to WGS84.
func tileCoordToLatLng(tx, ty int32, z, x, y int, extent uint32) geo.LatLng {
	worldPx := float64(geo.TileSize) * float64(int(1)<<z)
	pxPerExtent := float64(geo.TileSize) / float64(extent)
	px := float64(x*geo.TileSize) + float64(tx)*pxPerExtent
	py := float64(y*geo.TileSize) + float64(ty)*pxPerExtent
	if px > worldPx {
		px = worldPx
	}
	if py > worldPx {
		py = worldPx
	}
	return geo.WorldPixelToLatLng(px, py, z)
}

// stringOf converts an MVT value into a string (used to fold typed
// values into our [data.OSMTags] which are string-only).
func stringOf(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "yes"
		}
		return "no"
	case int64:
		return fmt.Sprintf("%d", x)
	case uint64:
		return fmt.Sprintf("%d", x)
	case float32:
		return fmt.Sprintf("%g", x)
	case float64:
		return fmt.Sprintf("%g", x)
	}
	return fmt.Sprintf("%v", v)
}
