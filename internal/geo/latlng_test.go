// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package geo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLatLng(t *testing.T) {
	tests := []struct {
		name    string
		lat     float64
		lng     float64
		wantErr bool
	}{
		{"bordeaux", 44.8378, -0.5792, false},
		{"north pole", 90, 0, false},
		{"south pole", -90, 0, false},
		{"antimeridian east", 0, 180, false},
		{"antimeridian west", 0, -180, false},
		{"lat too high", 91, 0, true},
		{"lat too low", -91, 0, true},
		{"lng too east", 0, 181, true},
		{"lng too west", 0, -181, true},
		{"nan lat", math.NaN(), 0, true},
		{"inf lng", 0, math.Inf(1), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ll, err := NewLatLng(tc.lat, tc.lng)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.lat, ll.Lat)
			assert.Equal(t, tc.lng, ll.Lng)
			assert.True(t, ll.Valid())
		})
	}
}

func TestLatLngString(t *testing.T) {
	ll := LatLng{Lat: 44.8378, Lng: -0.5792}
	assert.Equal(t, "44.837800,-0.579200", ll.String())
}

func TestParseLatLng(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LatLng
		wantErr bool
	}{
		{"comma", "44.8378,-0.5792", LatLng{44.8378, -0.5792}, false},
		{"spaces", " 44.8378 , -0.5792 ", LatLng{44.8378, -0.5792}, false},
		{"missing comma", "44.8378", LatLng{}, true},
		{"too many", "1,2,3", LatLng{}, true},
		{"bad number", "abc,def", LatLng{}, true},
		{"out of range", "100,0", LatLng{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLatLng(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.want.Lat, got.Lat, 1e-9)
			assert.InDelta(t, tc.want.Lng, got.Lng, 1e-9)
		})
	}
}

func TestClampLat(t *testing.T) {
	assert.Equal(t, MaxMercatorLat, ClampLat(90))
	assert.Equal(t, -MaxMercatorLat, ClampLat(-90))
	assert.Equal(t, 45.0, ClampLat(45))
}

func TestNormalizeLng(t *testing.T) {
	assert.InDelta(t, 0.0, NormalizeLng(360), 1e-9)
	assert.InDelta(t, -179.0, NormalizeLng(181), 1e-9)
	assert.InDelta(t, 179.0, NormalizeLng(-181), 1e-9)
	assert.InDelta(t, 45.0, NormalizeLng(45), 1e-9)
}
