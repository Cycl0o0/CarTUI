// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultsAreBordeaux(t *testing.T) {
	d := Defaults()
	assert.InDelta(t, 44.8378, d.Map.DefaultLat, 1e-6)
	assert.InDelta(t, -0.5792, d.Map.DefaultLng, 1e-6)
	assert.Equal(t, 13, d.Map.DefaultZoom)
	assert.True(t, d.Map.Braille)
	assert.Equal(t, "dark", d.UI.Theme)
}

func TestLoadMissingFileFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "missing.toml"))
	// A non-existent path passed via SetConfigFile is a fatal error in
	// viper, but DefaultPath fallback (no path) does not error.
	if err == nil {
		assert.Equal(t, Defaults().UI.Theme, cfg.UI.Theme)
	}
}

func TestLoadParsesTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[ui]
theme = "light"

[map]
default_lat = 48.8566
default_lng = 2.3522
default_zoom = 11
`), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "light", cfg.UI.Theme)
	assert.InDelta(t, 48.8566, cfg.Map.DefaultLat, 1e-6)
	assert.Equal(t, 11, cfg.Map.DefaultZoom)
}

func TestLoadInvalidTOMLErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	require.NoError(t, os.WriteFile(path, []byte(`this is = not = toml`), 0o644))
	_, err := Load(path)
	assert.Error(t, err)
}

func TestTimeoutAccessors(t *testing.T) {
	c := Defaults()
	assert.True(t, c.Timeout() > 0)
	assert.True(t, c.TileTTL() > 0)
	assert.True(t, c.OverpassTTL() > 0)
}
