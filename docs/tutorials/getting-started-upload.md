# Connect Your Account and Upload Your First Photos

This tutorial walks you through launching Rainforest for the first time, importing your Amazon Photos session cookies, running health checks, and uploading a photo with preflight inspection.

## Prerequisites

- Rainforest installed on macOS, Windows, or Linux.
- A desktop web browser signed in to Amazon Photos (`https://www.amazon.com/photos` or your regional marketplace).
- One or more local photo files (`.jpg`, `.png`, `.heic`, etc.).

## 1. Export Your Browser Session Cookies

Amazon Photos authenticates using three browser session cookies:

- `session-id`
- `ubid-main` (US) or `ubid-acb<tld>` (regional marketplaces such as `ca`, `uk`, `jp`, `de`)
- `at-main` (US) or `at-acb<tld>` (regional marketplaces)

Save these three cookies in a local `cookies.json` or Netscape `cookies.txt` file, or copy them as a JSON map:

```json
{
  "session-id": "140-0000000-0000000",
  "ubid-main": "130-0000000-0000000",
  "at-main": "Atza|..."
}
```

## 2. Import Cookies in Settings

1. Launch **Rainforest** and open the **Settings** tab.
2. Select your **Amazon Region** (or leave it on automatic inference if your cookie suffixes uniquely identify one marketplace).
3. Either paste your cookie JSON/header into the **Cookie Input** box and click **Save & Connect**, or click **Import Cookie File…** to pick your `cookies.json` / `cookies.txt` file.
4. Once connected, the status badge turns **Connected**, displays your resolved region and verification grade (`verified` for US, `experimental` for other regions), and loads your storage usage (`Photos`, `Videos`, `Other`).

## 3. Run Graded Diagnostics

Still in **Settings**, click **Run Diagnostics**:

- Rainforest checks local vault permissions (`0600` on Unix / `icacls` on Windows), cookie and region configuration, live account root and usage endpoints, and search read connectivity.

## 4. Plan and Confirm Your First Upload

1. Switch to the **Transfers / Upload** tab.
2. Drag and drop photos (or a folder of photos) onto the window, or click **Select Files** / **Select Folder**.
3. Rainforest computes a local **Preflight Upload Plan** before sending any network traffic:
   - Total photo and video counts and byte size.
   - Number of unsupported non-media files automatically skipped.
   - Local-only image preview thumbnails and dimensions.
   - Conflict policy selection (`Skip verified duplicates`, `Rename on conflict`, or `Fail on conflict`).
4. Click **Start Upload**.
5. Watch real-time progress in the **Active Transfers** queue. Once Amazon Photos returns HTTP `200`/`201` with a verified remote node ID, the transfer moves to **Completed** (`confirmed`).

## Next Steps

- Set up continuous one-way folder backup in [Create Your First Automatic Backup Rule](first-backup-rule.md).
- Learn how to pause, resume, or retry transfers in [Pause, Resume, and Retry Transfers](../how-to/manage-transfer-queue.md).
