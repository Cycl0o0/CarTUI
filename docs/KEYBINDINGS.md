# Keybindings

All bindings live in [`internal/tui/keys.go`](../internal/tui/keys.go) so
the help screen and the update loop never drift apart.

## Map navigation (NORMAL mode)

| Key         | Action                              |
| ----------- | ----------------------------------- |
| `h` / `←`   | Pan left                            |
| `j` / `↓`   | Pan down                            |
| `k` / `↑`   | Pan up                              |
| `l` / `→`   | Pan right                           |
| `+` / `=`   | Zoom in                             |
| `-` / `_`   | Zoom out                            |
| `gg`        | Centre on the configured default    |
| `0`         | Reset zoom to default               |

## Mode entry

| Key   | Mode               |
| ----- | ------------------ |
| `/`   | Search             |
| `p`   | Points of interest |
| `i`   | Routing wizard     |
| `?`   | Help               |
| `:`   | Command (`:goto`)  |

## Bookmarks

| Key   | Action                                        |
| ----- | --------------------------------------------- |
| `f`   | Bookmark the current map centre               |
| `F`   | List bookmarks (TODO: list overlay)           |

## Search mode (`/`)

| Key       | Action                                 |
| --------- | -------------------------------------- |
| Type      | Live debounced query (300ms)           |
| `↑` / `↓` | Move through results                   |
| `Enter`   | Centre on selection, place a marker    |
| `Esc`     | Cancel and return to NORMAL            |

## Routing mode (`i`)

| Key      | Action                                       |
| -------- | -------------------------------------------- |
| `Space`  | Set start (1st press) / end (2nd press) at the centre, then run OSRM |
| `p`      | Cycle profile: driving → cycling → walking   |
| `e`      | Export current route as `cartui-route.gpx`   |
| `x`      | Clear the current route                      |
| `Esc`    | Return to NORMAL                             |

## POI mode (`p`)

| Key       | Action                            |
| --------- | --------------------------------- |
| `↑` / `↓` | Move through categories           |
| `Enter`   | Fetch POIs in the current view    |
| `Esc`     | Return to NORMAL                  |

## Global

| Key          | Action            |
| ------------ | ----------------- |
| `Tab`        | Toggle sidebar    |
| `q`          | Quit              |
| `Ctrl+C`     | Quit              |

## Customising

There is no runtime keymap override yet — edit
[`internal/tui/keys.go`](../internal/tui/keys.go) and rebuild. A
config-driven keymap is on the roadmap.
