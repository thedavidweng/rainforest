# Upload Reconciliation, Deduplication, and Recovery

This page explains how Rainforest reconciles interrupted uploads, prevents duplicate POSTs, and surfaces recovery warnings.

## Why Content-Based Reconciliation Is Required

Amazon's `POST /cdproxy/nodes` upload endpoint is non-idempotent. If the desktop app is force-quit or loses network connectivity after transmitting file bytes but before receiving the HTTP `201 Created` response, the file may already exist in Amazon Photos while the local SQLite database still shows the item as `running` or `unconfirmed`.

Reconciling solely by filename and byte size is unsafe: two different photos (for example `IMG_0001.JPG` from two different cameras or edited versions of the same dimensions) can share a name or byte length while having different content.

## Strict MD5 Reconciliation (`findRemoteNodeForItem`)

Rainforest's `findRemoteNodeForItem` requires cryptographic content evidence (`MD5`) before promoting any `running` or `unconfirmed` item to `confirmed`:

1. It obtains the item's 32-character hex MD5 digest (from `TransferItem.MD5` or by hashing `TransferItem.LocalPath` when stable on disk). If no MD5 can be determined, reconciliation returns `nil` and keeps the item `unconfirmed`.
2. It queries `client.QueryNodes` for `AVAILABLE` `FILE` nodes matching `contentProperties.md5:<md5>` (and `parents:<destNodeId>` when a destination folder is specified).
3. Every candidate node is validated by `matchesCandidate`:
   - `cand.ContentProperties` must be non-nil (preventing nil-pointer panics when remote nodes omit metadata).
   - `cand.ContentProperties.MD5` must be non-empty and match the local MD5 case-insensitively.
   - If both local and remote sizes are positive, `cand.ContentProperties.Size` must equal `item.Size`.
   - If a destination parent folder ID is specified and `cand.Parents` is populated, the parent ID must match.
4. Only when a candidate passes all checks is the local item promoted to `confirmed` with `RemoteNodeID = cand.ID`. If a same-named remote node has a different MD5 or nil `ContentProperties`, the item stays `unconfirmed`.

## Preventing Automatic Re-Upload of `unconfirmed` Backup Files

When a backup upload finishes as `unconfirmed`:

- `processSingleBackupItem` records the file in `tracked_files` with `status = 'unconfirmed'` and keeps the `TransferItem` in `unconfirmed` state inside the `unconfirmed` backup transfer.
- On the next scheduled or manual `ScanNow`:
  1. `ScanNow` identifies any `tracked_files` entry in `unconfirmed` status as `reconcileOnly`.
  2. Before creating a new upload job, `ScanNow` calls `findRemoteNodeForItem` to check whether the previous upload landed on Amazon Photos.
  3. If found with matching MD5, `ScanNow` updates both `tracked_files` and the open `TransferItem` to `confirmed` without uploading again.
  4. If still unconfirmed and an open `unconfirmed` transfer item exists for that file, `ScanNow` **does not** re-POST the file automatically; it preserves the `unconfirmed` state and reports that the item is awaiting confirmation or manual retry.

## Startup Reconciliation Error Visibility

When `ReconcileOnStartup` runs during `Services.Init` or `reopen`:

- Even if remote queries fail (for example due to expired cookies or offline network), all interrupted `running` or `queued` items are deterministically transitioned to `unconfirmed` in SQLite so no transfer is ever falsely shown as `completed`.
- Any remote reconciliation error is recorded on `appState.lastReconcileErr`, emitted as a `Startup Recovery Warning` notification event, and included in `TransferListDTO.warning` so the React UI displays an immediate alert banner.
