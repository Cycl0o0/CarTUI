// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cycl0o0/cartui/internal/geo"
)

// gpsdAddr is the default `gpsd` daemon TCP address.
const gpsdAddr = "127.0.0.1:2947"

// GPSFix is a simplified position report extracted from a gpsd `TPV`
// payload. Only the fields CarTUI uses are decoded; everything else is
// ignored.
type GPSFix struct {
	Time    time.Time
	Lat     float64
	Lng     float64
	AltM    float64 // metres above sea level (0 when missing)
	SpeedMS float64 // metres per second (0 when missing)
	Mode    int     // 0 unknown, 1 no fix, 2 2D, 3 3D
}

// AsLatLng returns the position as a [geo.LatLng].
func (f GPSFix) AsLatLng() geo.LatLng { return geo.LatLng{Lat: f.Lat, Lng: f.Lng} }

// HasFix reports whether the receiver had a 2D or 3D fix when the report
// was emitted.
func (f GPSFix) HasFix() bool { return f.Mode >= 2 }

// GPSD is a minimal gpsd client. The daemon is queried with a single
// `?WATCH={"enable":true,"json":true}` command and we read JSON lines until
// the context is done. Each received `TPV` line emits one [GPSFix].
type GPSD struct {
	addr string
}

// NewGPSD builds a client. An empty addr defaults to `127.0.0.1:2947`.
func NewGPSD(addr string) *GPSD {
	if addr == "" {
		addr = gpsdAddr
	}
	return &GPSD{addr: addr}
}

// Stream opens a connection and pushes every [GPSFix] received on the
// returned channel until ctx is cancelled. The channel is closed when
// the stream ends. Errors are surfaced via the second return value.
func (g *GPSD) Stream(ctx context.Context) (<-chan GPSFix, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", g.addr)
	if err != nil {
		return nil, fmt.Errorf("gpsd dial: %w", err)
	}
	if _, err := conn.Write([]byte(`?WATCH={"enable":true,"json":true};` + "\n")); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("gpsd watch: %w", err)
	}

	out := make(chan GPSFix, 1)
	go func() {
		defer close(out)
		defer func() { _ = conn.Close() }()

		go func() {
			<-ctx.Done()
			_ = conn.SetDeadline(time.Now())
		}()

		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 0, 4096), 1<<20)
		for sc.Scan() {
			if ctx.Err() != nil {
				return
			}
			fix, ok := parseTPV(sc.Bytes())
			if !ok {
				continue
			}
			select {
			case out <- fix:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// CurrentFix opens a short-lived connection, returns the first valid fix and
// closes. Useful for one-shot status updates without keeping a stream open.
func (g *GPSD) CurrentFix(ctx context.Context) (GPSFix, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ch, err := g.Stream(ctx)
	if err != nil {
		return GPSFix{}, err
	}
	for fix := range ch {
		if fix.HasFix() {
			return fix, nil
		}
	}
	return GPSFix{}, errors.New("gpsd: no valid fix in window")
}

// parseTPV decodes a single newline-delimited JSON message from gpsd. The
// function only recognises the `TPV` class; anything else is ignored.
func parseTPV(line []byte) (GPSFix, bool) {
	var raw struct {
		Class string  `json:"class"`
		Time  string  `json:"time"`
		Lat   float64 `json:"lat"`
		Lon   float64 `json:"lon"`
		Alt   float64 `json:"alt"`
		Speed float64 `json:"speed"`
		Mode  int     `json:"mode"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return GPSFix{}, false
	}
	if raw.Class != "TPV" {
		return GPSFix{}, false
	}
	t, _ := time.Parse(time.RFC3339, raw.Time)
	return GPSFix{
		Time:    t,
		Lat:     raw.Lat,
		Lng:     raw.Lon,
		AltM:    raw.Alt,
		SpeedMS: raw.Speed,
		Mode:    raw.Mode,
	}, true
}
