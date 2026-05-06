// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tiles

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func newTestBolt(t *testing.T) *bolt.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := bolt.Open(filepath.Join(dir, "tiles.db"), 0o600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCacheRoundTrip(t *testing.T) {
	bdb := newTestBolt(t)
	c, err := NewCache(bdb, time.Hour)
	require.NoError(t, err)

	a := Address{Z: 13, X: 1, Y: 2}
	_, err = c.Get(a)
	assert.True(t, errors.Is(err, ErrCacheMiss))

	require.NoError(t, c.Put(a, []byte("hello")))
	got, err := c.Get(a)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), got)
}

func TestCacheExpired(t *testing.T) {
	bdb := newTestBolt(t)
	c, err := NewCache(bdb, time.Nanosecond)
	require.NoError(t, err)

	a := Address{Z: 13, X: 1, Y: 2}
	require.NoError(t, c.Put(a, []byte("x")))
	time.Sleep(10 * time.Millisecond)

	_, err = c.Get(a)
	assert.True(t, errors.Is(err, ErrCacheExpired))
}

func TestCacheStats(t *testing.T) {
	bdb := newTestBolt(t)
	c, err := NewCache(bdb, time.Hour)
	require.NoError(t, err)
	require.NoError(t, c.Put(Address{Z: 1, X: 1, Y: 1}, []byte("a")))
	require.NoError(t, c.Put(Address{Z: 1, X: 1, Y: 2}, []byte("b")))
	n, err := c.Stats()
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}
