# One-Way Backup and File Safety Design

This page explains the safety invariants that protect both your remote Amazon Photos library and your local files during automatic backups and recovery downloads.

## Strictly One-Way Backup

Two-way synchronization engines must decide whether a missing local file was intentionally deleted by the user or disappeared because a folder was moved, a filter was changed, or an external drive was unplugged. Guessing wrong deletes cloud backups.

Rainforest enforces a strict one-way invariant:

- Backup rules (`internal/engine/backup.go`) only call `GetRoot`, `QueryNodes`, and `UploadFile`.
- Neither `internal/engine` nor `internal/gui` ever invokes `MoveToTrash` or any remote deletion endpoint during uploads, backup scans, or rule removal.
- Deleting, moving, or renaming a local file has zero effect on files already confirmed in Amazon Photos.

## External Drive Disconnection Protection (`source_unavailable`)

Before walking a backup rule's directory, `ScanNow` runs `os.Stat(rule.LocalPath)`:

- If the directory is missing or not a directory (for example, an unmounted external drive or disconnected SMB/NFS share), the rule transitions to `source_unavailable` and records the error.
- Existing `tracked_files` rows for that rule are preserved intact so that when the drive is reconnected, Rainforest resumes incremental scanning without re-uploading confirmed files.

## Surface-All Filesystem Walk Errors

During `filepath.WalkDir`, if a subdirectory or file cannot be read due to permissions or I/O errors (other than a transient `os.ErrNotExist` on a vanishing temp file):

- `ScanNow` records the failure in `scanWarnings`, sets the rule status to `error` with `lastError` populated, and returns a non-nil error.
- An incomplete directory scan is never silently reported as a clean `active` scan, ensuring unreadable subdirectories are retried on subsequent scans once permissions are fixed.

## Mid-Upload File Mutation Guard

If another application is still exporting or modifying a photo/video while Rainforest is uploading it:

1. `processSingleBackupItem` (and `processSingleUploadItem`) records `sizeBefore` and `mtimeBefore` (`ModTime().UnixNano()`) before opening the upload stream.
2. After `UploadFile` returns, it re-stats the local file (`sizeAfter`, `mtimeAfter`).
3. If the file vanished or its size or modification timestamp changed while the HTTP `POST` was in flight, Rainforest marks both the `TransferItem` and the `TrackedFile` as **`unconfirmed`** (never `confirmed` and never silently `skipped`).
4. Subsequent scans reconcile the remote state safely before deciding whether the updated local file needs to be uploaded.

## Local File Protection on Download

Recovery downloads never overwrite existing local files:

- `processSingleDownloadItem` and `pkg/aphotos.Download` support `skip` (default in Rainforest), `rename`, and `fail` conflict policies.
- Temporary `.aphotos-dl-*.part` files are finalized via atomic link/exclusive-create (`os.Link` / `O_CREATE|O_EXCL`) only after byte size and MD5 verification succeed.
