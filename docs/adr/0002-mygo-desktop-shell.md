# ADR 0002: MyGo v0.4.8 Desktop Shell, Generated TypeScript Bindings, and Frontend Backend Seam

## Status

Accepted. Superseded in part by [ADR 0005](0005-native-ui.md): the frontend, generated TypeScript bindings, and `Backend` seam no longer exist.

## Context

Rainforest needs a lightweight, pure-Go (`CGO_ENABLED=0`) cross-platform desktop shell that hosts the React/TypeScript web frontend, exposes `internal/gui` services over typed IPC, integrates native file/directory dialogs, drag-and-drop, system tray status, and quit confirmation, and keeps the frontend 100% testable in Happy-DOM without launching a native WebView or opening SQLite.

Key constraints from `github.com/egoist/mygo v0.4.8` and the Rainforest spec include:
1. **Codegen execution model (`MYGO_GENERATE`)**: Running `mygo generate` (or `MYGO_GENERATE=frontend/src/mygo.ts go run .`) executes `main()` up to `mygo.App.Run()`, which writes the generated TypeScript bindings and exits before starting the OS event loop. Any bound services (`mygo.BindAs`) and package-level events (`mygo.NewEvent`) must be registered before `mygo.App.Run()`, while side effects (opening SQLite, loading the credential vault, starting background backup loops, creating windows/trays) must be deferred to `mygo.App.WhenReady`.
2. **Error serialization over MyGo IPC**: Unlike Wails, MyGo's IPC reply encoder serializes a returned `error` via `err.Error()` as the `CallError.message` string. `internal/gui.Error.Error()` therefore marshals a sanitized JSON envelope (`code`, `category`, `message`, `retryAfterMs`, `details`) that `frontend/src/mygo-backend.ts` parses into a structured `BackendError`.
3. **WebView isolation**: Because WebKitGTK has custom-scheme `<video>` limitations (MyGo #161) and Windows WebView2 has early cookie store race conditions (MyGo #343), Rainforest never uses the WebView cookie jar for Amazon authentication and never serves cloud media streams through custom schemes.

## Decision

1. **Thin Root Shell (`main.go`, `tray_icon.go`, `mygo.config.ts`)**:
   - Declare package-level `mygo.NewEvent` descriptors (`EventTransferStage`, `EventTransferProgress`, `EventBackupStatus`, `EventAuthChanged`, `EventFilesDropped`, `EventNotification`) at the root package so `mygo generate` discovers their payload types.
   - Construct `svc := gui.NewEmptyServices()` and register `Auth`, `Transfers`, `Backups`, `Downloads`, and `Settings` via `mygo.BindAs` before `mygo.App.Run()`.
   - Inside `mygo.App.WhenReady`, initialize `svc.Init(...)` under `mygo.App.Path(mygo.PathUserData)`, wire `svc.SetEmitter` to broadcast typed events and trigger `mygo.NewNotification`, wire `svc.SetFilePicker` to `mygo.Dialog.Open` (with `RAINFOREST_PICK_FILES` / `RAINFOREST_PICK_DIR` overrides for headless testing), create the main window and system tray (`gui.TrayTracker`), and guard window close / app quit with `gui.DecideQuit` and `mygo.Dialog.Message`.
2. **Frontend `Backend` Seam (`frontend/src/backend.ts`, `frontend/src/mygo-backend.ts`, `frontend/src/testing/memory-backend.ts`)**:
   - `frontend/src/backend.ts` re-exports the generated DTO types from `./mygo` and defines the `Backend` interface and `BackendError` contract.
   - `frontend/src/mygo-backend.ts` adapts the generated `./mygo` service calls and `mygo-runtime` (`isCallError`, `onFileDrop`) to `Backend`, decoding JSON `gui.Error` payloads in `toBackendError`.
   - `frontend/src/testing/memory-backend.ts` provides a deterministic in-memory `MemoryBackend` and `failingBackend` so React component and workflow tests run in milliseconds under `bun test` + Happy-DOM.

## Consequences

### Positive

- `MYGO_GENERATE=frontend/src/mygo.ts go run .` deterministically generates complete TypeScript bindings without touching disk state or display servers, enabling a CI drift gate.
- All UI components depend exclusively on the `Backend` TypeScript interface and are tested without a WebView.
- `internal/gui` remains 100% free of `github.com/egoist/mygo` imports.

### Negative

- Adding or changing a method on `internal/gui` requires regenerating `frontend/src/mygo.ts` and updating both `mygo-backend.ts` and `memory-backend.ts`.
