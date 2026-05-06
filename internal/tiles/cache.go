// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// CacheBucket is the BoltDB bucket name used by the tile cache.
var CacheBucket = []byte("tiles")

// ErrCacheMiss is returned when a tile is absent from the cache.
var ErrCacheMiss = errors.New("tile cache miss")

// ErrCacheExpired is returned by Get when the cached entry is older than the
// configured TTL.
var ErrCacheExpired = errors.New("tile cache expired")

// CacheEntry is the value stored in BoltDB. The blob is the raw HTTP body of
// the upstream tile (typically a PNG).
type CacheEntry struct {
	FetchedAt time.Time `json:"fetched_at"`
	Blob      []byte    `json:"blob"`
}

// Cache is a Bolt-backed tile cache. The cache is shared with the rest of the
// app store; pass any open `*bolt.DB` to operate on it.
type Cache struct {
	db  *bolt.DB
	ttl time.Duration
}

// NewCache builds a cache wrapper. ttl ≤ 0 means "never expire".
func NewCache(db *bolt.DB, ttl time.Duration) (*Cache, error) {
	if db == nil {
		return nil, errors.New("tiles: nil bolt db")
	}
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(CacheBucket)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("tiles: ensure bucket: %w", err)
	}
	return &Cache{db: db, ttl: ttl}, nil
}

// Get returns the cached tile bytes or one of [ErrCacheMiss], [ErrCacheExpired].
func (c *Cache) Get(a Address) ([]byte, error) {
	var entry CacheEntry
	err := c.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(CacheBucket).Get([]byte(a.Key()))
		if v == nil {
			return ErrCacheMiss
		}
		return json.Unmarshal(v, &entry)
	})
	if err != nil {
		return nil, err
	}
	if c.ttl > 0 && time.Since(entry.FetchedAt) > c.ttl {
		return nil, ErrCacheExpired
	}
	return entry.Blob, nil
}

// Put stores the tile bytes. The fetched-at timestamp is set to time.Now().
func (c *Cache) Put(a Address, blob []byte) error {
	raw, err := json.Marshal(CacheEntry{FetchedAt: time.Now().UTC(), Blob: blob})
	if err != nil {
		return err
	}
	return c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(CacheBucket).Put([]byte(a.Key()), raw)
	})
}

// Stats returns the number of cached entries (used for the status bar).
func (c *Cache) Stats() (count int, err error) {
	err = c.db.View(func(tx *bolt.Tx) error {
		count = tx.Bucket(CacheBucket).Stats().KeyN
		return nil
	})
	return count, err
}
