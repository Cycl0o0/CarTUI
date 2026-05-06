// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package providers wraps the external HTTP APIs CarTUI relies on
// (Nominatim, Overpass, OSRM). All providers share the same [Client]
// implementation, which adds rate limiting, retries with exponential
// back-off, and a User-Agent honouring the upstream usage policies.
package providers

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/cycl0o0/cartui/internal/version"
)

// Client is a rate-limited, retrying HTTP wrapper used by every provider.
//
// A single Client instance can be shared by multiple providers; rate limits
// are enforced per host so requests to different upstreams do not interfere.
// All public methods are safe for concurrent use.
type Client struct {
	http       *http.Client
	userAgent  string
	maxRetries int

	mu       sync.Mutex
	limiters map[string]*hostLimiter
}

// hostLimiter throttles a single hostname to a target requests-per-second.
type hostLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

// ClientOptions tunes the [Client] behaviour. Zero values have sensible
// defaults: 15s timeout, 3 retries, version-derived UA, no rate limits set.
type ClientOptions struct {
	Timeout    time.Duration
	MaxRetries int
	UserAgent  string
	Transport  http.RoundTripper // injected during tests; nil → http.DefaultTransport
}

// NewClient builds a configured client. Pass [ClientOptions]{} to take the
// defaults; populate fields you want to override.
func NewClient(opts ClientOptions) *Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	retries := opts.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = version.UserAgent()
	}
	tr := opts.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	return &Client{
		http:       &http.Client{Timeout: timeout, Transport: tr},
		userAgent:  ua,
		maxRetries: retries,
		limiters:   map[string]*hostLimiter{},
	}
}

// SetRateLimit sets the maximum requests-per-second for a hostname. A zero or
// negative rps removes the limit. The hostname must match the URL's host
// component verbatim (case-sensitive, with optional port).
func (c *Client) SetRateLimit(host string, rps float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rps <= 0 {
		delete(c.limiters, host)
		return
	}
	c.limiters[host] = &hostLimiter{
		interval: time.Duration(float64(time.Second) / rps),
	}
}

// limiterFor returns the limiter associated with host, creating none if absent.
func (c *Client) limiterFor(host string) *hostLimiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.limiters[host]
}

// wait blocks until the limiter allows the next request or ctx is done.
func (l *hostLimiter) wait(ctx context.Context) error {
	l.mu.Lock()
	wait := time.Until(l.next)
	if wait < 0 {
		wait = 0
	}
	l.next = time.Now().Add(wait + l.interval)
	l.mu.Unlock()

	if wait == 0 {
		return nil
	}
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Do executes an HTTP request with rate limiting and retries. The body of the
// last attempt is left on the returned response for the caller to read; all
// previous bodies are drained automatically.
//
// Retries trigger on network errors and on responses with status codes
// 429, 502, 503, 504. The 5xx schedule uses exponential back-off with jitter.
// 429 responses honour the Retry-After header when present.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	host := req.URL.Host
	limiter := c.limiterFor(host)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if limiter != nil {
			if err := limiter.wait(ctx); err != nil {
				return nil, err
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := c.http.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = fmt.Errorf("http: %w", err)
			if !shouldRetry(0, err) {
				return nil, lastErr
			}
			if err := backoff(ctx, attempt, 0); err != nil {
				return nil, err
			}
			continue
		}
		if !shouldRetry(resp.StatusCode, nil) {
			return resp, nil
		}
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		drain(resp)
		lastErr = fmt.Errorf("upstream %s: %s", host, resp.Status)
		if attempt == c.maxRetries {
			break
		}
		if err := backoff(ctx, attempt, retryAfter); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

// GetJSON performs a GET request and decodes the body into out. Gzip-encoded
// responses are transparently decompressed. The body is fully consumed and
// closed on return.
func (c *Client) GetJSON(ctx context.Context, url string, out any) error {
	return c.RequestJSON(ctx, http.MethodGet, url, nil, nil, out)
}

// RequestJSON is the generic JSON entry point: any verb, optional headers,
// optional request body. The response body is decoded as JSON into out.
func (c *Client) RequestJSON(ctx context.Context, method, url string, headers map[string]string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer drain(resp)

	reader, err := decompress(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		// Read up to 1 KiB of the error body for context.
		buf, _ := io.ReadAll(io.LimitReader(reader, 1<<10))
		return fmt.Errorf("upstream %s: %s: %s", req.URL.Host, resp.Status, string(buf))
	}
	if out == nil {
		return nil
	}
	return decodeJSON(reader, out)
}

// RequestRaw is like [Client.RequestJSON] but writes the (decompressed)
// response body verbatim into out. Useful when the caller wants to cache
// the bytes alongside the parsed value.
//
// Body size is capped at 32 MiB to avoid unbounded buffering.
func (c *Client) RequestRaw(ctx context.Context, method, url string, headers map[string]string, body io.Reader, out *bytes.Buffer) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer drain(resp)

	reader, err := decompress(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(io.LimitReader(reader, 1<<10))
		return fmt.Errorf("upstream %s: %s: %s", req.URL.Host, resp.Status, string(buf))
	}
	const maxBody = 32 << 20
	if _, err := io.Copy(out, io.LimitReader(reader, maxBody)); err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	return nil
}

// shouldRetry reports whether an error or status code is transient and worth
// retrying.
func shouldRetry(status int, err error) bool {
	if err != nil {
		var t interface{ Timeout() bool }
		if errors.As(err, &t) && t.Timeout() {
			return true
		}
		return true // network error: retry
	}
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff sleeps before retrying. honoursRetryAfter takes priority over the
// computed exponential schedule.
func backoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	d := retryAfter
	if d <= 0 {
		base := time.Duration(1<<attempt) * 250 * time.Millisecond
		jitter := time.Duration(rand.Int64N(int64(base))) //nolint:gosec // jitter only
		d = base + jitter
	}
	const maxBackoff = 10 * time.Second
	if d > maxBackoff {
		d = maxBackoff
	}
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// parseRetryAfter accepts the two RFC-allowed forms of Retry-After: a number
// of seconds or an HTTP-date.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if d, err := time.ParseDuration(h + "s"); err == nil && d > 0 {
		return d
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// drain consumes any remaining bytes and closes the response body. Required
// to allow the underlying TCP connection to be reused.
func drain(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
}

// decompress wraps the response body with a gzip reader when the upstream
// applied gzip encoding.
func decompress(resp *http.Response) (io.Reader, error) {
	if resp.Header.Get("Content-Encoding") != "gzip" {
		return resp.Body, nil
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	return gz, nil
}
