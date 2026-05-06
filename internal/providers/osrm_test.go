// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const osrmSample = `{
  "code": "Ok",
  "routes": [{
    "distance": 1234.5,
    "duration": 600,
    "geometry": {"coordinates": [[-0.58,44.83],[-0.57,44.84],[-0.56,44.85]]},
    "legs": [{"steps": [
      {
        "distance": 600,
        "duration": 300,
        "geometry": {"coordinates": [[-0.58,44.83],[-0.575,44.835]]},
        "maneuver": {"type": "depart"},
        "name": "Cours de la Marne"
      },
      {
        "distance": 634.5,
        "duration": 300,
        "geometry": {"coordinates": [[-0.575,44.835],[-0.56,44.85]]},
        "maneuver": {"type": "turn", "modifier": "right"},
        "name": "Quai Richelieu"
      }
    ]}]
  }]
}`

func TestOSRMRoute(t *testing.T) {
	var calledPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path
		w.WriteHeader(200)
		_, _ = w.Write([]byte(osrmSample))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	o := NewOSRM(c, srv.URL)
	wp := []geo.LatLng{{Lat: 44.83, Lng: -0.58}, {Lat: 44.85, Lng: -0.56}}
	r, err := o.Route(context.Background(), data.ProfileDriving, wp)
	require.NoError(t, err)
	assert.InDelta(t, 1234.5, r.Distance, 1e-6)
	assert.Equal(t, 600, int(r.Duration.Seconds()))
	require.Len(t, r.Geometry, 3)
	assert.Equal(t, "depart", r.Steps[0].Instruction)
	assert.Equal(t, "turn right", r.Steps[1].Instruction)
	assert.Contains(t, calledPath, "/route/v1/driving/")
}

func TestOSRMRouteNotEnoughWaypoints(t *testing.T) {
	c := NewClient(ClientOptions{})
	o := NewOSRM(c, "https://example.invalid/")
	_, err := o.Route(context.Background(), data.ProfileDriving, []geo.LatLng{{Lat: 0, Lng: 0}})
	require.Error(t, err)
}

func TestOSRMRouteAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"code":"NoRoute","message":"impossible"}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	o := NewOSRM(c, srv.URL)
	wp := []geo.LatLng{{Lat: 1, Lng: 1}, {Lat: 2, Lng: 2}}
	_, err := o.Route(context.Background(), data.ProfileDriving, wp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NoRoute")
}

func TestToGPX(t *testing.T) {
	r := data.Route{
		Geometry: []geo.LatLng{{Lat: 44.83, Lng: -0.58}, {Lat: 44.84, Lng: -0.57}},
	}
	gpx := ToGPX(r, "test")
	assert.True(t, strings.HasPrefix(gpx, `<?xml`))
	assert.Contains(t, gpx, `lat="44.830000"`)
	assert.Contains(t, gpx, `lon="-0.570000"`)
	assert.Contains(t, gpx, `<name>test</name>`)
}

func TestToGPXEscapesName(t *testing.T) {
	r := data.Route{Geometry: nil}
	gpx := ToGPX(r, `bad "name" <>&`)
	assert.Contains(t, gpx, "&quot;")
	assert.Contains(t, gpx, "&lt;")
	assert.Contains(t, gpx, "&gt;")
	assert.Contains(t, gpx, "&amp;")
}

func TestOSRMProfileMapping(t *testing.T) {
	assert.Equal(t, "driving", osrmProfile(data.ProfileDriving))
	assert.Equal(t, "cycling", osrmProfile(data.ProfileCycling))
	assert.Equal(t, "foot", osrmProfile(data.ProfileWalking))
	assert.Equal(t, "driving", osrmProfile(""))
}
