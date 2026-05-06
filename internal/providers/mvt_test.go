// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"testing"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/pbf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildLineFeature constructs a minimal MVT feature blob: a LINESTRING
// from (0,0) to (4096,4096) with one string tag (key index 0, value 0).
func buildLineFeature(t *testing.T) []byte {
	t.Helper()
	var f []byte

	// field 2: tags (packed varint) [0, 0]
	tagsHeader := byte((2 << 3) | pbf.WireBytes)
	f = append(f, tagsHeader)
	tagsBytes := []byte{0, 0}
	f = pbf.EncodeVarint(f, uint64(len(tagsBytes)))
	f = append(f, tagsBytes...)

	// field 3: type = LINESTRING (varint=2)
	f = append(f, byte((3<<3)|pbf.WireVarint))
	f = pbf.EncodeVarint(f, mvtGeomLineString)

	// field 4: geometry — packed varint
	// MoveTo(1) at (0,0): cmd=9, dx=0, dy=0 -> [9, 0, 0]
	// LineTo(1) at (4096,4096): cmd=10, dx=8192, dy=8192 (zigzag)
	// Zigzag(8192) = 16384.
	geom := []byte{}
	geom = pbf.EncodeVarint(geom, (1<<3)|cmdMoveTo) // count=1
	geom = pbf.EncodeVarint(geom, 0)                // dx=0
	geom = pbf.EncodeVarint(geom, 0)                // dy=0
	geom = pbf.EncodeVarint(geom, (1<<3)|cmdLineTo) // count=1
	geom = pbf.EncodeVarint(geom, 16384)            // dx zigzag-encoded
	geom = pbf.EncodeVarint(geom, 16384)            // dy

	f = append(f, byte((4<<3)|pbf.WireBytes))
	f = pbf.EncodeVarint(f, uint64(len(geom)))
	f = append(f, geom...)

	return f
}

// buildLayer wraps a feature blob in a Layer message.
func buildLayer(t *testing.T, name string, feat []byte, key string, val string) []byte {
	t.Helper()
	var l []byte

	// field 1: name
	l = append(l, byte((1<<3)|pbf.WireBytes))
	l = pbf.EncodeVarint(l, uint64(len(name)))
	l = append(l, name...)

	// field 2: feature
	l = append(l, byte((2<<3)|pbf.WireBytes))
	l = pbf.EncodeVarint(l, uint64(len(feat)))
	l = append(l, feat...)

	// field 3: key (string)
	l = append(l, byte((3<<3)|pbf.WireBytes))
	l = pbf.EncodeVarint(l, uint64(len(key)))
	l = append(l, key...)

	// field 4: value (Value message with field 1 = string)
	var valueMsg []byte
	valueMsg = append(valueMsg, byte((1<<3)|pbf.WireBytes))
	valueMsg = pbf.EncodeVarint(valueMsg, uint64(len(val)))
	valueMsg = append(valueMsg, val...)

	l = append(l, byte((4<<3)|pbf.WireBytes))
	l = pbf.EncodeVarint(l, uint64(len(valueMsg)))
	l = append(l, valueMsg...)

	// field 5: extent (4096)
	l = append(l, byte((5<<3)|pbf.WireVarint))
	l = pbf.EncodeVarint(l, 4096)

	return l
}

// buildTile wraps a layer blob in a Tile message.
func buildTile(t *testing.T, layer []byte) []byte {
	t.Helper()
	var tile []byte
	tile = append(tile, byte((3<<3)|pbf.WireBytes))
	tile = pbf.EncodeVarint(tile, uint64(len(layer)))
	tile = append(tile, layer...)
	return tile
}

func TestDecodeMVTLineString(t *testing.T) {
	feat := buildLineFeature(t)
	layer := buildLayer(t, "roads", feat, "kind", "highway")
	tile := buildTile(t, layer)

	fc, err := DecodeMVT(tile, 13, 4084, 2952)
	require.NoError(t, err)
	require.Len(t, fc.Features, 1)

	f := fc.Features[0]
	assert.Equal(t, data.GeometryLineString, f.Geometry.Kind)
	assert.Equal(t, "highway", f.Tags["kind"])
	assert.Equal(t, "roads", f.Tags["__layer"])
	assert.Len(t, f.Geometry.Points, 2)
}

func TestRewritePMFeatureRoads(t *testing.T) {
	f := data.Feature{Tags: data.OSMTags{"__layer": "roads", "kind": "highway"}}
	rewritePMFeature(&f)
	assert.Equal(t, "motorway", f.Tags["highway"])

	f = data.Feature{Tags: data.OSMTags{"__layer": "water"}}
	rewritePMFeature(&f)
	assert.Equal(t, "water", f.Tags["natural"])

	f = data.Feature{Tags: data.OSMTags{"__layer": "buildings"}}
	rewritePMFeature(&f)
	assert.Equal(t, "yes", f.Tags["building"])
}
