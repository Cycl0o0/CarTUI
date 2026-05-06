// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cycl0o0/cartui/internal/data"
	"github.com/cycl0o0/cartui/internal/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestBookmarksRoundTrip(t *testing.T) {
	db := newTestDB(t)
	b := data.Bookmark{
		Name:     "Bordeaux",
		Position: geo.LatLng{Lat: 44.8378, Lng: -0.5792},
	}
	saved, err := db.SaveBookmark(b)
	require.NoError(t, err)
	require.NotEmpty(t, saved.ID)
	assert.False(t, saved.CreatedAt.IsZero())

	got, err := db.GetBookmark(saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bordeaux", got.Name)
}

func TestBookmarkNotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := db.GetBookmark("missing")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestListBookmarksOrderedDesc(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		_, err := db.SaveBookmark(data.Bookmark{
			Name:      "p",
			CreatedAt: now.Add(time.Duration(i) * time.Hour),
		})
		require.NoError(t, err)
	}
	list, err := db.ListBookmarks()
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.True(t, list[0].CreatedAt.After(list[1].CreatedAt))
	assert.True(t, list[1].CreatedAt.After(list[2].CreatedAt))
}

func TestDeleteBookmark(t *testing.T) {
	db := newTestDB(t)
	saved, _ := db.SaveBookmark(data.Bookmark{Name: "x"})
	require.NoError(t, db.DeleteBookmark(saved.ID))
	_, err := db.GetBookmark(saved.ID)
	assert.True(t, errors.Is(err, ErrNotFound))
	require.NoError(t, db.DeleteBookmark("nonexistent"))
}

func TestHistory(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, db.AppendHistory(data.HistoryEntry{
			Query:     "q",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}))
	}
	all, err := db.History(0)
	require.NoError(t, err)
	assert.Len(t, all, 5)

	limited, err := db.History(2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestClearHistory(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AppendHistory(data.HistoryEntry{Query: "x"}))
	require.NoError(t, db.ClearHistory())
	all, err := db.History(0)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestViewportRoundTrip(t *testing.T) {
	db := newTestDB(t)
	_, err := db.LoadViewport()
	assert.True(t, errors.Is(err, ErrNotFound))

	v := Viewport{Center: geo.LatLng{Lat: 44.8378, Lng: -0.5792}, Zoom: 13}
	require.NoError(t, db.SaveViewport(v))

	got, err := db.LoadViewport()
	require.NoError(t, err)
	assert.Equal(t, v, got)
}

func TestDefaultPath(t *testing.T) {
	p, err := DefaultPath()
	require.NoError(t, err)
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "cartui.db")
}
