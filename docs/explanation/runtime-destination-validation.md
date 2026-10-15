# Why Destination Validation Happens at Runtime

Regional evidence and account authorization answer different questions. SDK evidence describes what protocol behavior has been observed for a marketplace. An account's ability to read a particular node depends on current credentials, permissions, node state, and identity. A regional label cannot prove or disprove that access.

Rainforest therefore preserves the user's target ID and checks it through the current account before each batch. Missing credentials or temporary connectivity failures leave the target pending. Definitive read errors fail visibly. Neither case changes the target or sends files elsewhere. The UI discards stale asynchronous probe responses when the target or account changes; execution does not trust a previous UI result.

A folder is a storage parent. An album is a visual collection, not a storage folder. For albums Rainforest uploads into the account root, confirms the media Node ID, saves it as `album_pending`, then calls the SDK's album-membership operation. Only a successful membership response marks the item confirmed. An upload failure and an association failure remain distinct.

The saved media ID is a recovery checkpoint. If association fails or its response is lost, retrying membership with that same ID avoids another non-idempotent media upload. Backup scans recover this checkpoint even when the individual local file has disappeared. Startup reconciliation only reads remote state; matching uploaded content for an album produces `album_pending`, never a false claim of completed membership.

SQLite `destination_bindings` ties tasks and backup baselines to the resolved account root, region, and exact target ID. Reusing a job after an account or region switch is rejected; returning to the original account permits a new probe and retry. Changing a backup target/source clears its local baseline transactionally. This deliberately requires a fresh scan rather than interpreting files uploaded elsewhere as present in the new destination.

These operations are additive. Failure causes no remote deletion, move, rename, or replacement. Existing remote media remains intact, including media already uploaded when its album association fails. See [reconciliation and recovery](reconciliation-and-recovery.md) for uncertain upload commits and [regional reference](../reference/regions.md) for statuses and routing.
