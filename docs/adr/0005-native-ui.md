# 0005: Native MyGo UI

## Status

Accepted. Supersedes the frontend and generated-bindings parts of
[ADR 0002](0002-mygo-desktop-shell.md) and the React parts of
[ADR 0004](0004-web-sign-in-and-native-i18n.md).

## Context

The prototype hosted a React/TypeScript frontend in the system WebView and
reached `internal/gui` through generated TypeScript bindings and a `Backend`
seam. That required a bun/Node toolchain, a bindings drift gate, and a second
copy of every DTO and string catalog. MyGo 0.5.1 ships a native widget toolkit
(`github.com/egoist/mygo/ui`) with a headless tester, so the interface can be
plain Go.

## Decision

1. The UI is a Go package, `internal/view`, built on `github.com/egoist/mygo/ui`.
   It calls the `internal/gui` services directly; blocking calls run off the UI
   thread and apply their results through `Update`.
2. `internal/view` never imports the root `github.com/egoist/mygo` package. The
   root `main` package remains the only place that does: window, tray, quit
   confirmation, notifications, and native file pickers.
3. `frontend/`, `mygo.BindAs`, generated TypeScript bindings, and the
   `EventFilesDropped` broadcast are removed. File drops are handled by the
   native view. The pages are Upload, Activity, Backup, Restore and Settings,
   plus a Welcome sign-in screen.
4. The MyGo CLI is pinned as a Go tool and invoked as `go tool mygo`. Icons are
   embedded Phosphor SVGs in `internal/view/icons`, pinned by the version and
   `weight name` lines in `icons.txt`; `mise run icons` syncs the SVGs from that
   manifest and a test fails when the manifest and files differ. `locales/*.json` remain the
   single string source and `locales/catalog_gen.go` is checked for drift by
   `mise run i18n-check`.

5. Backup is a split view: a compact list of folders on the left and, for the
   selected folder, its status, settings summary and a tree of tracked files on
   the right. Destinations are always shown as `Amazon Photos › Trips` paths
   chosen with a read-only folder picker; raw node IDs remain available in a
   collapsed field. Raw SDK errors are mapped to short sentences and keep the
   original text in a tooltip.

## Consequences

- No bun or Node toolchain; `mise run check` is Go-only.
- No TypeScript bindings to regenerate when a `gui` DTO changes.
- Views are tested headless with `ui.NewTester` against real `gui.Services`
  backed by temporary SQLite databases and stub HTTP transports.
- The Amazon sign-in window still uses the system WebView through MyGo's
  cookie APIs, as described in ADR 0004.
