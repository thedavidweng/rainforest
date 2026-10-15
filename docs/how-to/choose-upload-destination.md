# Choose an Upload Destination

1. On the **Upload** page, choose or drop local files. In the review dialog, the **Upload to** row shows where files will go, for example `Amazon Photos › Trips`. Click **Change…** to browse Amazon Photos, open folders, and click **Choose this folder**. To use an exact ID, expand **Use a folder or album ID** in the picker. The default is the account root.
2. Check the displayed target name, protocol type, and `pending` / `readable` / `failed` status. Rainforest accepts `AVAILABLE` folders and albums (`VISUAL_COLLECTION`), and the destination picker browses folders read-only. Rainforest never renames, moves, or deletes remote folders.
3. Changing the destination rechecks it. Start when the target is `readable`. Rainforest checks again before execution. Readability does not prove upload permission or successful album association.

For automatic backups, open **Backup → Add folder** or **Edit…**, use **Upload to → Change…**, and watch the destination status. You can save a `pending` configuration while offline or without credentials. A failed or pending probe blocks execution; Rainforest keeps your ID and never silently redirects uploads to root.

- **401:** sign in again, then check again.
- **403:** select a target accessible to this account or correct its permissions.
- **404 / wrong ID / unsupported type / trash:** correct the target ID or restore the target using Amazon's interface.
- **429 / timeout / disconnected network:** reconnect or wait, then check again or click **Back up now**. Scheduled scans also recheck.

Experimental marketplace badges are informational. They do not disable destination selection, and a successful check does not promote the region to verified.

## Keep Folder Structure

When the destination is a folder and you choose a local folder such as `Trips`, Rainforest finds or creates `Trips` and its subfolders under the destination by exact name and uploads each file into the matching folder. Loose files go directly into the destination. Backup rules do the same for the rule's subdirectories. The **Keep folder structure** switch appears in the review dialog when your selection includes a folder; turn it off to upload everything flat into the destination. It is also in the backup dialog, on by default.

Album destinations are always flat because albums have no folders, so the switch is disabled with the hint "Albums don't have folders." If the mirrored folder cannot be resolved while checking an unconfirmed transfer, the item stays unconfirmed and nothing is uploaded again.

## Recover an Album Association

If media uploads but album association fails, the transfer is `partial` and the item is `album_pending`, with its uploaded media Node ID retained in Details. Click **Retry**: only album membership is retried, even if the local source file has been removed. A later backup scan also retries saved associations. The local source folder must be mounted for a backup scan to run.

Keep the same account, region and target for retries. To work under a different account, create a new job/rule. Restore the original account to resume an old task. When explicitly changing a backup rule's destination or source path, its local baseline is cleared; the new target is probed before a fresh scan. Existing remote files are never moved or deleted. Stop an active scan before editing its rule.
