# Configure Backup Sources and Schedules

This guide shows how to configure, update, pause, and stop one-way folder backup rules in Rainforest.

## Select a Destination

In **Add folder** or **Edit…**, the **Upload to** row shows the destination as a path such as `Amazon Photos › Trips`. Click **Change…** to browse Amazon Photos read-only, open folders and choose one with **Choose this folder**; the default is the account root. To use an exact ID, expand **Use a folder or album ID** in the picker. The destination status shows its probe result. Pending configurations may be saved offline, but every scan must validate the target before uploading. See [Choose an upload destination](choose-upload-destination.md).

## Keep Folder Structure

**Keep folder structure** is on by default: subfolders of the backup folder are recreated under the destination. Turn it off to put every file directly in the destination. Albums don't have folders, so the switch is unavailable for an album destination.

## Review a Backup

The **Backup** page is a split view. Pick a folder in the list on the left to see its path, status, schedule and destination on the right, along with **Back up now** and the **…** menu (Pause or Resume, Edit…, Remove…). Below, **Backed up files** is a tree of the files Rainforest has recorded, each with its status, size and when it last synced.

## Choose Between Photos-Only and Photos + Videos

Amazon Prime accounts typically include unlimited full-resolution photo storage alongside a fixed video/file storage quota. Per backup rule, you can toggle **Include Videos**:

- **Unchecked (Photos only)**: Uploads only supported photo and RAW formats (`.jpg`, `.png`, `.heic`, `.dng`, `.cr2`, `.nef`, `.arw`, etc.) and ignores video files.
- **Checked (Photos + Videos)**: Uploads both supported photo and video formats (`.mp4`, `.mov`, `.m4v`, `.mkv`, etc.).

## Select Immediate or Interval Scheduling

Each backup rule supports two schedule modes:

- `immediate`: Runs an initial scan as soon as the rule is created or resumed, then polls every 5 minutes while Rainforest is running.
- `interval`: Polls every `intervalMinutes` (minimum 1 minute; default 15 minutes).

You can trigger an on-demand scan at any time by clicking **Back up now** on the folder card.

## Pause a Single Rule vs. Stop All Backups

- **Per-Rule Pause / Resume**: Choose **Pause** from the **…** menu of an individual folder to suspend scheduled scans for that folder and cancel any in-flight backup transfer for that rule. Choose **Resume** to re-enable scanning.
- **Pause all**: Click **Pause all** in the **Backup** header (**Resume all** undoes it) to set `backupsStopped = true` globally across all rules and cancel any running backup transfers.

## Handle Removable and External Drives Safely

If a backup rule points to a folder on an external USB/Thunderbolt drive or NAS mount:

- When the drive is unmounted or disconnected, `os.Stat` on the root path fails and Rainforest sets the rule status to `source_unavailable`.
- Rainforest never treats an unmounted volume as an empty directory and never deletes remote cloud files.
- Once you reconnect the drive, click **Back up now** (or wait for the next scheduled interval) and the rule automatically returns to `active`.
