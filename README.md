# CarTUI

```
   ____           _____ _   _ ___
  / ___|__ _ _ __|_   _| | | |_ _|
 | |   / _` | '__|  | | | | | || |
 | |__| (_| | |     | | | |_| || |
  \____\__,_|_|     |_|  \___/|___|
```

A terminal-native cartography app. Google Maps power, zero pixels — only Braille,
ANSI, and a fast Go runtime. Pan, zoom, search, route, and bookmark places without
ever leaving your shell.

> Status: early scaffolding. See `docs/ARCHITECTURE.md` and `CHANGELOG.md` for
> the live state of the project.

## Why

- Maps belong everywhere — including the terminals where many of us live.
- Open data, open source: built on OpenStreetMap-derived APIs and licensed AGPL-3.0.
- No tracking, no telemetry, no surprise dependencies.

## Quickstart

```bash
go install github.com/cycl0o0/cartui/cmd/cartui@latest
cartui
```

See `docs/KEYBINDINGS.md` for the full keymap.

## License

AGPL-3.0-or-later. See `LICENSE`.
