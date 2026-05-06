// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cycl0o0/cartui/internal/data"
	bolt "go.etcd.io/bbolt"
)

// SaveBookmark persists a bookmark, generating an ID and CreatedAt when they
// are unset. The (possibly mutated) bookmark is returned for the caller to
// keep in memory.
func (d *DB) SaveBookmark(b data.Bookmark) (data.Bookmark, error) {
	if b.ID == "" {
		b.ID = newID()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return data.Bookmark{}, fmt.Errorf("marshal bookmark: %w", err)
	}
	err = d.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketBookmarks).Put([]byte(b.ID), raw)
	})
	if err != nil {
		return data.Bookmark{}, fmt.Errorf("put bookmark: %w", err)
	}
	return b, nil
}

// DeleteBookmark removes a bookmark by ID. Deleting a non-existent ID is a
// no-op (no error).
func (d *DB) DeleteBookmark(id string) error {
	return d.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketBookmarks).Delete([]byte(id))
	})
}

// ListBookmarks returns every bookmark, ordered by CreatedAt descending.
// The returned slice is fresh and safe to mutate.
func (d *DB) ListBookmarks() ([]data.Bookmark, error) {
	var out []data.Bookmark
	err := d.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketBookmarks).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var b data.Bookmark
			if err := json.Unmarshal(v, &b); err != nil {
				continue
			}
			out = append(out, b)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Sort by CreatedAt desc; older items toward the end.
	sortBookmarks(out)
	return out, nil
}

// GetBookmark returns the bookmark with the given ID or [ErrNotFound].
func (d *DB) GetBookmark(id string) (data.Bookmark, error) {
	var out data.Bookmark
	err := d.bolt.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketBookmarks).Get([]byte(id))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &out)
	})
	return out, err
}

// newID returns a 16-byte hex identifier prefixed with the current Unix time
// (8 bytes big-endian). This keeps cursor iteration roughly chronological
// without forcing a secondary index.
func newID() string {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], uint64(time.Now().UnixNano()))
	if _, err := rand.Read(buf[8:]); err != nil {
		// crypto/rand failures are catastrophic; fall back to a
		// deterministic suffix rather than panicking inside the TUI.
		for i := 8; i < 16; i++ {
			buf[i] = byte(i)
		}
	}
	return hex.EncodeToString(buf[:])
}

// sortBookmarks orders bookmarks by CreatedAt descending in place.
func sortBookmarks(b []data.Bookmark) {
	// In-place insertion sort — simpler than pulling sort.Slice for the
	// modest list sizes typical here (< 1000 entries).
	for i := 1; i < len(b); i++ {
		v := b[i]
		j := i - 1
		for j >= 0 && b[j].CreatedAt.Before(v.CreatedAt) {
			b[j+1] = b[j]
			j--
		}
		b[j+1] = v
	}
}
