// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package version exposes build metadata. All fields are populated by the
// release toolchain via -ldflags; defaults are sensible for `go run` and
// development builds.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// These are overridden at link time, e.g.
//
//	go build -ldflags "-X github.com/cycl0o0/cartui/internal/version.Version=v0.1.0"
var (
	// Version is the semantic-version string (without leading "v").
	Version = "0.1.0-dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "unknown"
	// Date is the build date in RFC 3339 form.
	Date = "unknown"
)

// String renders a one-line build identifier.
func String() string {
	return fmt.Sprintf("CarTUI %s (commit %s, built %s, %s)",
		Version, Commit, Date, runtime.Version())
}

// UserAgent returns the User-Agent string used for outbound HTTP requests.
// Public APIs (Nominatim, Overpass) require an identifiable UA.
func UserAgent() string {
	return fmt.Sprintf("CarTUI/%s (+github.com/cycl0o0/cartui)", Version)
}

// init populates Commit from the embedded VCS info when not provided via
// ldflags. Falls back silently when the binary is built outside a git tree.
func init() {
	if Commit != "unknown" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			Commit = s.Value[:7]
		}
		if s.Key == "vcs.time" && Date == "unknown" {
			Date = s.Value
		}
	}
}
