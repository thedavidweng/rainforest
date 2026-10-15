# Choose an Upload Destination

1. In **Transfers / Upload**, select or drop local files. In the confirmation dialog, enter an existing folder or album's exact **Destination Node ID**. Leave it blank to use the current account root.
2. Check the displayed target name, protocol type, and `pending` / `readable` / `failed` status. Rainforest accepts `AVAILABLE` folders and albums (`VISUAL_COLLECTION`), without creating or browsing a full cloud library.
3. Use **Check Destination** to recheck after reconnecting. Start when the target is `readable`. Rainforest checks again before execution. Readability does not prove upload permission or successful album association.

For automatic backups, open **Backups → Add Backup Folder** or **Edit**, set **Destination Node ID**, and use **Check Destination**. You can save a `pending` configuration while offline or without credentials. A failed or pending probe blocks execution; Rainforest keeps your ID and never silently redirects uploads to root.

- **401:** import fresh cookies in Settings, then check again.
- **403:** select a target accessible to this account or correct its permissions.
- **404 / wrong ID / unsupported type / trash:** correct the target ID or restore the target using Amazon's interface.
- **429 / timeout / disconnected network:** reconnect or wait, then check again or run **Scan Now**. Scheduled scans also recheck.

Experimental marketplace badges are informational. They do not disable destination selection, and a successful check does not promote the region to verified.

## Recover an Album Association

If media uploads but album association fails, the transfer is `partial` and the item is `album_pending`, with its uploaded media Node ID retained in Details. Click **Retry**: only album membership is retried, even if the local source file has been removed. A later backup scan also retries saved associations. The local source folder must be mounted for a backup scan to run.

Keep the same account, region and target for retries. To work under a different account, create a new job/rule. Restore the original account to resume an old task. When explicitly changing a backup rule's destination or source path, its local baseline is cleared; the new target is probed before a fresh scan. Existing remote files are never moved or deleted. Stop an active scan before editing its rule.
