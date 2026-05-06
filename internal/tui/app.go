// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/cartui/internal/config"
	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/i18n"
	"github.com/cycl0o0/cartui/internal/providers"
	"github.com/cycl0o0/cartui/internal/render"
	"github.com/cycl0o0/cartui/internal/store"
	"github.com/cycl0o0/cartui/internal/version"
)

// Mode is the current top-level interaction mode.
type Mode uint8

// Modes available to the app.
const (
	ModeNormal Mode = iota
	ModeSearch
	ModeRoute
	ModePOI
	ModeHelp
	ModeMeasure
	ModeBookmarks
)

// Deps groups every dependency the [App] needs. Each is optional in tests —
// nil values disable the corresponding functionality cleanly.
type Deps struct {
	Cfg       config.Config
	Store     *store.DB
	Nominatim *providers.Nominatim
	Overpass  *providers.Overpass
	OSRM      *providers.OSRM
	TomTom    *providers.TomTom // nil when no API key is configured
}

// App is the root Bubble Tea model.
type App struct {
	deps    Deps
	keys    Keymap
	t       i18n.Strings
	theme   render.Theme
	ascii   bool
	mode    Mode
	prevKey string

	width  int
	height int

	viewport Viewport

	features data.FeatureCollection
	pois     data.FeatureCollection
	markers  []geo.LatLng
	route    *data.Route

	search    searchModel
	poi       poiModel
	rt        routeModel
	measure   measureModel
	bookmarks bookmarkModel
	gps       gpsState
	traffic   trafficState

	heatmap bool

	// layerDebounceID is incremented every time the viewport changes;
	// only the most recent tick passes through layerDebounceMsg, so
	// rapid pan/zoom keystrokes coalesce into a single fetch.
	layerDebounceID int

	notification    string
	notificationExp time.Time

	sidebar bool

	bgCtx context.Context
}

// New builds the root model.
func New(deps Deps) *App {
	t := i18n.For(deps.Cfg.UI.Lang)
	theme := render.ThemeByName(deps.Cfg.UI.Theme)

	center := geo.LatLng{Lat: deps.Cfg.Map.DefaultLat, Lng: deps.Cfg.Map.DefaultLng}
	zoom := deps.Cfg.Map.DefaultZoom
	if deps.Store != nil {
		if v, err := deps.Store.LoadViewport(); err == nil {
			center = v.Center
			zoom = v.Zoom
		}
	}

	return &App{
		deps:     deps,
		keys:     DefaultKeymap(),
		t:        t,
		theme:    theme,
		ascii:    !deps.Cfg.Map.Braille,
		mode:     ModeNormal,
		viewport: Viewport{Center: center, Zoom: zoom},
		search:    newSearchModel(t),
		poi:       newPOIModel(t),
		rt:        newRouteModel(),
		bookmarks: newBookmarkModel(t),
		sidebar:   deps.Cfg.UI.Sidebar,
		bgCtx:    context.Background(),
	}
}

// Init is called once at startup. It seeds the layer fetch.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.refreshLayers(),
		welcomeNotification(a.t),
	)
}

// scheduleLayerRefresh debounces the next Overpass fetch — typing several
// pan/zoom keys quickly only triggers one round-trip after the user
// settles. The debounce window matches the search field's.
func (a *App) scheduleLayerRefresh() tea.Cmd {
	a.layerDebounceID++
	id := a.layerDebounceID
	return tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
		return layerDebounceMsg{id: id}
	})
}

// welcomeNotification shows a one-second splash-style status line.
func welcomeNotification(t i18n.Strings) tea.Cmd {
	return func() tea.Msg {
		return notificationMsg{
			text:    fmt.Sprintf("%s — %s", t.AppName, version.String()),
			expires: time.Now().Add(2 * time.Second),
		}
	}
}

