# Rainforest Domain Glossary

See `CONTEXT.md` for full domain context and architectural pointers.

## Terms

- **Rainforest** — Unofficial cross-platform desktop client for Amazon Photos focused exclusively on one-time uploads, one-way folder backups, transfer queue tracking, and recovery downloads.
- **CredentialStore** — OS-protected local vault (`internal/credstore`) persisting region, canonical Amazon session cookies (`session-id`, `ubid-*`, `at-*`), and optional host overrides.
- **Store** — Local SQLite persistence layer (`internal/store`) managing `settings`, `backup_rules`, `tracked_files`, `transfers`, and `transfer_items` without indexing remote cloud libraries.
- **UploadPlan** — Pre-execution summary of dropped or selected local paths (photo/video counts, total bytes, skipped unsupported files, destination capability, and local-only preview metadata).
- **Transfer** — A batch `upload`, `backup`, or `download` job progressing through `queued`, `running`, `paused`, `completed`, `partial`, `failed`, `cancelled`, or `unconfirmed` stages.
- **TransferItem** — A single media file within a `Transfer` with status `queued`, `running`, `confirmed`, `unconfirmed`, `skipped`, `failed`, or `cancelled`.
- **BackupRule** — A one-way automatic backup configuration for a local folder (`include_videos`, `schedule_mode`, `interval_minutes`, `conflict_policy`, `destination_node_id`, `status`).
- **TrackedFile** — Local file baseline (`rel_path`, `size`, `mtime_ns`, `md5`, `remote_node_id`, `status`) used to detect new or modified files during incremental backup scans.
- **SourceUnavailable** — Backup rule state (`source_unavailable`) entered when the local source directory is missing or on a disconnected volume.
- **ReconcileOnStartup** — Startup pass that checks `running` or `unconfirmed` items against remote MD5/name metadata to promote completed uploads to `confirmed` or mark interrupted ones for safe retry.
- **RecoverySource** — A confirmed local transfer-history item or remote folder/album entry selectable for verified recovery download.
