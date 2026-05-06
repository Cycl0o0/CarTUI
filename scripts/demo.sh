#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Quick demo: build the binary and launch it on Bordeaux.

set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -x ./cartui ]]; then
  echo ">> building..."
  go build -o cartui ./cmd/cartui
fi

echo ">> launching CarTUI on Bordeaux (press q to quit)"
exec ./cartui --lat 44.8378 --lng -0.5792 --zoom 13 "$@"
