# Changelog

All notable changes to CarTUI are documented here. Format follows [Keep
a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public scaffolding: `geo`, `render`, `providers`, `store`,
  `tiles`, `config`, `i18n`, `tui`, `cmd/cartui` packages.
- Braille (2×4) sub-cell canvas with Bresenham lines, scanline
  polygons, layered colour priority and ASCII fallback.
- Web Mercator projection (LatLng ↔ WorldPixel ↔ slippy tile) with
  Vincenty + Haversine distance.
- HTTP client with per-host rate limiting, exponential back-off,
  Retry-After honouring, gzip, identifiable User-Agent.
- Nominatim search + reverse geocoding.
- Overpass viewport-bounded queries with zoom-aware feature selection.
- OSRM routing (driving / cycling / walking) and GPX 1.1 export.
- BoltDB-backed bookmarks, search history, viewport persistence and
  raster tile cache.
- Bubble Tea TUI with NORMAL / SEARCH / ROUTE / POI / HELP modes,
  vim-style keybindings and a sidebar for the active selection.
- Cobra CLI with `--goto`, `--lat`, `--lng`, `--zoom`, `--theme`,
  `--lang`, `--ascii`, `--config`, `--log`, `--log-level` flags.
- Viper TOML config with `CARTUI_*` env-var overrides.
- French + English i18n, `dark`, `light`, `mono` themes.
- GitHub Actions CI matrix (Linux/macOS × Go 1.22/1.23) with vet,
  build, test (race + coverage) and golangci-lint.
- GoReleaser configuration for Linux/macOS/Windows × amd64/arm64.

### Added (bonus features)

- **Distance ruler** (`m` mode): chain points at the centre, get
  cumulative haversine distance with undo / clear.
- **Bookmarks overlay** (`F`): list, centre, delete bookmarks.
- **gpsd / follow-me** (`G`): connect to a local gpsd at
  `127.0.0.1:2947`, recentre on each fix every 2 seconds.
- **Offline prefetch** subcommand: `cartui prefetch --bbox … --zoom …`
  warms the raster tile cache for fully-offline browsing later.
- **POI density heatmap** (`H`): blue→red 5-stop gradient over the
  currently-loaded POIs, painted only on otherwise-empty cells.
- **Real-time traffic** (`T`): TomTom Traffic Incidents v5 overlay,
  refreshed every 60s while enabled. Requires a free dev API key in
  `providers.tomtom_api_key`.

### Changed

- Default Overpass endpoint switched to
  `https://overpass.private.coffee/api/interpreter` (less rate-limited
  than the public OSM-hosted instance).
- CI Go matrix bumped to 1.24/1.25 to match the version required by
  `bubbles v1.0.0`. golangci-lint upgraded to v2.x with a migrated
  config file.

### Fixed

- **Overpass reliability**: every public mirror is flaky in different
  ways. Three changes lift this:
  - **Multi-endpoint fallback**: the client now tries
    `private.coffee → overpass-api.de → kumi.systems → maps.mail.ru`
    in order, falling through on transport, 5xx and 429 errors.
  - **BoltDB response cache** (60-min default TTL) keyed by query SHA1.
    Cache hits skip the network entirely; misses warm the cache after
    the first successful fetch.
  - **Pan/zoom debounce**: rapid keystrokes coalesce into a single
    fetch 300ms after the user settles, instead of firing four
    requests in 200ms.
  - **Lighter queries by zoom**: at z<13 only majors+water are asked
    for; buildings only at z≥16; boundaries at z≥14. Cuts response
    size by ~10× at low zoom.
  - On error the previously-loaded features stay rendered — no more
    blank screens between flaky fetches.
- Skip Overpass fetches below zoom 11 — over a continent-sized bbox
  the public endpoint reliably timed out. Client timeout brought down
  from 30s to 20s to surface failures earlier.
- CI coverage profiling restricted to `internal/...` to avoid the
  GOCOVERDIR `covdata` tool requirement on the Go 1.22 runner.
- CI lint job: bumped `golangci-lint-action` to v7 (v6 only supports
  golangci-lint v1; we run v2.0.2 to support Go 1.25).

### Future

Out of scope for this release; PRs welcome:

- Mapillary Street-View → ASCII art viewer.
- Bookmark sync via WebDAV / Nextcloud.
- HERE traffic backend as an alternative to TomTom.
- Configurable keymap from `config.toml`.
