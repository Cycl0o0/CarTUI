// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cycl0o0/cartui/internal/config"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/cycl0o0/cartui/internal/providers"
	"github.com/cycl0o0/cartui/internal/store"
	"github.com/cycl0o0/cartui/internal/tiles"
	"github.com/spf13/cobra"
)

// newPrefetchCmd returns the `cartui prefetch` subcommand. It populates the
// raster tile cache for a given bbox + zoom range so the user can later use
// the app fully offline.
func newPrefetchCmd() *cobra.Command {
	var (
		bboxStr  string
		zoomStr  string
		parallel int
	)
	cmd := &cobra.Command{
		Use:   "prefetch",
		Short: "Download tiles for a bbox and zoom range into the local cache",
		Long: `Prefetch warms the on-disk tile cache so CarTUI can render the
chosen area without further network calls.

Coordinates use the OSM convention "south,west,north,east"; zooms are a
range "min-max" or comma-separated "13,14,15".

Example:
  cartui prefetch --bbox 44.80,-0.65,44.90,-0.50 --zoom 13-15`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load("")
			if err != nil {
				return err
			}

			bbox, err := geo.ParseBBox(bboxStr)
			if err != nil {
				return fmt.Errorf("--bbox: %w", err)
			}
			zooms, err := parseZoomRange(zoomStr)
			if err != nil {
				return fmt.Errorf("--zoom: %w", err)
			}
			if parallel < 1 {
				parallel = 4
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = db.Close() }()

			cache, err := tiles.NewCache(db.Bolt(), cfg.TileTTL())
			if err != nil {
				return err
			}

			httpClient := providers.NewClient(providers.ClientOptions{
				Timeout:    cfg.Timeout(),
				MaxRetries: cfg.Network.Retries,
			})
			httpClient.SetRateLimit(hostOf(cfg.Providers.TileURL), cfg.Providers.Rate.TileRPS)
			fetcher := tiles.NewFetcher(httpClient, cfg.Providers.TileURL, cache)

			var addresses []tiles.Address
			for _, z := range zooms {
				addresses = append(addresses, tiles.CoveringTiles(bbox, z)...)
			}
			total := len(addresses)
			if total == 0 {
				return fmt.Errorf("nothing to prefetch")
			}
			fmt.Printf("prefetching %d tiles (zooms %v, %d parallel)…\n", total, zooms, parallel)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			start := time.Now()
			done := make(chan struct{})
			var fetched, failed int64

			work := make(chan tiles.Address)
			go func() {
				defer close(work)
				for _, a := range addresses {
					select {
					case work <- a:
					case <-ctx.Done():
						return
					}
				}
			}()
			for i := 0; i < parallel; i++ {
				go func() {
					for a := range work {
						_, err := fetcher.Fetch(ctx, a)
						if err != nil {
							atomic.AddInt64(&failed, 1)
							continue
						}
						atomic.AddInt64(&fetched, 1)
					}
					done <- struct{}{}
				}()
			}
			for i := 0; i < parallel; i++ {
				<-done
			}
			fmt.Printf("done — %d ok, %d failed in %s\n",
				atomic.LoadInt64(&fetched), atomic.LoadInt64(&failed),
				time.Since(start).Round(time.Millisecond))
			return nil
		},
	}
	cmd.Flags().StringVar(&bboxStr, "bbox", "", "south,west,north,east")
	cmd.Flags().StringVar(&zoomStr, "zoom", "13", "zoom levels: '13', '13,14,15' or '13-15'")
	cmd.Flags().IntVar(&parallel, "parallel", 4, "concurrent downloads (be polite to the tile server)")
	_ = cmd.MarkFlagRequired("bbox")
	return cmd
}

// parseZoomRange accepts a single number, a comma-separated list or a
// dash-separated range — whichever feels most natural to the caller.
func parseZoomRange(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty")
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		lo, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, err
		}
		hi, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, err
		}
		if hi < lo {
			lo, hi = hi, lo
		}
		out := make([]int, 0, hi-lo+1)
		for z := lo; z <= hi; z++ {
			out = append(out, z)
		}
		return out, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
