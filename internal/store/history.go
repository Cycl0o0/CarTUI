// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cycl0o0/cartui/internal/data"
	bolt "go.etcd.io/bbolt"
)

// AppendHistory records a search event. The key is the RFC 3339 nanosecond
// timestamp, which keeps the cursor iterating in chronological order.
func (d *DB) AppendHistory(e data.HistoryEntry) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}
	key := []byte(e.CreatedAt.UTC().Format(time.RFC3339Nano))
	return d.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketHistory).Put(key, raw)
	})
}

// History returns the most recent search entries up to the given limit (most
// recent first). A zero or negative limit returns all entries.
func (d *DB) History(limit int) ([]data.HistoryEntry, error) {
	var out []data.HistoryEntry
	err := d.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketHistory).Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var e data.HistoryEntry
			if err := json.Unmarshal(v, &e); err != nil {
				continue
			}
			out = append(out, e)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		return nil
	})
	return out, err
}

// ClearHistory removes every recorded search.
func (d *DB) ClearHistory() error {
	return d.bolt.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketHistory); err != nil {
			return err
		}
		_, err := tx.CreateBucket(bucketHistory)
		return err
	})
}
