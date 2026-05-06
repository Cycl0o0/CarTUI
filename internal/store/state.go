// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"encoding/json"
	"errors"

	"github.com/cycl0o0/cartui/internal/geo"
	bolt "go.etcd.io/bbolt"
)

// Viewport is the persisted last-known map state.
type Viewport struct {
	Center geo.LatLng `json:"center"`
	Zoom   int        `json:"zoom"`
}

var stateKeyView = []byte("last_view")

// SaveViewport persists the last-known viewport so the next launch reopens
// where the user left off.
func (d *DB) SaveViewport(v Viewport) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return d.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketState).Put(stateKeyView, raw)
	})
}

// LoadViewport returns the persisted viewport, or [ErrNotFound] when no
// session has been saved yet.
func (d *DB) LoadViewport() (Viewport, error) {
	var v Viewport
	err := d.bolt.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketState).Get(stateKeyView)
		if raw == nil {
			return ErrNotFound
		}
		return json.Unmarshal(raw, &v)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return v, err
		}
		return v, err
	}
	return v, nil
}
