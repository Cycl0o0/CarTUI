// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package pbf

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVarintRoundTrip(t *testing.T) {
	cases := []uint64{0, 1, 127, 128, 16383, 16384, 1<<32 - 1, 1 << 56}
	for _, v := range cases {
		buf := EncodeVarint(nil, v)
		got, err := New(buf).Varint()
		require.NoError(t, err)
		assert.Equal(t, v, got)
	}
}

func TestZigZagDecode(t *testing.T) {
	assert.Equal(t, int64(0), ZigZagDecode(0))
	assert.Equal(t, int64(-1), ZigZagDecode(1))
	assert.Equal(t, int64(1), ZigZagDecode(2))
	assert.Equal(t, int64(-2), ZigZagDecode(3))
	assert.Equal(t, int64(2147483647), ZigZagDecode(4294967294))
}

func TestTagAndSkip(t *testing.T) {
	// Build a fake message: field 1 (varint=42), field 2 (string="hi"), field 3 (fixed32=1).
	var buf []byte
	buf = EncodeVarint(buf, (1<<3)|WireVarint)
	buf = EncodeVarint(buf, 42)
	buf = EncodeVarint(buf, (2<<3)|WireBytes)
	buf = EncodeVarint(buf, 2)
	buf = append(buf, 'h', 'i')
	buf = EncodeVarint(buf, (3<<3)|Wire32bit)
	buf = append(buf, 0x01, 0x00, 0x00, 0x00)

	r := New(buf)

	// Field 1: read.
	field, wire, err := r.Tag()
	require.NoError(t, err)
	assert.Equal(t, 1, field)
	assert.Equal(t, WireVarint, wire)
	v, err := r.Varint()
	require.NoError(t, err)
	assert.Equal(t, uint64(42), v)

	// Field 2: skip.
	field, wire, err = r.Tag()
	require.NoError(t, err)
	assert.Equal(t, 2, field)
	require.NoError(t, r.Skip(wire))

	// Field 3: read fixed32.
	field, wire, err = r.Tag()
	require.NoError(t, err)
	assert.Equal(t, 3, field)
	assert.Equal(t, Wire32bit, wire)
	x, err := r.Fixed32()
	require.NoError(t, err)
	assert.Equal(t, uint32(1), x)

	_, _, err = r.Tag()
	assert.ErrorIs(t, err, io.EOF)
}

func TestTruncated(t *testing.T) {
	_, err := New([]byte{0x80}).Varint()
	assert.ErrorIs(t, err, ErrTruncated)
}

func TestPackedVarint(t *testing.T) {
	// 3 packed varints: 1, 2, 130
	var inner []byte
	inner = EncodeVarint(inner, 1)
	inner = EncodeVarint(inner, 2)
	inner = EncodeVarint(inner, 130)

	var buf []byte
	buf = EncodeVarint(buf, uint64(len(inner)))
	buf = append(buf, inner...)

	r := New(buf)
	got, err := r.PackedVarint()
	require.NoError(t, err)
	assert.Equal(t, []uint64{1, 2, 130}, got)
}
