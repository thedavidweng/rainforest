# Regional Compatibility and Runtime Destination Status

Rainforest uses `amazon-photos` v0.1.3 regional routing. Evidence is protocol provenance, never an upload permission switch.

| Region | Drive API | Upload host | Required cookies | Evidence |
|---|---|---|---|---|
| `us` | `https://www.amazon.com/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-main`, `at-main` | `verified` |
| `ca` | `https://www.amazon.ca/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbca`, `at-acbca` | `experimental` |
| `uk` | `https://www.amazon.co.uk/drive/v1` | `https://content-eu.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbuk`, `at-acbuk` | `experimental` |
| `jp` | `https://www.amazon.co.jp/drive/v1` | `https://content-jp.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbjp`, `at-acbjp` | `experimental` |
| EU marketplace code | `https://www.amazon.{code}/drive/v1` | `https://content-eu.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acb{code}`, `at-acb{code}` | `experimental` |

European codes accepted by the SDK: `at`, `be`, `bg`, `ch`, `cy`, `cz`, `de`, `dk`, `ee`, `es`, `fi`, `fr`, `gr`, `hr`, `hu`, `ie`, `is`, `it`, `li`, `lt`, `lu`, `lv`, `mt`, `nl`, `no`, `pl`, `pt`, `ro`, `se`, `si`, `sk`. Use a marketplace code, not a literal `eu`. SDK aliases include `com` → `us`, `gb` / `co.uk` → `uk`, and `co.jp` → `jp`. Australia and Turkey are not supported by this SDK version.

## Destination Rules (Every Supported Region)

Blank `destinationNodeId` selects the current account root. An explicit ID is preserved and read through the current client's `GetNode`. The returned ID must match exactly and status must be `AVAILABLE`.

| Node kind | Behavior |
|---|---|
| `FOLDER` | Upload with this node's ID as `parentNodeId`. |
| `VISUAL_COLLECTION` (album) | Upload media to the account's ordinary root folder, then call `AddAlbumMembers` with the confirmed media Node ID. |
| Any other kind, including `FILE` or literal `ALBUM` | Reject. `ALBUM` is a UI concept; the protocol kind is `VISUAL_COLLECTION`. |
| Any non-`AVAILABLE` status, including `TRASH` | Reject. |

`DestinationCapabilityDTO` now contains `region`, `evidence`, `destinationNodeId`, `name`, `kind`, `status`, and `message`. The former `rootOnly` field is removed. `Transfers.probeDestination(id)` performs a read-only probe; `planUpload` includes the same result.

| Probe status | Meaning |
|---|---|
| `pending` | Not checked successfully: credentials unavailable, HTTP 429, network failure or timeout. Configuration may be saved; uploads cannot execute. |
| `readable` | Exact target is readable and structurally suitable. Upload and album association have separate success conditions. |
| `failed` | Invalid target or definitive read failure. HTTP 401 requires reauthentication; 403 means no access; 404 means missing target. |

Every upload batch (including retry) and backup scan probes again before execution. The result is shared across that batch, not requested per file. Account/region/target changes invalidate previous results; existing tasks and backup baselines are bound to region, account root and target ID.

## Evidence Policy

`verified` records the SDK's existing production protocol evidence; `experimental` records routing without equivalent live-account evidence. A readable probe never changes this grade. The runtime-probe implementation is tested with injected HTTP transports for US, CA, UK, JP and EU routing. It does not add any real Amazon account verification evidence.

See [Choose an upload destination](../how-to/choose-upload-destination.md) and [Runtime destination validation](../explanation/runtime-destination-validation.md).
