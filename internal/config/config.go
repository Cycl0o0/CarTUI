// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config wires Viper to the CarTUI options. It exposes a single
// [Config] struct and a single loader [Load]; defaults are sensible and the
// expected file location is the XDG config directory.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// UI holds visual preferences.
type UI struct {
	Theme   string `mapstructure:"theme"`
	Sidebar bool   `mapstructure:"sidebar"`
	Lang    string `mapstructure:"lang"`
}

// Map holds the default viewport and renderer toggles.
type Map struct {
	DefaultLat  float64 `mapstructure:"default_lat"`
	DefaultLng  float64 `mapstructure:"default_lng"`
	DefaultZoom int     `mapstructure:"default_zoom"`
	Braille     bool    `mapstructure:"braille"`
}

// Providers holds API endpoints and routing profile.
type Providers struct {
	NominatimURL string `mapstructure:"nominatim_url"`
	OverpassURL  string `mapstructure:"overpass_url"`
	OSRMURL      string `mapstructure:"osrm_url"`
	OSRMProfile  string `mapstructure:"osrm_profile"`
	TileURL      string `mapstructure:"tile_url"`

	Rate ProvidersRate `mapstructure:"rate"`
}

// ProvidersRate holds the per-host requests-per-second limits.
type ProvidersRate struct {
	NominatimRPS float64 `mapstructure:"nominatim_rps"`
	OverpassRPS  float64 `mapstructure:"overpass_rps"`
	OSRMRPS      float64 `mapstructure:"osrm_rps"`
	TileRPS      float64 `mapstructure:"tile_rps"`
}

// Cache holds time-to-live values for cached resources.
type Cache struct {
	TileTTLHours        int `mapstructure:"tile_ttl_hours"`
	OverpassTTLMinutes int `mapstructure:"overpass_ttl_minutes"`
}

// Network groups timeout/retry knobs.
type Network struct {
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
	Retries        int `mapstructure:"retries"`
}

// Config is the top-level user configuration.
type Config struct {
	UI        UI        `mapstructure:"ui"`
	Map       Map       `mapstructure:"map"`
	Providers Providers `mapstructure:"providers"`
	Cache     Cache     `mapstructure:"cache"`
	Network   Network   `mapstructure:"network"`

	// DBPath optionally overrides the on-disk store location.
	DBPath string `mapstructure:"db_path"`
}

// Defaults returns the baked-in defaults — sensible for a first-run experience
// in Bordeaux 🌊.
func Defaults() Config {
	return Config{
		UI: UI{
			Theme:   "dark",
			Sidebar: true,
			Lang:    "fr",
		},
		Map: Map{
			DefaultLat:  44.8378,
			DefaultLng:  -0.5792,
			DefaultZoom: 13,
			Braille:     true,
		},
		Providers: Providers{
			NominatimURL: "https://nominatim.openstreetmap.org/",
			OverpassURL:  "https://overpass-api.de/api/interpreter",
			OSRMURL:      "https://router.project-osrm.org/",
			OSRMProfile:  "driving",
			TileURL:      "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
			Rate: ProvidersRate{
				NominatimRPS: 1.0,
				OverpassRPS:  0.5,
				OSRMRPS:      5.0,
				TileRPS:      2.0,
			},
		},
		Cache: Cache{
			TileTTLHours:       168,
			OverpassTTLMinutes: 60,
		},
		Network: Network{
			TimeoutSeconds: 15,
			Retries:        3,
		},
	}
}

// Load reads the configuration from `~/.config/cartui/config.toml` (or
// `$XDG_CONFIG_HOME/cartui/config.toml` when set) and merges it with
// defaults. Environment variables prefixed with `CARTUI_` override file
// settings; e.g. `CARTUI_MAP_DEFAULT_ZOOM=10`. The optional `path` argument
// short-circuits the lookup.
func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetEnvPrefix("CARTUI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := Defaults()
	bindDefaults(v, cfg)

	if path == "" {
		dir, err := DefaultDir()
		if err == nil {
			v.AddConfigPath(dir)
			v.SetConfigName("config")
		}
	} else {
		v.SetConfigFile(path)
	}
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !asConfigNotFound(err, &notFound) {
			// Genuine parse/read error: surface it.
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		// Missing config file is fine — defaults stand.
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

// asConfigNotFound is a tiny shim for errors.As against viper's typed
// "config file not found" error. Returning a typed bool keeps the error
// chain inspection short.
func asConfigNotFound(err error, target *viper.ConfigFileNotFoundError) bool {
	if err == nil {
		return false
	}
	if t, ok := err.(viper.ConfigFileNotFoundError); ok {
		*target = t
		return true
	}
	return false
}

// bindDefaults seeds Viper with the defaults so AutomaticEnv resolution sees
// every key. Without this an `env`-only override would not be applied.
func bindDefaults(v *viper.Viper, cfg Config) {
	v.SetDefault("ui.theme", cfg.UI.Theme)
	v.SetDefault("ui.sidebar", cfg.UI.Sidebar)
	v.SetDefault("ui.lang", cfg.UI.Lang)
	v.SetDefault("map.default_lat", cfg.Map.DefaultLat)
	v.SetDefault("map.default_lng", cfg.Map.DefaultLng)
	v.SetDefault("map.default_zoom", cfg.Map.DefaultZoom)
	v.SetDefault("map.braille", cfg.Map.Braille)
	v.SetDefault("providers.nominatim_url", cfg.Providers.NominatimURL)
	v.SetDefault("providers.overpass_url", cfg.Providers.OverpassURL)
	v.SetDefault("providers.osrm_url", cfg.Providers.OSRMURL)
	v.SetDefault("providers.osrm_profile", cfg.Providers.OSRMProfile)
	v.SetDefault("providers.tile_url", cfg.Providers.TileURL)
	v.SetDefault("providers.rate.nominatim_rps", cfg.Providers.Rate.NominatimRPS)
	v.SetDefault("providers.rate.overpass_rps", cfg.Providers.Rate.OverpassRPS)
	v.SetDefault("providers.rate.osrm_rps", cfg.Providers.Rate.OSRMRPS)
	v.SetDefault("providers.rate.tile_rps", cfg.Providers.Rate.TileRPS)
	v.SetDefault("cache.tile_ttl_hours", cfg.Cache.TileTTLHours)
	v.SetDefault("cache.overpass_ttl_minutes", cfg.Cache.OverpassTTLMinutes)
	v.SetDefault("network.timeout_seconds", cfg.Network.TimeoutSeconds)
	v.SetDefault("network.retries", cfg.Network.Retries)
}

// DefaultDir returns the directory where Cartui looks for its config file.
func DefaultDir() (string, error) {
	if env := os.Getenv("XDG_CONFIG_HOME"); env != "" {
		return filepath.Join(env, "cartui"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cartui"), nil
}

// Timeout is a convenience accessor returning the configured network timeout.
func (c Config) Timeout() time.Duration {
	return time.Duration(c.Network.TimeoutSeconds) * time.Second
}

// TileTTL exposes the cache duration for raster tiles.
func (c Config) TileTTL() time.Duration {
	return time.Duration(c.Cache.TileTTLHours) * time.Hour
}

// OverpassTTL exposes the cache duration for Overpass responses.
func (c Config) OverpassTTL() time.Duration {
	return time.Duration(c.Cache.OverpassTTLMinutes) * time.Minute
}
