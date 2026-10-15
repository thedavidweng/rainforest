# Rainforest Domain Context

## Core Creed

**Rainforest is a focused, cross-platform desktop backup and transfer client for Amazon Photos**, providing one-time upload, strictly one-way folder backup, a persistent transfer queue with startup reconciliation, and lightweight recovery download — without replicating the Amazon Photos web gallery.

## Core Concepts

- **Rainforest** — The AGPL-3.0-only cross-platform desktop application built on MyGo v0.4.8 and `github.com/thedavidweng/amazon-photos/pkg/aphotos`.
- **Credential Store (`internal/credstore`)** — OS-protected credential adapter (`0600` on Unix, restricted user ACL via `icacls` on Windows) storing canonical browser cookies (`session-id`, `ubid-*`, `at-*`), explicit region, and optional diagnostic host overrides. Never falls back to `.com` on ambiguous regions and scrubs secrets via `aphotos.Redact`.
- **State Store (`internal/store`)** — Local SQLite database (`modernc.org/sqlite`) persisting application `Settings`, `BackupRule` definitions, `TrackedFile` local inventory, `Transfer` jobs, and `TransferItem` records. Never builds a full remote cloud media index or triggers full-library scans on startup.
- **Upload Plan (`UploadPlan`)** — Preflight inspection result computed before starting a one-time upload: counts supported photos and videos, calculates total bytes, identifies skipped unsupported files, reports runtime destination readability independently of regional protocol evidence, and optionally generates local preview metadata/thumbnails for small local images.
- **Transfer & TransferItem** — A queued, running, paused, completed, partial, failed, cancelled, or unconfirmed upload/backup/download job (`Transfer`) composed of individual file operations (`TransferItem`).
- **Confirmed vs. Unconfirmed Item** — An upload item is `confirmed` only when the SDK returns a verified remote node ID. If a transfer is interrupted or returns an uncertain result, it is recorded as `unconfirmed` and reconciled on startup (`ReconcileOnStartup`) by querying remote MD5/name before any retry to prevent blind duplicate POSTs.
- **One-Way Backup (`BackupRule`)** — Strictly incremental local-to-cloud upload (`immediate` or `interval` schedule, `photos` only or `photos + videos`). Local file deletion, move, rename, or backup rule removal **never** calls `MoveToTrash` or deletes remote cloud media.
- **Source Unavailable (`source_unavailable`)** — Backup rule status set when the configured local folder does not exist or resides on an unmounted external drive (`os.Stat` failure), preventing the engine from misinterpreting an offline disk as an empty directory.
- **Mid-Write Stability Check** — Pre-upload and post-upload verification of a local file's `size` and `mtime_ns` so actively written or mutating files are skipped/deferred and never marked `confirmed`.
- **Recovery Download** — Secondary workflow (`ListRecoverySources`, `StartDownload`) that downloads explicitly selected remote node IDs, transfer-history items, or listed folder/album children to a local directory with verified size/MD5 and atomic rename.

## Architectural Sources of Truth

- Core engine, SQLite state store, one-way backup & reconciliation architecture: `docs/adr/0001-rainforest-architecture.md`
- MyGo v0.4.8 desktop shell, generated TypeScript bindings, and frontend `Backend` seam: `docs/adr/0002-mygo-desktop-shell.md`
- Domain terms: `GLOSSARY.md`
- Agent & repository rules: `AGENTS.md`