// Update is the main reducer.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.recomputeViewport()
		return a, a.scheduleLayerRefresh()

	case featuresLoadedMsg:
		if m.err != nil {
			// Keep the previously-loaded features visible. The user
			// might still be panning around the cached area; a
			// failure to refresh shouldn't blank the map.
			return a, a.notify(a.t.NetworkError + " — affichage en cache")
		}
		a.features = m.collection
		return a, nil

	case poisLoadedMsg:
		if m.err != nil {
			return a, a.notify(a.t.NetworkError + ": " + m.err.Error())
		}
		a.pois = m.collection
		return a, nil

	case searchResultsMsg:
		a.search.loading = false
		if m.err != nil {
			return a, a.notify(a.t.NetworkError + ": " + m.err.Error())
		}
		a.search.applyResults(m.results)
		return a, nil

	case routeLoadedMsg:
		a.rt.loading = false
		if m.err != nil {
			a.rt.err = m.err
			return a, a.notify(a.t.NetworkError + ": " + m.err.Error())
		}
		a.rt.route = &m.route
		a.route = &m.route
		return a, a.notify(fmt.Sprintf("%s: %.1f km — %s",
			a.t.Route, m.route.Distance/1000, formatDuration(m.route.Duration)))

	case notificationMsg:
		a.notification = m.text
		a.notificationExp = m.expires
		return a, tea.Tick(time.Until(m.expires), func(_ time.Time) tea.Msg {
			return notificationExpiredMsg{}
		})

	case notificationExpiredMsg:
		if time.Now().After(a.notificationExp) {
			a.notification = ""
		}
		return a, nil

	case debounceSearchMsg:
		// Ignore stale debounce ticks.
		if m.id != a.search.debounceID {
			return a, nil
		}
		if strings.TrimSpace(m.query) == "" {
			a.search.applyResults(nil)
			return a, nil
		}
		if a.deps.Nominatim == nil {
			return a, nil
		}
		a.search.loading = true
		return a, fetchResults(a.bgCtx, a.deps.Nominatim, m.query, a.deps.Cfg.UI.Lang)

	case gpsFixMsg:
		if m.err != nil {
			return a, tea.Batch(a.notify("GPS: "+m.err.Error()), a.rescheduleGPS())
		}
		a.applyFix(m.fix)
		return a, tea.Batch(a.refreshLayers(), a.rescheduleGPS())

	case gpsTickMsg:
		if !a.gps.enabled {
			return a, nil
		}
		return a, pollGPS(a.bgCtx)

	case trafficLoadedMsg:
		if m.err != nil {
			return a, tea.Batch(a.notify("Trafic: "+m.err.Error()), a.rescheduleTraffic())
		}
		a.traffic.incidents = m.incidents
		a.traffic.updatedAt = time.Now()
		return a, tea.Batch(
			a.notify(fmt.Sprintf("Trafic: %d incident(s)", len(m.incidents))),
			a.rescheduleTraffic(),
		)

	case trafficTickMsg:
		if !a.traffic.enabled {
			return a, nil
		}
		return a, fetchIncidents(a.bgCtx, a.deps.TomTom, a.viewport.BBox(), a.deps.Cfg.UI.Lang)

	case layerDebounceMsg:
		if m.id != a.layerDebounceID {
			return a, nil
		}
		return a, a.refreshLayers()

	case tea.KeyMsg:
		return a.handleKey(m)
	}

	switch a.mode {
	case ModeSearch:
		return a, a.search.Update(msg)
	case ModePOI:
		return a, a.poi.Update(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Mode-specific handling first; only the normal-mode bindings fall
	// through to the global handler.
	switch a.mode {
	case ModeSearch:
		return a.handleSearchKey(msg)
	case ModePOI:
		return a.handlePOIKey(msg)
	case ModeRoute:
		return a.handleRouteKey(msg)
	case ModeHelp:
		return a.handleHelpKey(msg)
	case ModeMeasure:
		return a.handleMeasureKey(msg)
	case ModeBookmarks:
		return a.handleBookmarkKey(msg)
	}
	return a.handleNormalKey(msg)
}

func (a *App) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatch(a.keys.Quit, msg):
		_ = a.persistViewport()
		return a, tea.Quit
	case keyMatch(a.keys.Help, msg):
		a.mode = ModeHelp
	case keyMatch(a.keys.OpenSearch, msg):
		a.mode = ModeSearch
		return a, a.search.Focus()
	case keyMatch(a.keys.OpenPOI, msg):
		a.mode = ModePOI
	case keyMatch(a.keys.OpenRoute, msg):
		a.mode = ModeRoute
	case keyMatch(a.keys.ToggleSidebar, msg):
		a.sidebar = !a.sidebar
		a.recomputeViewport()
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.PanLeft, msg):
		a.viewport.Pan(-2, 0)
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.PanRight, msg):
		a.viewport.Pan(2, 0)
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.PanUp, msg):
		a.viewport.Pan(0, -2)
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.PanDown, msg):
		a.viewport.Pan(0, 2)
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.ZoomIn, msg):
		a.viewport.SetZoom(a.viewport.Zoom + 1)
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.ZoomOut, msg):
		a.viewport.SetZoom(a.viewport.Zoom - 1)
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.Reset, msg):
		a.viewport.Center = geo.LatLng{Lat: a.deps.Cfg.Map.DefaultLat, Lng: a.deps.Cfg.Map.DefaultLng}
		a.viewport.Zoom = a.deps.Cfg.Map.DefaultZoom
		return a, a.scheduleLayerRefresh()
	case keyMatch(a.keys.Center, msg):
		// gg-style chord: 'g' once, then 'g' again to centre.
		if a.prevKey == "g" {
			a.viewport.Center = geo.LatLng{Lat: a.deps.Cfg.Map.DefaultLat, Lng: a.deps.Cfg.Map.DefaultLng}
			a.prevKey = ""
			return a, a.scheduleLayerRefresh()
		}
		a.prevKey = "g"
		return a, nil
	case keyMatch(a.keys.AddBookmark, msg):
		return a, a.addBookmark()
	case keyMatch(a.keys.ListBookmark, msg):
		if a.deps.Store != nil {
			items, _ := a.deps.Store.ListBookmarks()
			a.bookmarks.refresh(items)
		}
		a.mode = ModeBookmarks
		return a, nil
	case msg.String() == "m":
		a.mode = ModeMeasure
		return a, nil
	case msg.String() == "G":
		return a, a.startGPS()
	case msg.String() == "H":
		a.heatmap = !a.heatmap
		if a.heatmap {
			return a, a.notify("Heatmap PoI activée")
		}
		return a, a.notify("Heatmap PoI désactivée")
	case msg.String() == "T":
		return a, a.toggleTraffic()
	}
	a.prevKey = msg.String()
	return a, nil
}

