// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command cartui is the entrypoint for the CarTUI terminal mapping app. It
// wires the configuration, persistence layer and HTTP providers together,
// then hands control to the Bubble Tea event loop in `internal/tui`.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/cartui/internal/config"
	"github.com/cycl0o0/cartui/internal/providers"
	"github.com/cycl0o0/cartui/internal/store"
	"github.com/cycl0o0/cartui/internal/tui"
	"github.com/cycl0o0/cartui/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "cartui:", err)
		os.Exit(1)
	}
}

// flags collects the command-line overrides; values feed into the config.
type flags struct {
	configPath string
	logFile    string
	logLevel   string
	theme      string
	lang       string
	noBraille  bool
	startLat   float64
	startLng   float64
	startZoom  int
	gotoQuery  string
	versionOut bool
}

func newRootCmd() *cobra.Command {
	f := &flags{}
	cmd := &cobra.Command{
		Use:           "cartui",
		Short:         "Terminal-native cartography (OpenStreetMap)",
		Long:          "CarTUI brings interactive maps, search, POIs and routing to your terminal — backed by Nominatim, Overpass and OSRM.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.versionOut {
				fmt.Println(version.String())
				return nil
			}
			return run(cmd.Context(), *f)
		},
	}
	cmd.Flags().StringVar(&f.configPath, "config", "", "config file (default ~/.config/cartui/config.toml)")
	cmd.Flags().StringVar(&f.logFile, "log", "", "log file (default: discard logs in TUI mode)")
	cmd.Flags().StringVar(&f.logLevel, "log-level", "info", "log level: debug|info|warn|error")
	cmd.Flags().StringVar(&f.theme, "theme", "", "override theme: dark|light|mono")
	cmd.Flags().StringVar(&f.lang, "lang", "", "override language: fr|en")
	cmd.Flags().BoolVar(&f.noBraille, "ascii", false, "force ASCII rendering instead of Braille")
	cmd.Flags().Float64Var(&f.startLat, "lat", 0, "starting latitude (overrides config)")
	cmd.Flags().Float64Var(&f.startLng, "lng", 0, "starting longitude (overrides config)")
	cmd.Flags().IntVar(&f.startZoom, "zoom", 0, "starting zoom level (overrides config)")
	cmd.Flags().StringVar(&f.gotoQuery, "goto", "", "geocode this string and centre on the first result before launching")
	cmd.Flags().BoolVarP(&f.versionOut, "version", "V", false, "print version and exit")
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newPrefetchCmd())
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(version.String())
		},
	}
}

func run(ctx context.Context, f flags) error {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	applyFlagOverrides(&cfg, f)

	logger, closeLog, err := buildLogger(f.logFile, f.logLevel)
	if err != nil {
		return err
	}
	defer closeLog()
	slog.SetDefault(logger)
	logger.Info("starting cartui", "version", version.Version)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Warn("opening store failed; continuing without persistence", "err", err)
	}
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()

	clientOpts := providers.ClientOptions{
		Timeout:    cfg.Timeout(),
		MaxRetries: cfg.Network.Retries,
	}
	httpClient := providers.NewClient(clientOpts)
	httpClient.SetRateLimit(hostOf(cfg.Providers.NominatimURL), cfg.Providers.Rate.NominatimRPS)
	httpClient.SetRateLimit(hostOf(cfg.Providers.OSRMURL), cfg.Providers.Rate.OSRMRPS)
	httpClient.SetRateLimit(hostOf(cfg.Providers.TileURL), cfg.Providers.Rate.TileRPS)
	// Apply the Overpass rate limit to every endpoint in the rotation —
	// hitting four mirrors round-robin still has to respect their
	// individual fair-use policies.
	overpassEndpoints := providers.DefaultOverpassEndpoints
	if cfg.Providers.OverpassURL != "" {
		overpassEndpoints = strings.Split(cfg.Providers.OverpassURL, ",")
	}
	for _, ep := range overpassEndpoints {
		if h := hostOf(strings.TrimSpace(ep)); h != "" {
			httpClient.SetRateLimit(h, cfg.Providers.Rate.OverpassRPS)
		}
	}

	overpass := providers.NewOverpass(httpClient, cfg.Providers.OverpassURL)
	if db != nil {
		if cache, err := db.NewOverpassCache(cfg.OverpassTTL()); err == nil {
			overpass.SetCache(cache, cfg.OverpassTTL())
		} else {
			logger.Warn("overpass cache unavailable", "err", err)
		}
	}

	deps := tui.Deps{
		Cfg:       cfg,
		Store:     db,
		Nominatim: providers.NewNominatim(httpClient, cfg.Providers.NominatimURL),
		Overpass:  overpass,
		OSRM:      providers.NewOSRM(httpClient, cfg.Providers.OSRMURL),
		TomTom:    providers.NewTomTom(httpClient, cfg.Providers.TomTomURL, cfg.Providers.TomTomAPIKey),
	}

	if f.gotoQuery != "" {
		if err := geocodeAndApply(ctx, deps.Nominatim, f.gotoQuery, &deps.Cfg, cfg.UI.Lang); err != nil {
			logger.Warn("geocode goto failed", "err", err)
		}
	}

	app := tui.New(deps)
	prog := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Forward SIGINT / SIGTERM to a graceful program quit.
	notifyCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-notifyCtx.Done()
		prog.Quit()
	}()

	_, err = prog.Run()
	return err
}

func applyFlagOverrides(cfg *config.Config, f flags) {
	if f.theme != "" {
		cfg.UI.Theme = f.theme
	}
	if f.lang != "" {
		cfg.UI.Lang = f.lang
	}
	if f.noBraille {
		cfg.Map.Braille = false
	}
	if f.startLat != 0 {
		cfg.Map.DefaultLat = f.startLat
	}
	if f.startLng != 0 {
		cfg.Map.DefaultLng = f.startLng
	}
	if f.startZoom > 0 {
		cfg.Map.DefaultZoom = f.startZoom
	}
}

func geocodeAndApply(ctx context.Context, n *providers.Nominatim, query string, cfg *config.Config, lang string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	res, err := n.Search(ctx, query, providers.SearchOptions{Limit: 1, Language: lang})
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("no result for %q", query)
	}
	cfg.Map.DefaultLat = res[0].Position.Lat
	cfg.Map.DefaultLng = res[0].Position.Lng
	return nil
}

func buildLogger(file, level string) (*slog.Logger, func(), error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	if file == "" {
		// Discard logs by default in TUI mode — printing to stderr would
		// corrupt the terminal output.
		return slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: lvl})), func() {}, nil
	}
	w, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log: %w", err)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})), func() { _ = w.Close() }, nil
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// hostOf returns the host (with optional port) of u, or "" when u is invalid.
func hostOf(u string) string {
	for i := 0; i+3 < len(u); i++ {
		if u[i] == ':' && u[i+1] == '/' && u[i+2] == '/' {
			rest := u[i+3:]
			for j := 0; j < len(rest); j++ {
				if rest[j] == '/' {
					return rest[:j]
				}
			}
			return rest
		}
	}
	return ""
}
