// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cycl0o0/cartui/internal/providers"
)

// Fetcher downloads slippy-map raster tiles, consulting the cache first.
//
// All upstream calls go through the rate-limited [providers.Client] — the
// OpenStreetMap tile policy is strict (no bulk/automated downloads, max ~2
// req/s per host). Configure the client accordingly.
type Fetcher struct {
	client      *providers.Client
	urlTemplate string
	cache       *Cache
}

// NewFetcher builds a fetcher. urlTemplate must contain `{z}`, `{x}` and
// `{y}` placeholders. An empty template defaults to the official OSM
// endpoint.
func NewFetcher(c *providers.Client, urlTemplate string, cache *Cache) *Fetcher {
	if urlTemplate == "" {
		urlTemplate = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"
	}
	return &Fetcher{client: c, urlTemplate: urlTemplate, cache: cache}
}

// Fetch returns the raw bytes for a tile. Cache hits short-circuit the
// network round-trip. Cache writes happen after a successful fetch but their
// failure does not propagate to the caller.
func (f *Fetcher) Fetch(ctx context.Context, a Address) ([]byte, error) {
	if f.cache != nil {
		blob, err := f.cache.Get(a)
		if err == nil {
			return blob, nil
		}
		if !errors.Is(err, ErrCacheMiss) && !errors.Is(err, ErrCacheExpired) {
			return nil, err
		}
	}

	url := a.URL(f.urlTemplate)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("tile request: %w", err)
	}
	resp, err := f.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tile fetch %s: %w", a.Key(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tile %s: %s", a.Key(), resp.Status)
	}
	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tile read: %w", err)
	}
	if f.cache != nil {
		_ = f.cache.Put(a, blob)
	}
	return blob, nil
}