// toggleTraffic enables or disables the traffic overlay. When enabling, a
// fetch is kicked off immediately; the next refresh is scheduled by the
// trafficLoadedMsg handler.
func (a *App) toggleTraffic() tea.Cmd {
	a.traffic.enabled = !a.traffic.enabled
	if !a.traffic.enabled {
		a.traffic.incidents = nil
		return a.notify("Trafic désactivé")
	}
	if a.deps.TomTom == nil {
		a.traffic.enabled = false
		return a.notify("TomTom non configuré (providers.tomtom_api_key)")
	}
	return tea.Batch(
		a.notify("Trafic: récupération…"),
		fetchIncidents(a.bgCtx, a.deps.TomTom, a.viewport.BBox(), a.deps.Cfg.UI.Lang),
	)
}

func (a *App) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatch(a.keys.Cancel, msg):
		a.mode = ModeNormal
		a.search.Blur()
		return a, nil
	case keyMatch(a.keys.Confirm, msg):
		if r, ok := a.search.selected(); ok {
			a.viewport.Center = r.Position
			a.markers = []geo.LatLng{r.Position}
			a.mode = ModeNormal
			a.search.Blur()
			if a.deps.Store != nil {
				_ = a.deps.Store.AppendHistory(data.HistoryEntry{
					Query:    r.DisplayName,
					Position: r.Position,
				})
			}
			return a, a.refreshLayers()
		}
		return a, nil
	}
	prev := a.search.input.Value()
	cmd := a.search.Update(msg)
	if a.search.input.Value() != prev {
		return a, tea.Batch(cmd, a.search.debounce())
	}
	return a, cmd
}

func (a *App) handlePOIKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatch(a.keys.Cancel, msg):
		a.mode = ModeNormal
		return a, nil
	case keyMatch(a.keys.Confirm, msg):
		if cat, ok := a.poi.selectedCategory(); ok {
			a.poi.loading = true
			if a.deps.Overpass == nil {
				return a, nil
			}
			return a, fetchPOIs(a.bgCtx, a.deps.Overpass, a.viewport.BBox(), []data.POICategory{cat})
		}
		return a, nil
	}
	return a, a.poi.Update(msg)
}

