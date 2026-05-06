// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

// bucketOverpass is the Bolt bucket holding cached Overpass responses.
var bucketOverpass = []byte("overpass")

// overpassEntry wraps a cached blob with its fetched-at timestamp so the
// cache can enforce a TTL.
type overpassEntry struct {
	FetchedAt time.Time `json:"fetched_at"`
	Blob      []byte    `json:"blob"`
}

// OverpassCache is a Bolt-backed cache implementing the
// [providers.OverpassCache] contract.
type OverpassCache struct {
	db  *DB
	ttl time.Duration
}

// NewOverpassCache attaches a cache to the database. ttl ≤ 0 means
// "never expire".
func (d *DB) NewOverpassCache(ttl time.Duration) (*OverpassCache, error) {
	err := d.bolt.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketOverpass)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &OverpassCache{db: d, ttl: ttl}, nil
}

// Get returns the cached blob, or (nil, false) on miss / expiry.
func (c *OverpassCache) Get(key string) ([]byte, bool) {
	var entry overpassEntry
	err := c.db.bolt.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketOverpass).Get([]byte(key))
		if raw == nil {
			return ErrNotFound
		}
		return json.Unmarshal(raw, &entry)
	})
	if err != nil {
		return nil, false
	}
	if c.ttl > 0 && time.Since(entry.FetchedAt) > c.ttl {
		return nil, false
	}
	return entry.Blob, true
}

// Put stores a blob under key with a now() timestamp. Errors are
// silently swallowed — a cache failure must never abort a successful
// fetch.
func (c *OverpassCache) Put(key string, blob []byte) {
	raw, err := json.Marshal(overpassEntry{FetchedAt: time.Now().UTC(), Blob: blob})
	if err != nil {
		return
	}
	_ = c.db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketOverpass).Put([]byte(key), raw)
	})
}
