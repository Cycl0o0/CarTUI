// SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"encoding/json"
	"fmt"
	"io"
)

// decodeJSON wraps json.NewDecoder with a friendlier error message that
// includes the response prefix when parsing fails. It also enforces strict
// "no trailing content" semantics — APIs always return a single JSON value.
func decodeJSON(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}
