# Backup Scanning Rules and Transfer States

Reference for Rainforest's backup rule states, transfer job stages, and individual transfer item statuses.

## Backup Rule Statuses (`BackupStatus`)

| Status | Meaning |
|---|---|
| `active` | Rule is enabled and ready for scheduled or manual scans, and the last scan succeeded without errors. |
| `paused` | Rule is paused by the user (`paused = true`); scheduled scans skip this rule. |
| `scanning` | A directory walk or backup transfer for this rule is currently in progress. |
| `source_unavailable` | The rule's root `localPath` is missing or resides on an unmounted external volume (`os.Stat` failed). |
| `error` | The last scan or backup transfer encountered a filesystem walk error, offline client state, unconfirmed upload, or item upload failure (`lastError` contains the redacted diagnostic). |

## Transfer Job Stages (`TransferStage`)

| Stage | Terminal? | Meaning |
|---|---|---|
| `queued` | No | Job created and waiting for worker execution. |
| `running` | No | Workers are actively transferring items. |
| `paused` | No | Dispatch is paused by the user; in-flight workers wait before starting the next item. |
| `completed` | Yes | All items finished as `confirmed` or `skipped` with zero `failed` or `unconfirmed` items. |
| `partial` | Yes | One or more `album_pending` items, or at least one item succeeded (`confirmed` or `skipped`) and at least one item `failed` (with no `unconfirmed` items). |
| `unconfirmed` | Yes | One or more items finished in `unconfirmed` state and require reconciliation or manual retry. |
| `failed` | Yes | All non-cancelled items failed. |
| `cancelled` | Yes | Job was cancelled by the user before any item completed or failed. |

## Transfer Item Statuses (`ItemStatus`)

| Status | Meaning |
|---|---|
| `queued` | Waiting in the transfer queue. |
| `running` | Currently streaming upload or download bytes. |
| `confirmed` | Upload returned HTTP `200`/`201` with a verified remote `nodeId` (and stable local `size`/`mtime_ns`), or reconciled by matching remote MD5; album media additionally requires a successful membership response and retains its committed media ID even if the source changes; or download passed size and MD5 verification. |
| `skipped` | Upload `--on-conflict skip` verified an existing remote node with matching MD5, or download `--on-conflict skip` preserved an existing local file. |
| `album_pending` | Media Node ID saved; album membership failed or is pending. Retry associates only, without uploading media again. Counted in `itemsUnconfirmed` for queue totals. |
| `unconfirmed` | Commit state could not be proven (interrupted mid-flight, missing remote node ID, conflict candidate lacking MD5, or local file mutated during upload). |
| `failed` | Transfer failed with a definite error (`401`, `403`, `409` under `fail` policy, network exhaustion, or checksum mismatch). |
| `cancelled` | Cancelled by user action or global backup stop. |
