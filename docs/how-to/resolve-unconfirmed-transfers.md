# Resolve Unconfirmed Transfers

This guide explains what the `unconfirmed` transfer state means and how to resolve it without creating duplicate uploads.

## When Does a Transfer Become `unconfirmed`?

Rainforest marks an upload or backup item `unconfirmed` when bytes may have been sent to Amazon Photos, but the commit outcome could not be verified:

- The app was quit, crashed, or lost network connectivity while an item was `running`.
- `POST /cdproxy/nodes` returned HTTP `200`/`201` without a valid remote `nodeId`.
- `--on-conflict skip` received HTTP `409 Conflict`, and the remote node lacked a verifiable `contentProperties.md5`.
- The local file was modified or truncated while the upload stream was in flight (`file modified while upload was in progress`).

## Automatic Reconciliation on Startup and Backup Scans

Rainforest never blindly re-POSTs an `unconfirmed` item during periodic background scans:

1. **On App Startup (`ReconcileOnStartup`)**: When Rainforest starts with valid credentials, it inspects all transfers needing reconciliation and queries Amazon Photos (`GET /nodes`) using the item's MD5 digest (and filename/size/parent).
   - If a matching remote node with identical MD5 is found, the item is promoted to `confirmed`.
   - If no match can be proven, the item remains `unconfirmed` so you can inspect it.
   - If startup reconciliation encounters an API or authentication error, Rainforest displays a **Startup Recovery Warning** banner while preserving all local state.
2. **During Scheduled Backup Scans**: If a tracked file is `unconfirmed`, `ScanNow` first attempts safe MD5 reconciliation against the remote folder. If the file is still unconfirmed in an open transfer, periodic scans leave it in `unconfirmed` state rather than re-uploading duplicates.

## Manually Reconcile and Retry

To resolve an `unconfirmed` transfer in **Transfers / Upload**:

1. Expand the transfer details to see which item is `unconfirmed` and read its status message.
2. Click **Retry** on the transfer card.
3. `Retry` first checks Amazon Photos by MD5:
   - If Amazon Photos has finished indexing the uploaded file, Rainforest marks it `confirmed` without uploading again.
   - If the file is confirmed absent on the server (or was modified locally mid-flight), Rainforest uploads the current stable local file and records the new `confirmed` node ID.
