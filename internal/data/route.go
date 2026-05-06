// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package data

import (
	"time"

	"github.com/cycl0o0/cartui/internal/geo"
)

// RouteProfile selects the routing model — voiture/vélo/piéton.
type RouteProfile string

// Built-in profiles. Backed by OSRM's `driving`, `cycling`, `foot` profiles
// on `router.project-osrm.org`.
const (
	ProfileDriving RouteProfile = "driving"
	ProfileCycling RouteProfile = "cycling"
	ProfileWalking RouteProfile = "walking"
)

// Route is the result of an OSRM (or compatible) routing query.
type Route struct {
	Distance float64       // metres
	Duration time.Duration // travel time at the profile's typical speed
	Geometry []geo.LatLng  // ordered polyline, ready to draw
	Steps    []RouteStep
	Profile  RouteProfile
}

// RouteStep is a single turn-by-turn instruction.
type RouteStep struct {
	Instruction string
	Distance    float64 // metres
	Duration    time.Duration
	Geometry    []geo.LatLng
}
