// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"testing"

	"github.com/cycl0o0/cartui/internal/pbf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZxyToIDOrigin(t *testing.T) {
	// Tile 0/0/0 must map to id 0.
	assert.Equal(t, uint64(0), ZxyToID(0, 0, 0))
}

func TestZxyToIDZoom1(t *testing.T) {
	// At zoom 1 the four tiles are at ids 1..4 in Hilbert order.
	ids := map[[2]uint32]uint64{
		{0, 0}: 1,
		{0, 1}: 2,
		{1, 1}: 3,
		{1, 0}: 4,
	}
	for xy, want := range ids {
		got := ZxyToID(1, xy[0], xy[1])
		assert.Equal(t, want, got, "zxy=1/%d/%d", xy[0], xy[1])
	}
}

func TestDeserializeHeaderRejectsBadMagic(t *testing.T) {
	b := make([]byte, HeaderSize)
	copy(b, "BadHdr!")
	_, err := DeserializeHeader(b)
	assert.Error(t, err)
}

func TestDeserializeHeaderHappyPath(t *testing.T) {
	b := make([]byte, HeaderSize)
	copy(b, "PMTiles")
	b[7] = 3
	// Min/Max zoom for sanity.
	b[100] = 2
	b[101] = 14
	b[97] = CompressGzip
	b[98] = CompressGzip
	b[99] = TileTypeMVT

	h, err := DeserializeHeader(b)
	require.NoError(t, err)
	assert.Equal(t, uint8(2), h.MinZoom)
	assert.Equal(t, uint8(14), h.MaxZoom)
	assert.Equal(t, uint8(CompressGzip), h.InternalCompress)
	assert.Equal(t, uint8(TileTypeMVT), h.TileType)
}

func TestDeserializeEntries(t *testing.T) {
	// Build a minimal directory blob: 2 entries.
	// tile_ids: 1 (delta 1), 5 (delta 4)
	// run_lengths: 1, 1
	// lengths: 100, 200
	// offsets: 0 (consecutive after start), 0 (consecutive)
	var buf []byte
	buf = pbf.EncodeVarint(buf, 2) // count

	buf = pbf.EncodeVarint(buf, 1) // tile_id delta
	buf = pbf.EncodeVarint(buf, 4)

	buf = pbf.EncodeVarint(buf, 1) // run_lengths
	buf = pbf.EncodeVarint(buf, 1)

	buf = pbf.EncodeVarint(buf, 100) // lengths
	buf = pbf.EncodeVarint(buf, 200)

	buf = pbf.EncodeVarint(buf, 0) // offsets — first is 0
	buf = pbf.EncodeVarint(buf, 0) // second 0 means "right after previous"

	entries, err := DeserializeEntries(buf)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, uint64(1), entries[0].TileID)
	assert.Equal(t, uint32(1), entries[0].RunLength)
	assert.Equal(t, uint32(100), entries[0].Length)
	assert.Equal(t, uint64(0), entries[0].Offset)

	assert.Equal(t, uint64(5), entries[1].TileID)
	assert.Equal(t, uint64(100), entries[1].Offset, "consecutive layout")
}

func TestFindEntryRespectsRunLength(t *testing.T) {
	entries := []EntryV3{
		{TileID: 0, Offset: 0, Length: 10, RunLength: 5},
		{TileID: 10, Offset: 100, Length: 10, RunLength: 1},
	}
	// Inside the first run: tile id 3 must hit entry 0.
	e, ok := findEntry(entries, 3)
	require.True(t, ok)
	assert.Equal(t, uint64(0), e.TileID)

	// Just past the first run, before the second entry: not found.
	_, ok = findEntry(entries, 7)
	assert.False(t, ok)

	// Exactly the second entry.
	e, ok = findEntry(entries, 10)
	require.True(t, ok)
	assert.Equal(t, uint64(10), e.TileID)
}
