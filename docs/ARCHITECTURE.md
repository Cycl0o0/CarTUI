# Architecture

CarTUI is a single Go binary built around three asymmetric layers:

1. **Domain & math** (`internal/geo`, `internal/data`) — pure types and
   functions, zero dependencies beyond the stdlib.
2. **Capabilities** (`internal/render`, `internal/providers`,
   `internal/store`, `internal/tiles`, `internal/config`,
   `internal/i18n`) — single-purpose packages, each safe to import in
   isolation.
3. **Application** (`internal/tui`, `cmd/cartui`) — the Bubble Tea event
   loop and the Cobra CLI wiring everything together.

```
cartui (cmd) --> tui (Bubble Tea) --> { geo · render · providers · store · tiles }
                                       \-- depend only on stdlib + 3rd-party libs
```

No package below `internal/tui` imports `internal/tui`. The dependency
graph is acyclic and one-directional toward the binary entrypoint.

## Rendering pipeline

The renderer is the central piece of CarTUI's value proposition:
mapping vector geographic features onto a sub-cell terminal canvas.

```
LatLng polygons / lines     ┐
   (Overpass JSON)           │           ┌────────────────┐
LatLng points (POIs)         ├──────────▶│ Viewport.Pixel │── projects to canvas px
Active route (OSRM)          │           └────────────────┘
User markers                 │                   │
                             ┘                   ▼
                                          ┌────────────┐
                                          │ Bresenham  │ -- DrawLine, DrawPolyline
                                          │ Scanline   │ -- FillPolygon, FillRect
                                          └────────────┘
                                                 │
                                                 ▼
                                          ┌────────────┐
                                          │ Canvas (2×4│
                                          │  Braille)  │ -- 8 dots per cell
                                          └────────────┘
                                                 │
                                                 ▼
                                          Lipgloss styled string
```

Every Braille pixel is tagged with a `Layer` (water / green / building /
road / boundary / POI / route / marker). When two features paint the
same terminal cell, the highest-priority layer's colour wins. This gives
maps a clean "stack" appearance: roads are always visible above the
water and parks they cross.

## Bubble Tea model

A single root model (`internal/tui.App`) owns everything. Mode
transitions (`NORMAL`, `SEARCH`, `ROUTE`, `POI`, `HELP`) are tracked as
an enum field; key handling routes through mode-specific helpers.

Async work — Nominatim search, Overpass fetch, OSRM routing — is
expressed as `tea.Cmd` factories that capture a `context.Context` and
return typed messages (`searchResultsMsg`, `featuresLoadedMsg`,
`routeLoadedMsg`). The Bubble Tea `Update` reducer pattern matches on
those messages to mutate state.

Search uses a debounce loop: a `debounceSearchMsg` is scheduled 300ms
after every keystroke; only the most recent debounce id wins, the rest
are discarded.

## HTTP client

`internal/providers/client.go` implements:

- Per-host rate limit via a `time.Time`-based gate (no goroutines, no
  buffered channels — predictable behaviour under cancellation).
- Retries with exponential back-off + jitter on 429/5xx, honouring
  `Retry-After` when set.
- Mandatory `User-Agent: CarTUI/<version> (+github.com/cycl0o0/cartui)`
  to comply with Nominatim/Overpass usage policies.
- Transparent gzip decoding.

The same client serves Nominatim, Overpass, OSRM and the (optional)
raster tile fetcher. Hosts have independent rate limits so a slow
Overpass query never blocks geocoding.

## Persistence

[bbolt](https://github.com/etcd-io/bbolt) — pure-Go embedded KV store
(no cgo). One file at `~/.cache/cartui/cartui.db` with four buckets:

| Bucket      | Key                  | Value                                      |
| ----------- | -------------------- | ------------------------------------------ |
| `bookmarks` | `<hex>` (16 bytes)   | JSON `data.Bookmark`                       |
| `history`   | RFC 3339 nanosecond  | JSON `data.HistoryEntry`                   |
| `tiles`     | `z/x/y`              | JSON `tiles.CacheEntry` (PNG blob + meta)  |
| `state`     | `last_view`          | JSON `store.Viewport`                      |

The `last_view` entry lets CarTUI reopen exactly where it was closed.

## Configuration

[Viper](https://github.com/spf13/viper) reads
`~/.config/cartui/config.toml`. Defaults are baked in
(`config.Defaults()`); env-var overrides use the `CARTUI_` prefix and
underscore-mapped keys (e.g. `CARTUI_MAP_DEFAULT_ZOOM`).

The pattern: `Defaults() -> SetDefault() -> ReadInConfig() ->
Unmarshal() -> apply CLI flag overrides`.

## Why no goroutine pool / pubsub / cqrs?

The TUI runs single-threaded by design (Bubble Tea is an event loop).
Every async unit is a `tea.Cmd` that returns a message. Goroutines are
short-lived and bounded by the timeout on the captured `context.Context`.
This keeps the model trivially debuggable.

## Open trade-offs

- **Vector vs raster** — Overpass-driven vector rendering is the
  default. Raster tiles exist (`internal/tiles`) but stay opt-in: 256×256
  PNGs do not downsample cleanly to a 2×4 Braille cell.
- **Single-colour Braille cell** — terminals can't paint individual
  Braille dots in different colours. We accept this, picking the
  dominant layer's colour.
- **Antimeridian** — `BBox` and `LatLng` support wrap-around containment,
  but the Overpass query helper does not yet split bboxes that cross
  ±180° lng. Edge case relevant only at very low zoom near the date line.

## Future / out of scope

The user-facing prompt listed several bonus features that ship as
TODOs:

- `gpsd` integration for a real-time "follow me" mode.
- Mapillary Street View → ASCII art viewer.
- Bookmark sync via WebDAV / Nextcloud.
- TomTom / HERE traffic overlay.
- Heatmaps and pre-downloaded offline bbox.
- Distance-measurement multi-point ruler.

See [`CHANGELOG.md`](../CHANGELOG.md#unreleased--future) for the live
roadmap.
