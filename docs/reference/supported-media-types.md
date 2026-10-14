# Supported Media Types

Reference for photo, RAW, and video file extensions recognized by Rainforest's upload planner (`PlanUpload`) and one-way backup scanner (`ScanNow`).

## Photo and RAW Extensions (`MediaTypePhoto`)

Case-insensitive extensions classified as `photo`:

| Category | Extensions |
|---|---|
| Standard Raster Photos | `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.heic`, `.heif`, `.avif`, `.bmp`, `.tif`, `.tiff` |
| Camera RAW Formats | `.dng`, `.cr2`, `.cr3`, `.nef`, `.arw`, `.orf`, `.rw2`, `.raf` |

## Video Extensions (`MediaTypeVideo`)

Case-insensitive extensions classified as `video` (included when `IncludeVideos` is `true`):

| Category | Extensions |
|---|---|
| Video Containers | `.mp4`, `.mov`, `.m4v`, `.avi`, `.mkv`, `.webm`, `.3gp`, `.mts`, `.m2ts` |

## Filtering and Preflight Preview Rules

- **Unsupported Extensions**: Any file whose extension is not in the photo or video tables above is skipped during directory walks and counted under `UploadPlan.skippedUnsupported`.
- **Hidden Files and Directories**: Entries whose name starts with `.` (such as `.DS_Store` or `.git`) and non-regular files (symlinks, sockets, devices) are always skipped.
- **Local Preflight Thumbnails**: During `PlanUpload`, Rainforest decodes local `.jpg`, `.jpeg`, `.png`, and `.gif` files up to `20 MiB` (`maxThumbnailSourceBytes`) for the first `32` items (`maxPlanPreviewThumbs`) and scales them in memory to a maximum of `128x128` pixels (`data:image/png;base64,...`). No remote thumbnail URLs or WebView custom schemes are used.
