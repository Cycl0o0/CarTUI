// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@example.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringIncludesVersion(t *testing.T) {
	out := String()
	assert.Contains(t, out, "CarTUI")
	assert.Contains(t, out, Version)
}

func TestUserAgentFormat(t *testing.T) {
	ua := UserAgent()
	assert.True(t, strings.HasPrefix(ua, "CarTUI/"))
	assert.Contains(t, ua, "github.com/cycl0o0/cartui")
}
