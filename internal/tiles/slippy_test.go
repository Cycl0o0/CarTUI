// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tiles

import (
	"testing"

	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
)

func TestAddressURL(t *testing.T) {
	a := Address{Z: 13, X: 4084, Y: 2952}
	got := a.URL("https://tile.openstreetmap.org/{z}/{x}/{y}.png")
	assert.Equal(t, "https://tile.openstreetmap.org/13/4084/2952.png", got)
	assert.Equal(t, "13/4084/2952", a.Key())
}

func TestCoveringTilesSingle(t *testing.T) {
	bbox := geo.BBox{South: 44.83, West: -0.59, North: 44.84, East: -0.58}
	tiles := CoveringTiles(bbox, 13)
	assert.NotEmpty(t, tiles)
	for _, ti := range tiles {
		assert.Equal(t, 13, ti.Z)
	}
}

func TestCoveringTilesAreaIsRectangular(t *testing.T) {
	bbox := geo.BBox{South: 44.0, West: -1.0, North: 45.0, East: 0.0}
	tiles := CoveringTiles(bbox, 8)
	assert.GreaterOrEqual(t, len(tiles), 1)
}
