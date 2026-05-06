# Contributing to CarTUI

Thanks for your interest! CarTUI is a small project but takes its
hygiene seriously. Read this once, then file your PR.

## Quick rules

- **Conventional Commits** for every commit on every branch.
  `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`,
  `perf:`, `ci:`. Prefix with a scope when it helps:
  `feat(render): …`.
- **DCO sign-off** every commit (`git commit -s`). By signing off you
  agree to the [Developer Certificate of Origin
  1.1](https://developercertificate.org/).
- **AGPL-3.0-or-later** SPDX header at the top of every Go file:
  ```go
  // SPDX-FileCopyrightText: 2026 Cycl0o0 <contact@cyclooo.fr>
  // SPDX-License-Identifier: AGPL-3.0-or-later
  ```
  Replace the copyright year and email when you add files of your own.
- **`make test lint`** must pass before you push.
- **No new external services** without prior discussion. CarTUI's value
  proposition is "open data, no telemetry, no surprise calls".

## Development workflow

```bash
git clone https://github.com/cycl0o0/cartui.git
cd cartui
make build               # produces ./cartui
make test                # race detector + coverage
make lint                # golangci-lint (or fallback go vet)
make run                 # build + launch on Bordeaux
```

### Working on a feature

```bash
git switch -c feat/short-topic
# … edit, run make test/lint regularly …
git add -p
git commit -s -m "feat(render): describe the change"
git push -u origin feat/short-topic
gh pr create --fill --web
```

### Working on a bug fix

Add a regression test before the fix when at all possible. The test
should fail on `main` and pass with your patch.

## Code style

- Follow [Effective Go](https://go.dev/doc/effective_go) and the
  [Google Go Style Guide](https://google.github.io/styleguide/go/).
- `gofmt -s` + `goimports` are mandatory; the editor or `make fmt`
  takes care of it.
- Wrap errors with `fmt.Errorf("op: %w", err)`. Never `panic` from
  library code. The TUI may surface errors via `notify`.
- Doc-comment every exported symbol (`go doc` should yield useful
  output).
- Prefer composition over interfaces; introduce an interface only when
  there is more than one implementation.
- Tests go in `_test.go` next to the code; testify is OK.

## Adding new providers

Use `internal/providers/client.go` as the HTTP gateway — it owns the
rate-limiting, retry and User-Agent. Implement your provider as a small
wrapper that calls `Client.GetJSON` / `Client.RequestJSON`. Mock the
upstream with `httptest.NewServer` in tests.

## Releasing

The `Release` GitHub Action triggers on tags matching `v*`. Tag from
main:

```bash
git switch main
git pull
git tag -s -a v0.x.y -m "v0.x.y"
git push origin v0.x.y
```

GoReleaser builds Linux/macOS/Windows binaries and drafts the GitHub
release.

## Code of conduct

Be kind. Critique code, not people. Project maintainers reserve the
right to remove unwelcoming behaviour without notice.
