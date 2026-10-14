# MyGo, React, and Go SDK Architecture

This page explains Rainforest's layered architecture and how the desktop shell, GUI facade, transfer engine, SQLite store, and `amazon-photos` Go SDK interact.

## Five-Layer Separation

```text
React 19 + TypeScript UI (frontend/src/)
  └── Backend seam (frontend/src/backend.ts)
        ├── MyGo IPC adapter (frontend/src/mygo-backend.ts)
        └── In-memory test double (frontend/src/testing/memory-backend.ts)
              │
Thin MyGo v0.4.8 Desktop Adapter (main.go, tray_icon.go)
  └── Binds internal/gui services & desktop events
        │  (zero github.com/egoist/mygo imports below this line)
Framework-Neutral GUI Facade (internal/gui/)
  └── Auth, Transfers, Backups, Downloads, Settings, TrayTracker, DecideQuit
        │
Core Engine & Credential Vault (internal/engine/, internal/credstore/)
  └── Planner, Queue, One-Way Backup Scanner, ReconcileOnStartup
        │
SQLite State Store (internal/store/) + Amazon Photos Go SDK (pkg/aphotos)
```

## Why MyGo Lives Only in `main.go`

By restricting `github.com/egoist/mygo` imports to `main.go`:

- `internal/gui`, `internal/engine`, `internal/store`, and `internal/credstore` compile and run in standard `go test -race ./...` suites on headless Linux, macOS, and Windows runners without initializing a native window or system tray.
- `TestGUIPackageZeroMyGoOrWebviewImports` statically parses `internal/gui/*.go` to enforce that no WebView or desktop framework dependency leaks into the service layer.

## Deterministic TypeScript Binding Generation (`MYGO_GENERATE`)

MyGo generates `frontend/src/mygo.ts` by running the Go binary with `MYGO_GENERATE=<output-file>`. In `main.go`:

1. Package-level `mygo.NewEvent` descriptors and `svc := gui.NewEmptyServices()` are registered via `mygo.BindAs` before `mygo.App.Run()`.
2. When `MYGO_GENERATE` is set, `mygo.App.Run()` writes the TypeScript client definitions and exits immediately—before `mygo.App.WhenReady` opens SQLite or touches user directories.
3. `mise run bindings-check` verifies on every CI run that `frontend/src/mygo.ts` has zero drift from the Go service definitions.

## Avoiding WebView Cookie and Custom-Scheme Pitfalls

- **WebView Cookie Isolation ([MyGo #343](https://github.com/egoist/mygo/issues/343))**: Windows WebView2 initializes its cookie manager asynchronously, which can lose or race cookies read at startup. Rainforest never stores or reads Amazon cookies in the WebView; all HTTP traffic runs in Go via `pkg/aphotos`.
- **No Custom-Scheme Video Streaming ([MyGo #161](https://github.com/egoist/mygo/issues/161))**: Linux WebKitGTK does not reliably seek or stream `<video>` elements over custom WebView URI schemes. Rainforest only generates small local `data:image/png;base64` thumbnails during preflight planning.
