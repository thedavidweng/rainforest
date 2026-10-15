# Connect Your Account and Upload Your First Photos

This tutorial walks you through launching Rainforest for the first time, signing in through Amazon’s website, running health checks, and uploading a photo with preflight inspection.

## Prerequisites

- Rainforest installed on macOS, Windows, or Linux.
- A desktop web browser signed in to Amazon Photos (`https://www.amazon.com/photos` or your regional marketplace).
- One or more local photo files (`.jpg`, `.png`, `.heic`, etc.).

## 1. Choose Your Marketplace

Launch Rainforest. On the Welcome screen, choose the marketplace where your Amazon
Photos account is hosted: US, CA, UK, JP, DE, FR, IT, or ES. Language follows your
system language (English or Simplified Chinese), independently of marketplace.

## 2. Sign In on Amazon

Click **Sign in with Amazon**. Complete Amazon's password, MFA, and any challenge
steps in the separate Amazon window. Rainforest never asks you to paste cookies
in this normal flow. It captures the required session cookies natively, including
HttpOnly cookies, and checks account read access before saving them.

Wait for the main window to open. Cancel the prompt or close the Amazon window to keep your
previous account. If the page cannot load, check your connection and retry. See
[sign-in troubleshooting](../how-to/sign-in.md) for expired sessions or manual
Cookie imports.

`experimental` remains a protocol evidence label. Successful sign-in does not
verify uploads, album membership changes, or the entire region.

## 3. Run Graded Diagnostics

Open **Settings** and click **Run diagnostics**:

- Rainforest checks local vault permissions (`0600` on Unix / `icacls` on Windows), cookie and region configuration, live account root and usage endpoints, and search read connectivity.

## 4. Plan and Confirm Your First Upload

1. Open the **Upload** page.
2. Drag and drop photos (or a folder of photos) onto the window, or click **Choose files…** / **Choose folder…**.
3. Rainforest scans local files and probes the chosen target read-only to build a **Preflight Upload Plan**:
   - Total photo and video counts and byte size.
   - Number of unsupported non-media files automatically skipped.
   - Local-only image preview thumbnails and dimensions.
   - Conflict policy selection (`Skip verified duplicates`, `Rename on conflict`, or `Fail on conflict`).
4. Review the **Upload to** path (the default is `Amazon Photos`, the account root) and click **Change…** to pick another folder. If you chose a folder, **Keep folder structure** recreates its subfolders in Amazon Photos. Then click **Upload N items**.
5. Watch real-time progress on the **Activity** page. Once Amazon Photos returns HTTP `200`/`201` with a verified remote node ID, the transfer is marked **Done** (`confirmed`).

## Next Steps

- Set up continuous one-way folder backup in [Create Your First Automatic Backup Rule](first-backup-rule.md).
- Learn how to pause, resume, or retry transfers in [Pause, Resume, and Retry Transfers](../how-to/manage-transfer-queue.md).
