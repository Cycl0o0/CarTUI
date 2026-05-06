// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"time"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/providers"
)

// featuresLoadedMsg is published when an Overpass fetch completes.
type featuresLoadedMsg struct {
	collection data.FeatureCollection
	err        error
}

// poisLoadedMsg is published when a category-filtered POI fetch completes.
type poisLoadedMsg struct {
	collection data.FeatureCollection
	err        error
}

// searchResultsMsg is the outcome of a Nominatim search.
type searchResultsMsg struct {
	results []providers.SearchResult
	err     error
}

// routeLoadedMsg is the outcome of an OSRM routing query.
type routeLoadedMsg struct {
	route data.Route
	err   error
}

// notificationMsg shows a transient status-line notification.
type notificationMsg struct {
	text    string
	expires time.Time
}

// notificationExpiredMsg ticks once a notification has aged out.
type notificationExpiredMsg struct{}

// debounceSearchMsg fires after the search input settles.
type debounceSearchMsg struct {
	query string
	id    int
}
