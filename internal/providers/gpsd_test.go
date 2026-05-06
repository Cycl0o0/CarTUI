// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGPSD listens on a random port and replays a fixed sequence of TPV
// JSON lines. Useful to exercise the Stream loop without an actual gpsd
// daemon.
func fakeGPSD(t *testing.T, lines []string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// Wait for the WATCH command before responding.
		sc := bufio.NewReader(conn)
		_, _ = sc.ReadString('\n')
		for _, l := range lines {
			_, _ = conn.Write([]byte(l + "\n"))
		}
		// Hold the connection open until the client closes it.
		<-time.After(2 * time.Second)
	}()
	return ln.Addr().String()
}

func TestGPSDStreamEmitsTPV(t *testing.T) {
	addr := fakeGPSD(t, []string{
		`{"class":"VERSION","release":"3.20"}`,
		`{"class":"TPV","time":"2026-05-06T10:00:00Z","lat":44.8378,"lon":-0.5792,"alt":12.3,"speed":1.4,"mode":3}`,
	})
	g := NewGPSD(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := g.Stream(ctx)
	require.NoError(t, err)
	select {
	case fix, ok := <-ch:
		require.True(t, ok)
		assert.True(t, fix.HasFix())
		assert.InDelta(t, 44.8378, fix.Lat, 1e-6)
		assert.InDelta(t, -0.5792, fix.Lng, 1e-6)
		assert.Equal(t, 3, fix.Mode)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for fix")
	}
}

func TestParseTPVRejectsOtherClasses(t *testing.T) {
	_, ok := parseTPV([]byte(`{"class":"VERSION"}`))
	assert.False(t, ok)
}

func TestParseTPVRejectsMalformed(t *testing.T) {
	_, ok := parseTPV([]byte(`not json`))
	assert.False(t, ok)
}
