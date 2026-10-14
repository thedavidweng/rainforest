# Contributing to Rainforest

Thank you for your interest in contributing to **Rainforest**!

## Getting Started

```bash
git clone https://github.com/thedavidweng/rainforest.git
cd rainforest
mise install
bun install --cwd frontend
mise run check
```

Requires Go `1.27+` and Bun `1.4+` (automatically managed when using [mise](https://mise.jdx.dev/)).

## Development Workflow

```bash
# Run all quality gates: fmt, build, test, lint, conventions, bindings-check, frontend-check
mise run check

# Individual tasks
mise run fmt             # Format Go code with gofumpt -extra -w .
mise run build           # Compile all Go packages
mise run test            # Run Go unit and contract tests
mise run lint            # Run golangci-lint
mise run conventions     # Check agent-file rules and <= 5% Go comment budget
mise run bindings-check  # Verify frontend/src/mygo.ts matches MYGO_GENERATE output
mise run frontend-check  # Run tsc, bun test (Happy-DOM), and vite build
```

When modifying any method signature or DTO on `internal/gui` services (`Auth`, `Transfers`, `Backups`, `Downloads`, `Settings`), regenerate the TypeScript bindings and update the frontend adapters:

```bash
MYGO_GENERATE=frontend/src/mygo.ts go run .
```

Then update `frontend/src/mygo-backend.ts` and `frontend/src/testing/memory-backend.ts` to match.

## Project Structure

```text
main.go                         Thin MyGo v0.4.8 desktop shell & entrypoint
tray_icon.go                    Embedded 32x32 PNG tray icon
mygo.config.ts                  MyGo CLI configuration
internal/
  credstore/                    OS-protected credential vault (0600 / icacls) & cookie parser
  store/                        SQLite persistence (settings, backup_rules, tracked_files, transfers)
  engine/                       Upload planner, transfer queue, one-way backup scanner, reconciliation
  gui/                          Framework-neutral GUI facade, DTOs, error sanitizer, tray/quit state
frontend/
  src/
    backend.ts                  Frontend Backend interface & BackendError contract
    mygo-backend.ts             Production MyGo IPC adapter
    mygo.ts                     Generated MyGo TypeScript bindings (do not hand-edit)
    testing/memory-backend.ts   In-memory Backend implementation for Happy-DOM tests
    views/                      TransfersView, BackupsView, DownloadsView, SettingsModal
docs/
  adr/                          Architecture Decision Records
  agents/                       Agent workflow, issue tracker, and domain conventions
```

## Engineering Rules

1. **Strict Product Scope**: Rainforest is a desktop upload, one-way folder backup, transfer queue, and lightweight recovery download client. Do not add web-gallery pages (no cloud photo wall, timeline browser, AI search, album/folder CRUD manager, or trash UI).
2. **Framework Isolation**: `internal/credstore`, `internal/store`, `internal/engine`, and `internal/gui` must **never** import `github.com/egoist/mygo`. MyGo imports belong only in the root desktop shell (`main.go`).
3. **One-Way Backup Safety**: Local file deletion, move, rename, or offline external drives (`source_unavailable`) must never trigger remote cloud deletion (`MoveToTrash`). Uncertain uploads must be marked `unconfirmed` and reconciled before retry.
4. **Test-First & Offline Determinism**: Write Go tests against injected `http.RoundTripper` mocks and SQLite temp DBs, and frontend tests against `MemoryBackend`. Never require live Amazon credentials in CI or commit real session cookies.
5. **Comment Budget & Single Agent File**: Keep code self-explanatory (`<= 5%` comment lines in non-test Go files). `AGENTS.md` is the only allowed agent instruction file.
6. **Secret Redaction**: All user-visible error messages and diagnostic outputs must pass through `aphotos.Redact`.

## Pull Requests

1. Fork the repository and create a topic branch.
2. Add or update unit/contract tests in Go (`internal/...`, `main_test.go`) and TypeScript (`frontend/src/**/*.test.tsx`).
3. Ensure `mise run check` passes with zero warnings and `git diff --exit-code` is clean.
4. Open a pull request referencing the relevant issue.

## License

By contributing to Rainforest, you agree that your contributions will be licensed under the **GNU Affero General Public License v3.0 only (`AGPL-3.0-only`)** as set forth in [LICENSE](LICENSE).
