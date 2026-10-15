# Connect Your Account and Upload Your First Photos

This tutorial walks you through launching Rainforest for the first time, signing in through Amazon’s website, running health checks, and uploading a photo with preflight inspection.

## Prerequisites

- Rainforest installed on macOS, Windows, or Linux.
- A desktop web browser signed in to Amazon Photos (`https://www.amazon.com/photos` or your regional marketplace).
- One or more local photo files (`.jpg`, `.png`, `.heic`, etc.).

## 1. Choose Your Marketplace

Launch Rainforest and open **Settings**. Choose the marketplace where your Amazon
Photos account is hosted: US, CA, UK, JP, DE, FR, IT, or ES. Language follows your
system language (English or Simplified Chinese), independently of marketplace.

## 2. Sign In on Amazon

Click **Sign in to Amazon**. Complete Amazon's password, MFA, and any challenge
steps in the separate Amazon window. Rainforest never asks you to paste cookies
in this normal flow. It captures the required session cookies natively, including
HttpOnly cookies, and checks account read access before saving them.

Wait for **Signed in**. Cancel the prompt or close the Amazon window to keep your
previous account. If the page cannot load, check your connection and retry. See
[sign-in troubleshooting](../how-to/sign-in.md) for expired sessions or manual
Cookie imports.

`experimental` remains a protocol evidence label. Successful sign-in does not
verify uploads, album membership changes, or the entire region.

## 3. Run Graded Diagnostics

Still in **Settings**, click **Run Doctor**:

- Rainforest checks local vault permissions (`0600` on Unix / `icacls` on Windows), cookie and region configuration, live account root and usage endpoints, and search read connectivity.

## 4. Plan and Confirm Your First Upload

1. Switch to the **Transfers / Upload** tab.
2. Drag and drop photos (or a folder of photos) onto the window, or click **Select Files** / **Select Folder**.
3. Rainforest scans local files and probes the chosen target read-only to build a **Preflight Upload Plan**:
   - Total photo and video counts and byte size.
   - Number of unsupported non-media files automatically skipped.
   - Local-only image preview thumbnails and dimensions.
   - Conflict policy selection (`Skip verified duplicates`, `Rename on conflict`, or `Fail on conflict`).
4. Review the destination Node ID (empty means account root), then click **Confirm & Start Upload**.
5. Watch real-time progress in the **Active Transfers** queue. Once Amazon Photos returns HTTP `200`/`201` with a verified remote node ID, the transfer moves to **Completed** (`confirmed`).

## Next Steps

- Set up continuous one-way folder backup in [Create Your First Automatic Backup Rule](first-backup-rule.md).
- Learn how to pause, resume, or retry transfers in [Pause, Resume, and Retry Transfers](../how-to/manage-transfer-queue.md).
