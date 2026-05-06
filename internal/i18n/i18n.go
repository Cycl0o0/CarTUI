// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package i18n holds CarTUI user-facing strings. The translations are kept
// inline (no resource bundles) to keep the binary self-contained and
// simplify static analysis. Currently French (default) and English.
package i18n

import "strings"

// Language identifies a translation set.
type Language string

// Built-in languages.
const (
	French  Language = "fr"
	English Language = "en"
)

// Strings is the bag of translatable labels rendered by the TUI.
type Strings struct {
	AppName    string
	ModeNormal string
	ModeSearch string
	ModeRoute  string
	ModePOI    string
	ModeHelp   string

	Search        string
	SearchHint    string
	Loading       string
	NoResults     string
	NetworkError  string
	OfflineNotice string

	Bookmarks       string
	Bookmark        string
	BookmarkAdded   string
	BookmarkRemoved string

	Route       string
	Distance    string
	Duration    string
	Profile     string
	StartPoint  string
	EndPoint    string
	NoRoute     string
	GPXExported string

	POI            string
	Restaurant     string
	Cafe           string
	Hospital       string
	Pharmacy       string
	School         string
	Transport      string
	Accommodation  string
	Shopping       string
	Culture        string
	Sport          string
	PublicService  string

	HelpTitle string
	Quit      string
	Center    string
	Reset     string
	ZoomIn    string
	ZoomOut   string
	PanLeft   string
	PanDown   string
	PanUp     string
	PanRight  string

	StatusZoom    string
	StatusCenter  string
	StatusScale   string
	StatusOffline string
	StatusOnline  string
}

// frenchStrings is the default translation.
var frenchStrings = Strings{
	AppName:         "CarTUI",
	ModeNormal:      "NORMAL",
	ModeSearch:      "RECHERCHE",
	ModeRoute:       "ITINÉRAIRE",
	ModePOI:         "POI",
	ModeHelp:        "AIDE",
	Search:          "Rechercher",
	SearchHint:      "Tapez un lieu, une ville, une adresse…",
	Loading:         "Chargement…",
	NoResults:       "Aucun résultat",
	NetworkError:    "Erreur réseau",
	OfflineNotice:   "Mode hors-ligne — données en cache",
	Bookmarks:       "Favoris",
	Bookmark:        "Favori",
	BookmarkAdded:   "Favori ajouté",
	BookmarkRemoved: "Favori supprimé",
	Route:           "Itinéraire",
	Distance:        "Distance",
	Duration:        "Durée",
	Profile:         "Profil",
	StartPoint:      "Départ",
	EndPoint:        "Arrivée",
	NoRoute:         "Aucun itinéraire trouvé",
	GPXExported:     "GPX exporté",
	POI:             "Points d'intérêt",
	Restaurant:      "Restaurants",
	Cafe:            "Cafés",
	Hospital:        "Hôpitaux",
	Pharmacy:        "Pharmacies",
	School:          "Écoles",
	Transport:       "Transports",
	Accommodation:   "Hébergement",
	Shopping:        "Shopping",
	Culture:         "Culture",
	Sport:           "Sports",
	PublicService:   "Services publics",
	HelpTitle:       "Aide — Raccourcis clavier",
	Quit:            "Quitter",
	Center:          "Centrer",
	Reset:           "Réinitialiser la vue",
	ZoomIn:          "Zoom avant",
	ZoomOut:         "Zoom arrière",
	PanLeft:         "Déplacer à gauche",
	PanDown:         "Déplacer en bas",
	PanUp:           "Déplacer en haut",
	PanRight:        "Déplacer à droite",
	StatusZoom:      "Zoom",
	StatusCenter:    "Centre",
	StatusScale:     "Échelle",
	StatusOffline:   "Hors-ligne",
	StatusOnline:    "En ligne",
}

// englishStrings holds the English translation.
var englishStrings = Strings{
	AppName:         "CarTUI",
	ModeNormal:      "NORMAL",
	ModeSearch:      "SEARCH",
	ModeRoute:       "ROUTE",
	ModePOI:         "POI",
	ModeHelp:        "HELP",
	Search:          "Search",
	SearchHint:      "Type a place, city or address…",
	Loading:         "Loading…",
	NoResults:       "No results",
	NetworkError:    "Network error",
	OfflineNotice:   "Offline — using cached data",
	Bookmarks:       "Bookmarks",
	Bookmark:        "Bookmark",
	BookmarkAdded:   "Bookmark added",
	BookmarkRemoved: "Bookmark removed",
	Route:           "Route",
	Distance:        "Distance",
	Duration:        "Duration",
	Profile:         "Profile",
	StartPoint:      "Start",
	EndPoint:        "End",
	NoRoute:         "No route found",
	GPXExported:     "GPX exported",
	POI:             "Points of interest",
	Restaurant:      "Restaurants",
	Cafe:            "Cafés",
	Hospital:        "Hospitals",
	Pharmacy:        "Pharmacies",
	School:          "Schools",
	Transport:       "Transport",
	Accommodation:   "Accommodation",
	Shopping:        "Shopping",
	Culture:         "Culture",
	Sport:           "Sports",
	PublicService:   "Public services",
	HelpTitle:       "Help — Keyboard shortcuts",
	Quit:            "Quit",
	Center:          "Center",
	Reset:           "Reset view",
	ZoomIn:          "Zoom in",
	ZoomOut:         "Zoom out",
	PanLeft:         "Pan left",
	PanDown:         "Pan down",
	PanUp:           "Pan up",
	PanRight:        "Pan right",
	StatusZoom:      "Zoom",
	StatusCenter:    "Center",
	StatusScale:     "Scale",
	StatusOffline:   "Offline",
	StatusOnline:    "Online",
}

// For returns the translation set for the given language; defaults to French
// for unknown values.
func For(lang string) Strings {
	switch strings.ToLower(lang) {
	case string(English), "en-us", "en-gb":
		return englishStrings
	default:
		return frenchStrings
	}
}
