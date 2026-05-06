// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cycl0o0/cartui/internal/config"
	"github.com/spf13/cobra"
)

// pbfRegion mirrors the welcome screen presets so a single source-of-
// truth controls both the interactive picker and the CLI subcommand.
type pbfRegion struct {
	Key  string
	Name string
	URL  string
	Note string
}

var pbfRegions = []pbfRegion{
	{Key: "aquitaine", Name: "Aquitaine",
		URL:  "https://download.geofabrik.de/europe/france/aquitaine-latest.osm.pbf",
		Note: "~150 MB · 80 MB RAM · ~5 min init"},
	{Key: "france", Name: "France",
		URL:  "https://download.geofabrik.de/europe/france-latest.osm.pbf",
		Note: "~5 GB · 1.5 GB RAM · ~5 min init"},
	{Key: "europe", Name: "Europe",
		URL:  "https://download.geofabrik.de/europe-latest.osm.pbf",
		Note: "~28 GB · 7 GB RAM · ~30 min init  ⚠ heavy"},
	{Key: "planet", Name: "Planet",
		URL:  "https://planet.openstreetmap.org/pbf/planet-latest.osm.pbf",
		Note: "~80 GB · 30 GB RAM · ~6 h init  ⚠⚠⚠ extreme"},
}

// newPBFDownloadCmd returns `cartui pbf-download <region>`.
//
// Downloads the chosen Geofabrik (or planet.osm.org) PBF into the user
// cache, wires the resulting path into the config and offers a final
// summary of the disk + memory cost.
func newPBFDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pbf-download <region>",
		Short: "Download an OSM PBF extract for offline mode",
		Long: "Download an OpenStreetMap PBF extract from Geofabrik (or\n" +
			"planet.osm.org for the world dump) into the user cache and update\n" +
			"the config so subsequent `cartui` runs use the offline backend.\n\n" +
			"Available regions:" + listPBFRegionsHelp(),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPBFDownload(cmd.Context(), args[0])
		},
	}
	return cmd
}

func listPBFRegionsHelp() string {
	out := "\n"
	for _, r := range pbfRegions {
		out += fmt.Sprintf("  %-10s  %s — %s\n", r.Key, r.Name, r.Note)
	}
	return out
}

func runPBFDownload(ctx context.Context, key string) error {
	var region *pbfRegion
	for i := range pbfRegions {
		if pbfRegions[i].Key == key {
			region = &pbfRegions[i]
			break
		}
	}
	if region == nil {
		return fmt.Errorf("unknown region %q (try: aquitaine, france, europe, planet)", key)
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cacheDir := pbfCacheDir()
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return fmt.Errorf("mkdir cache: %w", err)
	}
	dst := filepath.Join(cacheDir, region.Key+".osm.pbf")

	fmt.Printf("Downloading %s…\n", region.Name)
	fmt.Printf("  source: %s\n", region.URL)
	fmt.Printf("  target: %s\n", dst)
	fmt.Println()

	if err := streamDownload(ctx, region.URL, dst); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	cfg.Providers.PBFPath = dst
	if err := persistConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	st, _ := os.Stat(dst)
	fmt.Printf("\n✓ Saved %.1f MB to %s\n", float64(st.Size())/(1<<20), dst)
	fmt.Printf("✓ providers.pbf_path written to config.toml\n")
	fmt.Printf("\nNext run of `cartui` will load this PBF (~%s init time).\n", region.Note)
	return nil
}

// streamDownload copies the URL body to dst with periodic progress.
func streamDownload(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream %s", resp.Status)
	}
	total := resp.ContentLength

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	pr := &progressReader{r: resp.Body, total: total, last: time.Now()}
	if _, err := io.Copy(out, pr); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// progressReader prints a one-line download progress every 500ms.
type progressReader struct {
	r     io.Reader
	read  int64
	total int64
	last  time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if time.Since(p.last) > 500*time.Millisecond {
		p.last = time.Now()
		if p.total > 0 {
			pct := float64(p.read) / float64(p.total) * 100
			fmt.Printf("\r  %.0f%%  %d / %d MB         ",
				pct, p.read>>20, p.total>>20)
		} else {
			fmt.Printf("\r  %d MB downloaded            ", p.read>>20)
		}
	}
	return n, err
}

// pbfCacheDir is `${XDG_CACHE_HOME:-~/.cache}/cartui/pbf`.
func pbfCacheDir() string {
	if env := os.Getenv("XDG_CACHE_HOME"); env != "" {
		return filepath.Join(env, "cartui", "pbf")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache/cartui/pbf"
	}
	return filepath.Join(home, ".cache", "cartui", "pbf")
}

// persistConfig writes the in-memory config back to ~/.config/cartui/config.toml
// using the same encoder the setup command uses.
func persistConfig(cfg config.Config) error {
	dir, err := config.DefaultDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return writeConfigTOML(filepath.Join(dir, "config.toml"), cfg)
}
