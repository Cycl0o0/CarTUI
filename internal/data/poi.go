// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package data

import (
	"time"

	"github.com/cycl0o0/cartui/internal/geo"
)

// POI is a normalised point-of-interest, ready to render in the side panel.
type POI struct {
	ID       string
	Name     string
	Position geo.LatLng
	Category POICategory
	Tags     OSMTags

	// Optional, all derived from OSM tags when present.
	Address string
	Phone   string
	Website string
	Hours   string
}

// Bookmark is a user-saved location.
type Bookmark struct {
	ID        string
	Name      string
	Position  geo.LatLng
	Notes     string
	CreatedAt time.Time
}

// HistoryEntry records a search event so it can be re-played later.
type HistoryEntry struct {
	Query     string
	Position  geo.LatLng // 0,0 when the search did not resolve
	CreatedAt time.Time
}
