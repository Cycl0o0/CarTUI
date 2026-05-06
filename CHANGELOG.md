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

### Fixed

- Skip Overpass fetches below zoom 11 — over a continent-sized bbox
  the public endpoint reliably timed out. Client timeout brought down
  from 30s to 20s to surface failures earlier.
- CI coverage profiling restricted to `internal/...` to avoid the
  GOCOVERDIR `covdata` tool requirement on the Go 1.22 runner.

### Future

Out of scope for this release; PRs welcome:

- Heatmap of POI density per zoom.
- Mapillary Street-View → ASCII art viewer.
- Bookmark sync via WebDAV / Nextcloud.
- Real-time traffic overlays (TomTom or HERE).
- Configurable keymap from `config.toml`.
