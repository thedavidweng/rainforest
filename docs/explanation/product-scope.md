# Rainforest Product Scope and Boundaries

This page explains why Rainforest focuses strictly on desktop transfers and one-way folder backup rather than duplicating the Amazon Photos web application.

## Desktop Transfer Client vs. Web Gallery

The Amazon Photos web interface (`amazon.<tld>/photos`) is designed for browsing timelines, viewing AI-recognized people/places/things, sharing Family Vault albums, and managing cloud trash. Replicating a full cloud gallery inside a desktop app introduces large local metadata caches, background indexing spikes, custom WebView media streaming issues ([MyGo #161](https://github.com/egoist/mygo/issues/161)), and high risk of destructive synchronization bugs.

Rainforest instead solves the core desktop problem: **getting local photos and videos safely into Amazon Photos, keeping local folders backed up one-way, monitoring transfer progress, and recovering original files when needed.**

## What Rainforest Includes

1. **Account & Diagnostics (`SettingsView`)**: Cookie import (`cookies.txt`, JSON array/map, raw `Cookie` header), explicit region selection, storage usage summary, concurrency settings, and 5-grade `doctor` diagnostics.
2. **One-Time Preflight & Upload (`TransfersView`)**: Drag-and-drop or native file/folder selection with a preflight `UploadPlan` (photo/video counts, byte totals, skipped non-media count, local-only image thumbnails, and conflict policy selection).
3. **Transfer Queue (`TransfersView`)**: Real-time progress, pause/resume/cancel/retry controls, persistent SQLite history, and startup reconciliation (`ReconcileOnStartup`).
4. **One-Way Automatic Folder Backup (`BackupsView`)**: Scheduled (`immediate` or `interval`) local folder scanning with `Photos only` or `Photos + Videos` filtering and `source_unavailable` protection for removable drives.
5. **Lightweight Recovery Download (`DownloadsView`)**: Verified download of confirmed history items, listed folders/albums, or explicit remote node IDs into a local directory.

## What Rainforest Intentionally Excludes

- No cloud photo wall or infinite timeline browser.
- No AI people/places/things search UI.
- No album/folder CRUD manager or cloud file organizer.
- No sharing, Family Vault, or Memories UI.
- No remote trash viewer or permanent purge UI.
- No destructive two-way sync.

For scripting, search queries, aggregations, album/folder mutations, or trash management, use the companion [`aphotos` CLI and Go SDK](https://github.com/thedavidweng/amazon-photos).
