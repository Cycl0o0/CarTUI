// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForFrenchDefault(t *testing.T) {
	s := For("")
	assert.Equal(t, "RECHERCHE", s.ModeSearch)
	assert.Contains(t, s.Bookmarks, "Favoris")
}

func TestForEnglish(t *testing.T) {
	s := For("en")
	assert.Equal(t, "SEARCH", s.ModeSearch)
	assert.Contains(t, s.Bookmarks, "Bookmarks")
}

func TestForFallsBackToFrench(t *testing.T) {
	s := For("zz")
	assert.Equal(t, "RECHERCHE", s.ModeSearch)
}

func TestForCaseInsensitive(t *testing.T) {
	s := For("EN")
	assert.Equal(t, "SEARCH", s.ModeSearch)
}
