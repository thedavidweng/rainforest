# ADR 0001: Rainforest Core Engine, SQLite State Store, One-Way Backup, and Startup Reconciliation

## Status

Accepted

## Context

Rainforest is a desktop client for Amazon Photos focused on four workflows: account credential management, one-time drag-and-drop uploads, one-way automatic folder backups, and lightweight recovery downloads. Unlike web gallery applications, Rainforest must operate reliably across process restarts, network interruptions, rate limits, and removable drives without ever performing destructive two-way sync or indexing the user's entire cloud library.

Key hazards identified from official desktop workflows and prior open-source tools include:
1. **Uncertain upload commit state**: If the process crashes or the network drops after streaming a file to `cdproxy/nodes`, blindly re-POSTing on restart creates duplicate remote files, while assuming success loses data.
2. **Destructive folder sync**: Treating a missing local file or an unmounted external drive as a deletion event risks wiping cloud backups.
3. **Mid-write file uploads**: Uploading a photo or video while another process is still writing or exporting it produces truncated or corrupted remote media.
4. **Framework lock-in**: Coupling SQLite state, transfer scheduling, and credential storage to WebView or MyGo lifecycle primitives prevents deterministic in-process testing.

## Decision

1. **Framework-Neutral Core Packages (`internal/credstore`, `internal/store`, `internal/engine`)**:
   All persistence, credential normalization, upload planning, transfer execution, backup scanning, and recovery download logic live in pure Go packages that depend on `github.com/thedavidweng/amazon-photos/pkg/aphotos` and `modernc.org/sqlite` (`CGO_ENABLED=0`), with zero `github.com/egoist/mygo` imports.
2. **OS-Protected Credential Store (`internal/credstore`)**:
   Credentials are stored in a dedicated vault file with `0600` permissions inside a `0700` directory on Unix and user-only `icacls` ACLs on Windows. Cookie imports accept Netscape `cookies.txt`, JSON arrays, JSON maps, and raw `Cookie` headers, normalizing underscore aliases to canonical hyphenated wire names (`session-id`, `ubid-*`, `at-*`) and requiring an explicit or unambiguously inferred region (never falling back to `.com`). All error and diagnostic paths scrub secrets via `aphotos.Redact`.
3. **Minimal Local SQLite State (`internal/store`)**:
   SQLite stores only local operational state: `settings`, `backup_rules`, `tracked_files`, `transfers`, and `transfer_items`. Rainforest never builds a full remote media catalog or runs full-library scans on startup.
4. **Startup Reconciliation (`ReconcileOnStartup`)**:
   On engine startup, any `running` or `unconfirmed` transfer items from a prior session are reconciled. When online and an item has a known `md5` or `name`, the engine queries `QueryNodes` to check whether the file landed on Amazon Photos; matching items are promoted to `confirmed` with their `remote_node_id`, while unverified interrupted items are marked `unconfirmed` so retries check remote state first instead of blindly re-POSTing.
5. **Strictly One-Way Incremental Backup & Mid-Write Protection**:
   Backup rules only upload new or changed local media (`photo` only or `photo` + `video`). Local file deletions, renames, moves, or rule removals never invoke `MoveToTrash` or any remote deletion API. If `os.Stat` on a rule's root path fails, the rule enters `source_unavailable` rather than being treated as an empty directory. Before and after streaming a candidate file, the engine verifies that `size` and `mtime_ns` remain stable, deferring files that are actively being modified.

## Consequences

### Positive

- Every core workflow (preflight upload planning, conflict resolution, pause/resume/cancel/retry, one-way backup scanning, mid-write detection, startup reconciliation, and verified recovery download) is deterministically testable via an injected `http.RoundTripper` and temporary SQLite database.
- Users' cloud backups are protected against accidental deletion when local files are removed or external drives disconnect.
- Interrupted transfers recover cleanly across restarts without silent data loss or blind duplicate uploads.

### Negative

- Local state tracks only files uploaded or backed up by Rainforest; pre-existing remote files are resolved on demand via SDK conflict checks or lightweight recovery queries.
