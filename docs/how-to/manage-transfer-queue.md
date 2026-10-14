# Pause, Resume, and Retry Transfers

This guide shows how to control active transfers and retry failed or interrupted jobs in Rainforest.

## Pause and Resume an Active Transfer

In **Transfers / Upload**, every running upload or download card provides queue controls:

- **Pause**: Suspends worker dispatch before the next file begins streaming and transitions the transfer to `paused`.
- **Resume**: Wakes paused workers so remaining `queued` items continue without restarting already `confirmed` items.

## Cancel a Transfer

Click **Cancel** on an active or paused transfer:

- Rainforest cancels the in-flight HTTP context immediately.
- Already `confirmed` items remain intact in your Amazon Photos library.
- Remaining `queued` or `running` items are marked `cancelled`, and any temporary `.part` download files are removed.

## Retry Failed or Cancelled Transfers Safely

Click **Retry** on a transfer in `failed`, `partial`, `cancelled`, or `unconfirmed` stage:

1. Only items in `failed`, `cancelled`, `unconfirmed`, or `queued` state are re-queued; items already marked `confirmed` or `skipped` are never re-uploaded.
2. For upload and backup jobs, `Retry` enables pre-POST remote verification (`checkRemoteBeforePost = true`): before sending an HTTP `POST`, Rainforest computes the local file's MD5 and queries Amazon Photos (`GET /nodes`) to check whether the file already landed during the previous attempt.
3. If a remote node with the same MD5 (and matching size/parent) is found, Rainforest promotes the item directly to `confirmed` without uploading duplicate bytes.

## Clear Finished Transfer History

Click **Clear History** in **Transfers / Upload** to remove terminal transfer records (`completed`, `partial`, `failed`, `cancelled`) from the history list while preserving active transfers, unconfirmed transfers awaiting reconciliation, and `tracked_files` backup baselines.
