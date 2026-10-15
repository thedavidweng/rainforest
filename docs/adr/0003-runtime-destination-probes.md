# ADR 0003: Runtime Destination Probes and Album Checkpoints

## Status

Accepted

## Context

Regional protocol evidence cannot establish a current account's authorization to use a particular target. Albums are visual collections rather than storage parents. Uploading media and associating it with an album are separate writes; replaying the upload after an association failure can create duplicates.

## Decision

Use the same read-only destination probe in every supported marketplace. Preserve explicit IDs, require an exact AVAILABLE FOLDER or VISUAL_COLLECTION, and probe again before each execution batch. Keep protocol evidence separate from pending/readable/failed access status.

Upload album media to the account root, persist its Node ID and album_pending state before adding membership (atomically with the tracked-file checkpoint for backups), and retry membership using that ID. Bind transfer recovery and backup baselines to region, resolved account root, and target ID. Changing a backup target or source clears only its local baseline.

## Consequences

Readability never promises write permission. Partial album completion remains visible and recoverable without a second upload, including when a local file is gone. Switching accounts or regions requires a new job/rule or restoring the original identity. Failures never redirect to root or delete, move, or replace existing remote media.

See [runtime destination validation](../explanation/runtime-destination-validation.md) for the execution and recovery model.
