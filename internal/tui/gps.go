// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/providers"
)

// gpsFixMsg carries a fresh fix from the gpsd stream.
type gpsFixMsg struct {
	fix providers.GPSFix
	err error
}

// gpsTickMsg requests the next fix poll while follow mode is on.
type gpsTickMsg struct{}

// gpsState holds the per-app GPS bookkeeping.
type gpsState struct {
	enabled bool
	follow  bool
	last    *providers.GPSFix
}

// startGPS toggles GPS-follow mode and kicks off a poll loop. The first
// successful fix message recenters the viewport; subsequent fixes do too,
// while follow mode is on.
func (a *App) startGPS() tea.Cmd {
	a.gps.enabled = !a.gps.enabled
	a.gps.follow = a.gps.enabled
	if !a.gps.enabled {
		return a.notify("GPS désactivé")
	}
	return tea.Batch(a.notify("GPS activé — recherche de signal…"), pollGPS(a.bgCtx))
}

// pollGPS schedules a one-shot fix and reschedules itself after the result
// has been delivered.
func pollGPS(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		c := providers.NewGPSD("")
		fix, err := c.CurrentFix(ctx)
		return gpsFixMsg{fix: fix, err: err}
	}
}

// rescheduleGPS arms the next poll iff follow mode is still on.
func (a *App) rescheduleGPS() tea.Cmd {
	if !a.gps.enabled {
		return nil
	}
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return gpsTickMsg{} })
}

// applyFix updates state from a successful fix.
func (a *App) applyFix(fix providers.GPSFix) {
	a.gps.last = &fix
	if a.gps.follow {
		a.viewport.Center = fix.AsLatLng()
		a.markers = []geo.LatLng{fix.AsLatLng()}
	}
}
