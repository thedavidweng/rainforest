# MyGo Native UI and Go SDK Architecture

This page explains Rainforest's layered architecture and how the desktop shell, native views, GUI facade, transfer engine, SQLite store, and `amazon-photos` Go SDK interact.

## Layers

```text
Thin MyGo Desktop Adapter (main.go, tray_icon.go)
  └── Window, tray, quit confirmation, notifications, native file pickers
        │  (the only package importing github.com/egoist/mygo)
Native Views (internal/view, mygo/ui)
  └── Welcome, Upload, Activity, Backup, Restore, Settings
        │  (imports only github.com/egoist/mygo/ui)
Framework-Neutral GUI Facade (internal/gui/)
  └── Auth, Transfers, Backups, Downloads, Settings, TrayTracker, DecideQuit
        │  (zero MyGo imports from here down)
Core Engine & Credential Vault (internal/engine/, internal/credstore/)
  └── Planner, Queue, One-Way Backup Scanner, ReconcileOnStartup
        │
SQLite State Store (internal/store/) + Amazon Photos Go SDK (pkg/aphotos)
```

## Why the Root MyGo Package Lives Only in `main.go`

By restricting the root `github.com/egoist/mygo` package to `main.go`:

- `internal/gui`, `internal/engine`, `internal/store`, and `internal/credstore` compile and run in standard `go test -race ./...` suites on headless Linux, macOS, and Windows runners without initializing a native window or system tray.
- `TestGUIPackageZeroMyGoOrWebviewImports` statically parses `internal/gui/*.go` to enforce that no desktop framework dependency leaks into the service layer.
- `internal/view` depends only on `mygo/ui`, so its pages run headless under `ui.NewTester`.

## How Views Talk to Services

Views call `internal/gui` services directly; there is no IPC layer or generated client. `gui` calls block on the network or SQLite, so a view never calls them while building a frame: work runs on a goroutine and its result is applied on the UI thread through the `Update` function that `main.go` passes in. `main.go` forwards `gui` events (transfer progress, backup status, auth changes, notifications) to the view, which applies them the same way. `gui.Error` values are shown by their sanitized `Message`.

File drops are handled by the native view: dropping files on any signed-in page switches to Upload and opens the review dialog.

## Tooling

The MyGo CLI is a pinned Go tool (`go tool mygo`). `go tool mygo generate` regenerates the typed i18n accessors in `locales/catalog_gen.go`, and `mise run i18n-check` fails CI when they drift. Tests exercise views with `ui.NewTester` and real `gui.Services`.

## Avoiding WebView Cookie and Custom-Scheme Pitfalls

- **WebView Cookie Isolation ([MyGo #343](https://github.com/egoist/mygo/issues/343))**: Windows WebView2 initializes its cookie manager asynchronously, which can lose or race cookies read at startup. The native sign-in window is the only WebView use; Rainforest never reads Amazon cookies from it after saving them to the vault, and all HTTP traffic runs in Go via `pkg/aphotos`.
- **No Custom-Scheme Video Streaming ([MyGo #161](https://github.com/egoist/mygo/issues/161))**: Rainforest does not stream remote media. It only generates small local thumbnails during preflight planning, decoded natively in the review dialog.
