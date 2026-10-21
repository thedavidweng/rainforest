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
- **Local Preflight Thumbnails**: During `PlanUpload`, Rainforest builds a local thumbnail for the first `32` photos (`maxPlanPreviewThumbs`), longest side `320` pixels. JPEG files above 2 megapixels are decoded with 1/8-step IDCT scaling, so a multi-megabyte camera JPEG is not loaded at full resolution. TIFF-based RAW (including DNG, CR2, NEF, ARW, RW2) and CR3 use the largest embedded JPEG preview, capped at `20 MiB`. HEIC, HEIF, AVIF, and other containers use an embedded JPEG when one is present in the first `8 MiB`. PNG, GIF, WebP, BMP, and TIFF without a preview are fully decoded only when the bitmap stays within `24` megapixels. There is no source-file size cutoff. No remote thumbnail URLs or WebView custom schemes are used.
