# Contributing to Rainforest

Thank you for your interest in contributing to **Rainforest**!

## Getting Started

```bash
git clone https://github.com/thedavidweng/rainforest.git
cd rainforest
mise install
mise run check
```

Requires Go `1.27+` (automatically managed when using [mise](https://mise.jdx.dev/)). `mise.toml` pins the compiler used by CI and release builds, and `GOTOOLCHAIN=local` prevents Go from downloading a different toolchain. The `go` directive in `go.mod` is the minimum supported Go version, not the compiler pin. The MyGo CLI is pinned as a Go tool: run it as `go tool mygo`. There is no Node or bun toolchain.

## Development Workflow

```bash
# Run all read-only quality gates: fmt-check, mod-tidy-check, vet, build, test, test-race, lint, conventions, i18n-check
mise run check

# Individual tasks
mise run fmt             # Format Go code with gofumpt -extra -w .
mise run fmt-check       # Verify Go code formatting
mise run mod-tidy-check  # Verify go.mod and go.sum have no tidy drift
mise run vet             # Run go vet ./...
mise run build           # Compile all Go packages
mise run test            # Run Go unit and contract tests
mise run test-race       # Run Go tests with race detector
mise run lint            # Run golangci-lint
mise run conventions     # Check agent-file rules and <= 5% Go comment budget
mise run i18n-check      # Verify locales/catalog_gen.go matches `go tool mygo generate`
mise run icons           # Sync internal/view/icons from icons.txt
```

When modifying any method signature or DTO on `internal/gui` services (`Auth`, `Transfers`, `Backups`, `Downloads`, `Settings`), update the matching pages in `internal/view` and their tests. Views call the services directly, so there are no bindings to regenerate.

When editing `locales/*.json` (keep `en` and `zh-Hans` in sync), regenerate the typed accessors:

```bash
go tool mygo generate
```

View tests run headless with `ui.NewTester` against real `gui.Services` backed by temporary SQLite databases and stub HTTP transports (see `internal/view/harness_test.go`).

## Icons

Icons are [Phosphor](https://phosphoricons.com) SVGs in `internal/view/icons/`, pinned by `internal/view/icons/icons.txt` (first line is the `@phosphor-icons/core` version, then one `weight name` line per icon, such as `regular folder` or `fill house`; non-regular weights are saved as `<name>-<weight>.svg`). To add one, append its line to `icons.txt`, run `mise run icons`, and commit the downloaded SVG. Never hand-edit the SVGs.

## Project Structure

```text
main.go                         Thin MyGo desktop shell & entrypoint (window, tray, notifications, file pickers)
tray_icon.go                    Embedded 32x32 PNG tray icon
mygo.config.ts                  MyGo CLI configuration
internal/
  credstore/                    OS-protected credential vault (0600 / icacls) & cookie parser
  store/                        SQLite persistence (settings, backup_rules, tracked_files, transfers)
  engine/                       Upload planner, transfer queue, one-way backup scanner, reconciliation
  gui/                          Framework-neutral GUI facade, DTOs, error sanitizer, tray/quit state
  view/                         Native MyGo UI (mygo/ui): Welcome, Upload, Activity, Backup, Restore, Settings
    icons/                      Embedded Phosphor SVGs (MIT)
locales/                        en / zh-Hans ICU catalogs and generated accessors
docs/
  tutorials/                    Learning-oriented step-by-step tutorials
  how-to/                       Task-oriented operational guides
  reference/                    Supported platforms, settings, media types, states, regions
  explanation/                  Product scope, architecture, backup safety, reconciliation, storage
  adr/                          Architecture Decision Records
  agents/                       Agent workflow, issue tracker, and domain conventions
```

## Engineering Rules

1. **Strict Product Scope**: Rainforest is a desktop upload, one-way folder backup, transfer queue, and lightweight recovery download client. Do not add web-gallery pages (no cloud photo wall, timeline browser, AI search, album/folder CRUD manager, or trash UI).
2. **Framework Isolation**: `internal/credstore`, `internal/store`, `internal/engine`, `internal/core`, and `internal/gui` must **never** import MyGo. `internal/view` imports only `github.com/egoist/mygo/ui`; the root `github.com/egoist/mygo` package belongs only in the root desktop shell (`main.go`).
3. **One-Way Backup Safety**: Local file deletion, move, rename, or offline external drives (`source_unavailable`) must never trigger remote cloud deletion (`MoveToTrash`). Uncertain uploads must be marked `unconfirmed` and reconciled before retry.
4. **Test-First & Offline Determinism**: Write Go tests against injected `http.RoundTripper` mocks and SQLite temp DBs, and view tests against `ui.NewTester`. Never require live Amazon credentials in CI or commit real session cookies.
5. **Comment Budget & Single Agent File**: Keep code self-explanatory (`<= 5%` comment lines in non-test Go files). `AGENTS.md` is the only allowed agent instruction file.
6. **Secret Redaction**: All user-visible error messages and diagnostic outputs must pass through `aphotos.Redact`.

## Pull Requests

1. Fork the repository and create a topic branch.
2. Add or update unit/contract tests in Go (`internal/...`, `main_test.go`) and views (`internal/view/*_test.go`).
3. Ensure `mise run check` passes with zero warnings and `git diff --exit-code` is clean.
4. Open a pull request referencing the relevant issue.

## License

By contributing to Rainforest, you agree that your contributions will be licensed under the **GNU Affero General Public License v3.0 only (`AGPL-3.0-only`)** as set forth in [LICENSE](LICENSE).
