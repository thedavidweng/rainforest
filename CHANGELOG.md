# Changelog

All notable changes to **Rainforest** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Rebuilt the interface as a native desktop UI with a sidebar and a Welcome sign-in screen. Pages are now Upload, Activity, Backup, Restore, and Settings, with light and dark appearance that follows your system.
- Upload now has a drop zone, a review dialog with a thumbnail grid and clear destination status, and a lighter recent-activity list. Dropping files on any page opens the review.
- Activity groups transfers into In progress and Earlier, and explains unconfirmed uploads and downloads separately.
- Settings apply immediately; storage details, diagnostics, and account sign-out live in one page.

### Removed

- The WebView-based React frontend and its Node/bun tooling. Building from source now needs only Go (and `go tool mygo`).

## [0.1.0] - 2026-10-14

Initial release of **Rainforest**, the cross-platform desktop upload, one-way folder backup, transfer queue, and recovery download client for Amazon Photos.

### Added

#### Core Engine, Credential Vault & SQLite State Store (`internal/credstore`, `internal/store`, `internal/engine`)
- OS-protected credential store (`internal/credstore`) persisting browser session cookies (`session-id`, `ubid-*`, `at-*`), explicit marketplace region, and optional diagnostic host overrides with `0600` permissions on Unix and restricted user ACLs (`icacls`) on Windows.
- Multi-format cookie parser supporting Netscape `cookies.txt`, JSON cookie arrays, JSON key/value maps, and raw `Cookie` headers with underscore-to-hyphen normalization and fail-fast region validation.
- Local SQLite operational store (`internal/store` via `modernc.org/sqlite`, `CGO_ENABLED=0`) tracking `settings`, `backup_rules`, `tracked_files`, `transfers`, and `transfer_items` without indexing full remote cloud libraries.
- Preflight `UploadPlan` builder computing photo/video counts, total bytes, skipped unsupported files, local-only image thumbnail/dimension previews, and regional destination readiness (`verified` vs. `experimental`).
- Transfer queue execution supporting pause, resume, cancel, retry, per-item progress reporting, and conflict policies (`skip`, `rename`, `fail`).
- Startup reconciliation (`ReconcileOnStartup`) querying remote node MD5/filename metadata for interrupted `running` or `unconfirmed` transfers so landed uploads are promoted to `confirmed` without blind duplicate re-POSTs.
- Strictly one-way folder backup engine (`immediate` and `interval` schedules, `photos` only or `photos + videos`) with `source_unavailable` protection for unmounted external drives and pre/post-upload `size` + `mtime_ns` mid-write stability checks.
- Verified recovery download workflow (`ListRecoverySources`, `StartDownload`) supporting explicit remote node IDs, confirmed transfer-history items, and remote folder/album listings with atomic rename and size/MD5 verification.

#### GUI Service Facade & MyGo v0.4.8 Desktop Shell (`internal/gui`, `main.go`)
- Framework-neutral `internal/gui` service facade (`Auth`, `Transfers`, `Backups`, `Downloads`, `Settings`) with structured JSON error envelopes (`BackendError`), secret redaction (`aphotos.Redact`), `TrayTracker`, and `DecideQuit` quit-confirmation logic.
- MyGo v0.4.8 desktop shell (`main.go`, `tray_icon.go`, `mygo.config.ts`) binding services for `MYGO_GENERATE` TypeScript codegen, wiring native file/folder dialogs (`RAINFOREST_PICK_FILES` / `RAINFOREST_PICK_DIR` test overrides), drag-and-drop (`OnFileDrop`), system tray menu, close-to-tray behavior, and desktop notifications.

#### React + TypeScript Frontend (`frontend/`)
- Focused four-surface desktop UI (`Transfers / Upload`, `Backups`, `Downloads`, and `Settings & Diagnostics` modal) built with React 19, TypeScript, Vite, and Tailwind CSS v4—intentionally excluding web-gallery features (no cloud photo wall, AI search, album/folder manager, or trash UI).
- Frontend `Backend` seam (`frontend/src/backend.ts`), MyGo IPC adapter (`frontend/src/mygo-backend.ts`), and deterministic in-memory `MemoryBackend` (`frontend/src/testing/memory-backend.ts`) with Happy-DOM component and workflow tests.
- Configured valid MyGo Ed25519 update public key (`updates.publicKey`), ESLint Flat Config, Prettier formatting, `jactionlint` v2 workflow checks, and Diátaxis documentation under `docs/{tutorials,how-to,reference,explanation}`.
- Hardened `findRemoteNodeForItem` to require matching remote `contentProperties.md5`, `ScanNow` to reconcile `unconfirmed` tracked files without automatic re-POSTing and surface partial directory walk errors, `processSingleBackupItem` to mark post-POST local file mutations as `unconfirmed`, `gui.Services` to surface `ReconcileOnStartup` warnings, and `credstore.ResolveDataDir` to reject OS temporary directory fallbacks.
