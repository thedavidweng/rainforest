# Create Your First Automatic Backup Rule

This tutorial walks you through creating a strictly one-way automatic folder backup rule in Rainforest.

## Prerequisites

- Rainforest connected to your Amazon Photos account (see [Connect Your Account and Upload Your First Photos](getting-started-upload.md)).
- A local folder containing photos or videos you want backed up to Amazon Photos.

## 1. Open the Backup Page

1. Launch **Rainforest** and click **Backup** in the sidebar.
2. Click **Add folder** and pick a local folder.

## 2. Configure Your Backup Rule

Choose your rule settings:

- **Local Folder**: The directory on your computer or external drive to monitor.
- **Include Videos**: Check to back up both photos and videos, or uncheck for **Photos only** (useful when video storage is quota-limited).
- **Schedule Mode**:
  - **Immediate**: Scans immediately when added/resumed and re-checks every 5 minutes.
  - **Interval**: Scans on your chosen interval (for example, every 15, 30, or 60 minutes).
- **Upload to**: The Amazon Photos folder that receives the backup; click **Change…** to pick one.
- **Keep folder structure**: On by default, so subfolders are recreated in Amazon Photos.
- **Conflict Policy**: Defaults to `skip` (skips upload only when a remote node with the same name and verified MD5 already exists).

Click **Add folder** in the dialog.

## 3. Run and Verify the Initial Scan

1. If your rule is set to **Immediate**, Rainforest starts scanning right away; otherwise click **Back up now**.
2. Rainforest walks the folder, skips hidden files and non-media extensions, and creates a `backup` transfer job in **Activity**.
3. For each file, Rainforest verifies that its local byte size and modification timestamp (`mtime_ns`) remain stable before and after streaming, then records the confirmed remote node ID and MD5 in its local `tracked_files` table.

Select the folder in the **Backup** list to see its **Backed up files** tree fill in as files are confirmed.

## 4. Verify Incremental Behavior

Click **Back up now** a second time without modifying any files in the folder:

- Because every file's `size` and `mtime_ns` match the confirmed baseline in SQLite, Rainforest completes the scan in milliseconds without creating a new upload job or re-sending any bytes.
- Deleting or moving a file locally—or removing the backup rule—never deletes or trashes the remote copy in Amazon Photos.

## Next Steps

- Customize schedules and destination folders in [Configure Backup Sources and Schedules](../how-to/configure-backups.md).
- Understand how unconfirmed uploads are handled in [Resolve Unconfirmed Transfers](../how-to/resolve-unconfirmed-transfers.md).
