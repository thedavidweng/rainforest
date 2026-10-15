# Configure Backup Sources and Schedules

This guide shows how to configure, update, pause, and stop one-way folder backup rules in Rainforest.

## Select a Destination

Set **Destination Node ID** when adding or editing a rule; blank means account root. Check its name, type and probe status. Pending configurations may be saved offline, but every scan must validate the target before uploading. See [Choose an upload destination](choose-upload-destination.md).

## Choose Between Photos-Only and Photos + Videos

Amazon Prime accounts typically include unlimited full-resolution photo storage alongside a fixed video/file storage quota. Per backup rule, you can toggle **Include Videos**:

- **Unchecked (Photos only)**: Uploads only supported photo and RAW formats (`.jpg`, `.png`, `.heic`, `.dng`, `.cr2`, `.nef`, `.arw`, etc.) and ignores video files.
- **Checked (Photos + Videos)**: Uploads both supported photo and video formats (`.mp4`, `.mov`, `.m4v`, `.mkv`, etc.).

## Select Immediate or Interval Scheduling

Each backup rule supports two schedule modes:

- `immediate`: Runs an initial scan as soon as the rule is created or resumed, then polls every 5 minutes while Rainforest is running.
- `interval`: Polls every `intervalMinutes` (minimum 1 minute; default 15 minutes).

You can trigger an on-demand scan at any time by clicking **Scan Now** on the rule card.

## Pause a Single Rule vs. Stop All Backups

- **Per-Rule Pause / Resume**: Click **Pause** on an individual backup rule to suspend scheduled scans for that folder and cancel any in-flight backup transfer for that rule. Click **Resume** to re-enable scanning.
- **Stop All Backups**: Click **Stop All Backups** in the **Backups** header to set `backupsStopped = true` globally across all rules and cancel any running backup transfers.

## Handle Removable and External Drives Safely

If a backup rule points to a folder on an external USB/Thunderbolt drive or NAS mount:

- When the drive is unmounted or disconnected, `os.Stat` on the root path fails and Rainforest sets the rule status to `source_unavailable`.
- Rainforest never treats an unmounted volume as an empty directory and never deletes remote cloud files.
- Once you reconnect the drive, click **Scan Now** (or wait for the next scheduled interval) and the rule automatically returns to `active`.
