// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBBox(t *testing.T) {
	b, err := NewBBox(40, -74, 41, -73)
	require.NoError(t, err)
	assert.True(t, b.Valid())

	_, err = NewBBox(50, 0, 40, 1)
	assert.Error(t, err, "south > north should fail")

	_, err = NewBBox(0, -200, 1, 0)
	assert.Error(t, err)
}

func TestBBoxContains(t *testing.T) {
	b := BBox{South: 44, West: -1, North: 46, East: 1}
	assert.True(t, b.Contains(LatLng{45, 0}))
	assert.True(t, b.Contains(LatLng{44, -1}), "south-west corner should be inside")
	assert.True(t, b.Contains(LatLng{46, 1}), "north-east corner should be inside")
	assert.False(t, b.Contains(LatLng{43, 0}))
	assert.False(t, b.Contains(LatLng{47, 0}))
	assert.False(t, b.Contains(LatLng{45, -2}))
	assert.False(t, b.Contains(LatLng{45, 2}))
}

func TestBBoxContainsAntimeridian(t *testing.T) {
	b := BBox{South: -10, West: 170, North: 10, East: -170}
	assert.True(t, b.Contains(LatLng{0, 175}))
	assert.True(t, b.Contains(LatLng{0, -175}))
	assert.True(t, b.Contains(LatLng{0, 180}))
	assert.False(t, b.Contains(LatLng{0, 0}))
}

func TestBBoxCenterAndSpan(t *testing.T) {
	b := BBox{South: 40, West: -10, North: 50, East: 10}
	c := b.Center()
	assert.InDelta(t, 45.0, c.Lat, 1e-9)
	assert.InDelta(t, 0.0, c.Lng, 1e-9)
	latSpan, lngSpan := b.Span()
	assert.InDelta(t, 10.0, latSpan, 1e-9)
	assert.InDelta(t, 20.0, lngSpan, 1e-9)
}

func TestBBoxFromCenter(t *testing.T) {
	b := FromCenter(LatLng{0, 0}, 10, 20)
	assert.InDelta(t, -5.0, b.South, 1e-9)
	assert.InDelta(t, 5.0, b.North, 1e-9)
	assert.InDelta(t, -10.0, b.West, 1e-9)
	assert.InDelta(t, 10.0, b.East, 1e-9)
}

func TestBBoxString(t *testing.T) {
	b := BBox{South: 44.5, West: -0.5, North: 45.0, East: 0.5}
	assert.Equal(t, "44.500000,-0.500000,45.000000,0.500000", b.String())
}

func TestParseBBox(t *testing.T) {
	b, err := ParseBBox("40,-74,41,-73")
	require.NoError(t, err)
	assert.Equal(t, BBox{40, -74, 41, -73}, b)

	_, err = ParseBBox("not,a,box,here")
	assert.Error(t, err)
}
