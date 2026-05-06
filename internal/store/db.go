// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store wraps the local BoltDB persistence layer used by CarTUI for
// bookmarks, search history, raster tile cache and last-known-good viewport
// state. The package is deliberately thin: every type maps to a single
// bucket and exposes synchronous Get/Put/Delete/List helpers.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names. Exposed for callers that want raw access (e.g. backups).
var (
	bucketBookmarks = []byte("bookmarks")
	bucketHistory   = []byte("history")
	bucketTiles     = []byte("tiles")
	bucketState     = []byte("state")
)

// ErrNotFound is returned by Get-style methods when the requested key has no
// value in the underlying bucket.
var ErrNotFound = errors.New("store: not found")

// DB is the application's persistence handle. Safe for concurrent use across
// goroutines (Bolt's locking rules apply: any number of read transactions, a
// single write transaction at a time).
type DB struct {
	bolt *bolt.DB
	path string
}

// Open opens (and creates if needed) the database at path. Parent directories
// are created with mode 0o755.
//
// Pass an empty path to use the default XDG-friendly location:
// `${XDG_CACHE_HOME:-$HOME/.cache}/cartui/cartui.db`.
func Open(path string) (*DB, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	bdb, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open boltdb: %w", err)
	}
	if err := bdb.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketBookmarks, bucketHistory, bucketTiles, bucketState} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("create bucket %s: %w", b, err)
			}
		}
		return nil
	}); err != nil {
		_ = bdb.Close()
		return nil, err
	}
	return &DB{bolt: bdb, path: path}, nil
}

// Close releases the underlying Bolt handle.
func (d *DB) Close() error {
	if d == nil || d.bolt == nil {
		return nil
	}
	return d.bolt.Close()
}

// Path returns the on-disk location of the database file.
func (d *DB) Path() string { return d.path }

// DefaultPath builds the OS-appropriate default database path.
func DefaultPath() (string, error) {
	if env := os.Getenv("XDG_CACHE_HOME"); env != "" {
		return filepath.Join(env, "cartui", "cartui.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".cache", "cartui", "cartui.db"), nil
}
