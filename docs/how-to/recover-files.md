# Recover Files from the Cloud

This guide shows how to download original media files from Amazon Photos to a local directory using Rainforest's **Restore** page.

## 1. Choose a Local Destination Folder

1. Open the **Restore** page in Rainforest.
2. Enter a folder under **Save to** or click **Choose…** to select where recovered files should be written.
3. Choose **If a file already exists**:
   - `skip` (default): Preserves existing local files and marks matching filenames as `skipped`.
   - `rename`: Saves downloaded files with a disambiguated name (`IMG_0001 (1).JPG`) if the filename already exists.
   - `fail`: Marks the item `failed` if the local file already exists.

## 2. Select Files to Recover

Rainforest supports three recovery sources:

- **Confirmed Local Transfer History**: Select individual files that Rainforest previously uploaded or backed up from this machine (includes stored `remoteNodeId`, `size`, and `md5`).
- **Remote Folders or Albums**: Select a listed cloud folder or album (`VISUAL_COLLECTION`), or click **Browse folders…** to choose any folder in Amazon Photos, to recover all file nodes inside that container.
- **Explicit Remote Node IDs**: Paste one or more Amazon Photos node IDs directly.

Click **Download N items**. Progress appears on the **Activity** page.

## 3. Verify Downloaded Files

For every downloaded item, Rainforest and the underlying `pkg/aphotos` SDK:

1. Stream the file via `/nodes/{id}/contentRedirection` (stripping session cookies on cross-host CDN redirects) into a temporary `.aphotos-dl-*.part` file.
2. Verify the downloaded byte count against `Content-Length` and expected node size.
3. Compute the MD5 checksum while streaming and verify it against the remote node's `contentProperties.md5`.
4. Atomically finalize the file into the destination directory without overwriting existing files.