func (a *App) handleRouteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatch(a.keys.Cancel, msg):
		a.mode = ModeNormal
		return a, nil
	case msg.String() == " ":
		// Space sets the start (then end) at the current centre.
		c := a.viewport.Center
		if a.rt.start == nil {
			a.rt.start = &c
			return a, a.notify("Départ fixé")
		}
		a.rt.end = &c
		a.rt.loading = true
		if a.deps.OSRM == nil {
			return a, nil
		}
		return a, computeRoute(a.bgCtx, a.deps.OSRM, a.rt.profile, *a.rt.start, *a.rt.end)
	case msg.String() == "p":
		a.rt.profile = routeProfilesCycle(a.rt.profile)
		return a, nil
	case msg.String() == "x":
		a.rt.start, a.rt.end, a.rt.route, a.route = nil, nil, nil, nil
		return a, nil
	case msg.String() == "e":
		if err := exportGPX(a.rt.route, "CarTUI route", "cartui-route.gpx"); err != nil {
			return a, a.notify(err.Error())
		}
		return a, a.notify(a.t.GPXExported + ": cartui-route.gpx")
	}
	return a, nil
}

func (a *App) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatch(a.keys.Cancel, msg), keyMatch(a.keys.Help, msg):
		a.mode = ModeNormal
	}
	return a, nil
}

// View is the main render entry point.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return a.t.AppName + " — " + a.t.Loading
	}
	header := a.renderHeader()
	footer := a.renderFooter()
	bodyHeight := a.height - 2 // header + footer
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	mapStr := renderMap(a.viewport, a.theme, a.ascii, a.features, a.pois, a.route, a.markers, a.measure.points,
		renderConfig{
			Heatmap:   a.heatmap,
			Incidents: a.traffic.incidents,
		})

	body := mapStr
	if a.sidebar {
		side := a.renderSidebar(bodyHeight, a.sidebarWidth())
		body = lipgloss.JoinHorizontal(lipgloss.Top, mapStr, side)
	}

	out := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	switch a.mode {
	case ModeSearch:
		out = overlay(out, a.search.View(min(60, a.width-4), a.t))
	case ModePOI:
		out = overlay(out, a.poi.View(min(60, a.width-4), a.t))
	case ModeRoute:
		out = overlay(out, a.rt.View(min(60, a.width-4), a.t))
	case ModeHelp:
		out = overlay(out, helpView(min(70, a.width-4), a.t, a.keys))
	case ModeMeasure:
		out = overlay(out, a.measure.View(min(60, a.width-4), a.t))
	case ModeBookmarks:
		out = overlay(out, a.bookmarks.View(min(60, a.width-4), a.t))
	}
	return out
}

func (a *App) sidebarWidth() int {
	if a.width <= 60 {
		return 0
	}
	return 28
}

func (a *App) recomputeViewport() {
	bw := a.width
	if a.sidebar {
		bw -= a.sidebarWidth()
	}
	if bw < 10 {
		bw = 10
	}
	bh := a.height - 2
	if bh < 4 {
		bh = 4
	}
	a.viewport.WidthCells = bw
	a.viewport.HeightCells = bh
}

func (a *App) refreshLayers() tea.Cmd {
	if a.deps.Overpass == nil {
		return nil
	}
	return fetchMapLayers(a.bgCtx, a.deps.Overpass, a.viewport.BBox().Expand(0.001, 0.001), a.viewport.Zoom)
}

func (a *App) addBookmark() tea.Cmd {
	if a.deps.Store == nil {
		return a.notify("store unavailable")
	}
	b := data.Bookmark{
		Name:     a.viewport.Center.String(),
		Position: a.viewport.Center,
	}
	if _, err := a.deps.Store.SaveBookmark(b); err != nil {
		return a.notify(err.Error())
	}
	return a.notify(a.t.BookmarkAdded)
}

func (a *App) persistViewport() error {
	if a.deps.Store == nil {
		return nil
	}
	return a.deps.Store.SaveViewport(store.Viewport{
		Center: a.viewport.Center,
		Zoom:   a.viewport.Zoom,
	})
}

func (a *App) notify(text string) tea.Cmd {
	return func() tea.Msg {
		return notificationMsg{text: text, expires: time.Now().Add(4 * time.Second)}
	}
}
