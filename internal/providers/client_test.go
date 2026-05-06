// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoSetsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{UserAgent: "TestUA/1.0"})
	var out map[string]any
	require.NoError(t, c.GetJSON(context.Background(), srv.URL, &out))
	assert.Equal(t, "TestUA/1.0", got)
}

func TestDoRetriesOn5xx(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&n, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{MaxRetries: 5})
	var out struct{ OK bool }
	require.NoError(t, c.GetJSON(context.Background(), srv.URL, &out))
	assert.True(t, out.OK)
	assert.Equal(t, int32(3), atomic.LoadInt32(&n))
}

func TestDoFailsAfterMaxRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{MaxRetries: 1})
	err := c.GetJSON(context.Background(), srv.URL, nil)
	require.Error(t, err)
}

func TestDoRespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{Timeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := c.GetJSON(ctx, srv.URL, nil)
	assert.Error(t, err)
}

func TestRateLimitDelaysSecondCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	host := srv.Listener.Addr().String()
	c.SetRateLimit(host, 10) // 10 rps -> 100 ms gate

	start := time.Now()
	for i := 0; i < 3; i++ {
		require.NoError(t, c.GetJSON(context.Background(), srv.URL, nil))
	}
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
}

func TestParseRetryAfterSeconds(t *testing.T) {
	d := parseRetryAfter("3")
	assert.Equal(t, 3*time.Second, d)
}

func TestParseRetryAfterEmpty(t *testing.T) {
	assert.Equal(t, time.Duration(0), parseRetryAfter(""))
}

func TestShouldRetry(t *testing.T) {
	assert.True(t, shouldRetry(http.StatusTooManyRequests, nil))
	assert.True(t, shouldRetry(http.StatusBadGateway, nil))
	assert.False(t, shouldRetry(http.StatusOK, nil))
	assert.False(t, shouldRetry(http.StatusNotFound, nil))
}

func TestRequestJSONErrorsOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`not found`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{MaxRetries: 0})
	err := c.GetJSON(context.Background(), srv.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}
