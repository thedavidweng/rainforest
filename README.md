# Rainforest

<p align="center">
  <a href="https://github.com/thedavidweng/rainforest/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/thedavidweng/rainforest/ci.yml?branch=main&style=flat-square&label=ci" alt="CI"></a>
  <a href="https://github.com/thedavidweng/rainforest/releases"><img src="https://img.shields.io/github/v/release/thedavidweng/rainforest?style=flat-square" alt="Release"></a>
  <a href="https://github.com/thedavidweng/rainforest/blob/main/LICENSE"><img src="https://img.shields.io/github/license/thedavidweng/rainforest?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/go-%3E%3D1.27-blue?style=flat-square" alt="Go">
</p>

> **Disclaimer**: Unofficial desktop client; Not affiliated with, endorsed by, or sponsored by Amazon.com, Inc.

**Rainforest** is an independent, cross-platform desktop tool for **one-time uploads, one-way automatic folder backup, transfer queue monitoring, and lightweight recovery downloads** for Amazon Photos—**NOT** a replica of the Amazon Photos web gallery.

Built on [`github.com/thedavidweng/amazon-photos`](https://github.com/thedavidweng/amazon-photos) and [`github.com/egoist/mygo`](https://github.com/egoist/mygo) v0.4.8 (`CGO_ENABLED=0`), Rainforest replaces the complexity of the official desktop uploader with a fast, transparent, verifiable transfer and backup workflow for macOS, Linux, and Windows.

---

## Product Scope: Desktop Backup vs. Web Gallery

Rainforest intentionally models the **desktop transfer and folder backup workflow** rather than duplicating the Amazon Photos web interface. For full cloud search, album/folder CRUD, or trash management from scripts or servers, use the companion [`aphotos` CLI and Go SDK](https://github.com/thedavidweng/amazon-photos).

| Area | Included in Rainforest Desktop | Intentionally Excluded (Web Gallery / CLI Scope) |
|---|---|---|
| **Account & Status** | Import browser cookies (`cookies.txt`, JSON array/map, raw `Cookie` header), explicit region selection, session verification, storage usage breakdown (`Photo`, `Video`, `Other`), and credential expiration alerts. | Unverified automated password login or OAuth token refresh flows. |
| **One-Time Upload** | Drag-and-drop or native file/folder picker; preflight `UploadPlan` showing photo/video counts, total bytes, skipped unsupported files, local-only thumbnail/metadata previews, conflict policy (`skip`, `rename`, `fail`), and destination readiness before starting. | Cloud photo wall, infinite timeline scrolling, or remote media streaming preview. |
| **Transfer Queue** | Real-time aggregate and per-item progress (`queued`, `running`, `paused`, `confirmed`, `unconfirmed`, `skipped`, `failed`, `cancelled`), pause/resume/cancel/retry controls, persistent SQLite transfer history, and startup reconciliation (`ReconcileOnStartup`). | — |
| **One-Way Folder Backup** | Add, edit, pause, resume, scan now, or remove local backup folders; filter by **Photos only** or **Photos + Videos**; **Immediate** or **Interval** scheduling; safe handling of disconnected external drives (`source_unavailable`). | Destructive two-way sync, mirroring local deletions to the cloud, or background daemon execution after the app exits. |
| **Lightweight Recovery Download** | Download original files by explicit remote node ID, confirmed transfer-history items, or listed remote folders/albums into a local directory with byte-size, MD5, and path-traversal verification. | Full cloud file manager, album/folder CRUD editor, sharing/Family Vault, Memories, or trash purge UI. |
| **Tray & Diagnostics** | System tray status summary, close-to-tray background operation, quit confirmation when transfers are active, concurrency controls, and graded `doctor` diagnostics. | Custom-protocol cloud video player or remote gallery indexer. |

---

## One-Way Backup Safety Guarantees

Rainforest is designed so that local filesystem accidents or transient network interruptions cannot corrupt or delete your cloud library:

1. **Strictly One-Way Incremental Backup**: Deleting, moving, or renaming a local file—or removing a backup rule in Rainforest—**never** calls `MoveToTrash` or deletes remote cloud copies.
2. **Offline External Drive Protection (`source_unavailable`)**: Before scanning a backup folder, Rainforest verifies that the root directory is reachable via `os.Stat`. If an external volume is unmounted or missing, the rule enters `source_unavailable` rather than treating the folder as an empty directory.
3. **Mid-Write File Stability Verification**: Before and after streaming a local file to `cdproxy/nodes`, the engine checks that the file's `size` and high-resolution `mtime_ns` remain unchanged. Files still being written or exported by another application are deferred to the next scan and never marked `confirmed`.
4. **Startup Reconciliation (`ReconcileOnStartup`)**: An upload is marked `confirmed` only when Amazon Photos returns HTTP `200`/`201` with a verified remote node ID. If Rainforest is closed or loses connectivity mid-upload, in-flight items are recorded as `unconfirmed`. On startup (and before any manual retry), Rainforest queries remote node metadata by MD5 and filename to promote already-landed files to `confirmed` instead of blindly re-POSTing duplicates.

---

## Credential & Privacy Model

- **OS-Protected Vault (`internal/credstore`)**: Imported session cookies (`session-id`, `ubid-*`, `at-*`), explicit region, and optional diagnostic host overrides are persisted in an application-specific vault file with `0600` permissions inside a `0700` directory on Unix (macOS/Linux) and restricted per-user ACLs (`icacls`) on Windows.
- **Canonical Wire Cookie Normalization**: Input cookies accept both underscore (`session_id`, `ubid_main`, `at_main`, `ubid_acb*`, `at_acb*`) and hyphen keys, emitting only canonical hyphenated wire cookies alongside `x-amzn-sessionid`.
- **Fail-Fast Region Resolution**: If a region is not explicitly chosen and cannot be unambiguously inferred from regional cookie suffixes, Rainforest rejects the import and prompts for an explicit region—never silently falling back to `.com`.
- **Strict Secret Redaction (`aphotos.Redact`)**: Every SDK error, IPC `BackendError` payload, and diagnostic check passes through `aphotos.Redact` so session cookies, `x-amzn-sessionid`, tokens, and pre-signed `contentRedirection` URLs never appear in UI messages or logs.
- **Zero Reliance on WebView Cookie Jar ([MyGo #343](https://github.com/egoist/mygo/issues/343))**: All Amazon Photos HTTPS requests are executed directly in Go via `github.com/thedavidweng/amazon-photos/pkg/aphotos`. Rainforest never reads or stores Amazon credentials in the system WebView cookie jar, avoiding Windows WebView2 startup cookie race conditions.
- **No Custom-Scheme Video Player Needed ([MyGo #161](https://github.com/egoist/mygo/issues/161))**: Because Rainforest only displays lightweight local preflight thumbnails and never streams remote cloud videos through a custom WebView protocol, it avoids Linux WebKitGTK custom-scheme `<video>` playback limitations.

---

## Regional Routing & Verification Status

Rainforest pairs each marketplace region with its corresponding `drive/v1` API domain, regional `cdproxy/nodes` upload host, thumbnail host, and canonical session cookies:

| Region | Drive API (`DriveURL`) | Upload Host (`UploadURL`) | Canonical Wire Cookies | Verification Status |
|---|---|---|---|---|
| **US** (`us`) | `https://www.amazon.com/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-main`, `at-main` | `verified` |
| **CA** (`ca`) | `https://www.amazon.ca/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbca`, `at-acbca` | `experimental` |
| **UK** (`uk`) | `https://www.amazon.co.uk/drive/v1` | `https://content-eu.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbuk`, `at-acbuk` | `experimental` |
| **JP** (`jp`) | `https://www.amazon.co.jp/drive/v1` | `https://content-jp.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbjp`, `at-acbjp` | `experimental` |
| **EU** (`de`, `fr`, `it`, `es`) | `https://www.amazon.{tld}/drive/v1` | `https://content-eu.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acb{tld}`, `at-acb{tld}` | `experimental` |

Regions marked `experimental` have complete URL and cookie routing contracts tested against mock transports, and the UI surfaces an explicit `experimental` badge until verified against live regional accounts.

---

## Architecture Overview

```text
+-----------------------------------------------------------------------+
| React 19 + TypeScript + Tailwind v4 Frontend (frontend/src/)          |
|   TransfersView | BackupsView | DownloadsView | SettingsModal         |
+-----------------------------------+-----------------------------------+
                                    | Frontend Backend Seam (frontend/src/backend.ts)
                   +----------------+----------------+
                   |                                 |
     Production IPC Adapter             Deterministic Test Double
 (frontend/src/mygo-backend.ts)    (frontend/src/testing/memory-backend.ts)
                   |
                   v  Generated TypeScript Bindings (frontend/src/mygo.ts)
+-----------------------------------------------------------------------+
| Thin MyGo v0.4.8 Desktop Shell (main.go, tray_icon.go)                |
|   Window lifecycle, native file/folder dialogs, drag-and-drop, tray   |
+-----------------------------------+-----------------------------------+
                                    | Zero MyGo imports below this line
                                    v
+-----------------------------------------------------------------------+
| Framework-Neutral GUI Facade (internal/gui/)                          |
|   Auth | Transfers | Backups | Downloads | Settings | TrayTracker     |
+-----------------+-----------------------------------+-----------------+
                  |                                   |
                  v                                   v
+-----------------------------------+ +---------------------------------+
| Transfer & Backup Engine          | | Credential Vault                |
| (internal/engine/)                | | (internal/credstore/)           |
|   UploadPlan, Queue, One-Way      | |   0600 / icacls permissions,    |
|   Scanner, ReconcileOnStartup     | |   cookie normalization & redact |
+-----------------+-----------------+ +---------------+-----------------+
                  |                                   |
                  v                                   |
+-----------------------------------+                 |
| SQLite Operational Store          |                 |
| (internal/store/, modernc/sqlite) |                 |
+-----------------+-----------------+                 |
                  |                                   |
                  +-----------------+-----------------+
                                    |
                                    v
+-----------------------------------------------------------------------+
| Amazon Photos Go SDK (github.com/thedavidweng/amazon-photos)          |
|   pkg/aphotos — streaming cdproxy upload, contentRedirection download |
+-----------------------------------------------------------------------+
```

See [CONTEXT.md](CONTEXT.md), [GLOSSARY.md](GLOSSARY.md), [ADR 0001](docs/adr/0001-rainforest-architecture.md), and [ADR 0002](docs/adr/0002-mygo-desktop-shell.md) for detailed design decisions.

---

## Development, Testing & Build Instructions

### Prerequisites

- [mise](https://mise.jdx.dev/) (installs Go `1.27.2`, Bun `1.4.2`, `gofumpt`, and `golangci-lint` from `mise.toml`), or:
  - Go `1.27+` (`CGO_ENABLED=0` supported across all platforms)
  - Bun `1.4+`

### Install Dependencies & Run Quality Gates

```bash
# Install toolchains and frontend packages
mise install
bun install --cwd frontend

# Run all formatting, Go build/test/lint, comment budget, bindings drift, and frontend checks
mise run check
```

Individual tasks defined in `mise.toml`:

- `mise run fmt` — format Go sources with `gofumpt -extra -w .`
- `mise run build` — compile Go packages (`go build ./...`)
- `mise run test` — run Go unit and integration contract tests (`go test ./...`)
- `mise run lint` — run `golangci-lint run`
- `mise run conventions` — verify agent-file rules and the `<= 5%` Go comment budget
- `mise run bindings-check` — verify `frontend/src/mygo.ts` matches `MYGO_GENERATE` output
- `mise run frontend-check` — run TypeScript typecheck, Happy-DOM component tests (`bun --cwd frontend test`), and Vite production bundle build (`frontend/dist`)

### Live Development Mode

```bash
bunx mygo dev
```

### Production Desktop Build

Build the embedded production web bundle and package the native desktop application via `mygo build`:

```bash
# Build frontend assets into frontend/dist and package for the host platform
bun run --cwd frontend build
bunx mygo build -skip-dmg -skip-notarize

# Or build a standalone Go binary directly
go build -o dist/rainforest .
./dist/rainforest --version
```

Cross-platform targets can be built with `-platform` (for example `bunx mygo build -platform darwin/arm64,darwin/amd64,linux/amd64,windows/amd64 -skip-dmg -skip-notarize`).

---

## Acknowledgments & Credits

- **[`thedavidweng/amazon-photos`](https://github.com/thedavidweng/amazon-photos)** (MIT) — Go SDK (`pkg/aphotos`) and CLI providing the underlying Amazon Photos `/drive/v1` and `cdproxy/nodes` client, regional routing, retry engine, and redaction primitives.
- **[`thedavidweng/tg-drive`](https://github.com/thedavidweng/tg-drive)** (AGPL-3.0-only) — Architectural inspiration for the framework-neutral `internal/gui` service facade, `FilePicker` / `Emitter` / `TrayTracker` / `DecideQuit` ports, and the TypeScript `Backend` / `MemoryBackend` testing seam.
- **[`egoist/mygo`](https://github.com/egoist/mygo)** v0.4.8 (MIT) — Pure-Go cross-platform desktop application framework, WebView bridge, and TypeScript binding generator.
- **Upstream Amazon Photos Protocol Research**:
  - [`trevorhobenshield/amazon_photos`](https://github.com/trevorhobenshield/amazon_photos) (MIT) by Trevor Hobenshield and community PR contributors ([#18](https://github.com/trevorhobenshield/amazon_photos/pull/18), [#19](https://github.com/trevorhobenshield/amazon_photos/pull/19), [#20](https://github.com/trevorhobenshield/amazon_photos/pull/20), [#27](https://github.com/trevorhobenshield/amazon_photos/pull/27), [#28](https://github.com/trevorhobenshield/amazon_photos/pull/28), [#29](https://github.com/trevorhobenshield/amazon_photos/pull/29), [#30](https://github.com/trevorhobenshield/amazon_photos/pull/30), [#31](https://github.com/trevorhobenshield/amazon_photos/pull/31), [#33](https://github.com/trevorhobenshield/amazon_photos/pull/33), [#34](https://github.com/trevorhobenshield/amazon_photos/pull/34), [#35](https://github.com/trevorhobenshield/amazon_photos/pull/35)).
  - [`kpfitzgerald/amazon-photos-mcp`](https://github.com/kpfitzgerald/amazon-photos-mcp) — US account endpoint verification notes and cookie normalization insights.
  - [`vanessa/immich-amazon-sync`](https://github.com/vanessa/immich-amazon-sync) (MIT) — Upload conflict and trash protocol observations.

---

## License

Copyright (C) 2026 Davy Weng and contributors.

Rainforest is licensed under the **GNU Affero General Public License v3.0 only (`AGPL-3.0-only`)** — see [LICENSE](LICENSE) for the full text. The underlying Go SDK ([`github.com/thedavidweng/amazon-photos`](https://github.com/thedavidweng/amazon-photos)) is separately licensed under the MIT License.
