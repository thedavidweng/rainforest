# 0006: Mirrored Folder Structure

## Status

Accepted.

## Context

Uploading or backing up a local folder placed every file directly in the
destination, so a tree such as `Trips/2026/Rome` lost its organization in
Amazon Photos.

## Decision

1. Uploading a local folder `Trips` to destination folder `D` finds or creates
   `D/Trips` and its subfolders. Loose files go to `D`. A backup rule on folder
   `F` mirrors the subdirectories of `F` under `D`.
2. Remote folders are resolved by exact name under the parent: query first,
   create when missing, and query again if the create conflicts. Results are
   cached per destination probe.
3. Mirroring applies to folder destinations only. Album destinations stay flat
   and report `StructureSupported=false`.
4. `KeepFolderStructure` is true by default for uploads and new backup rules.
   Existing rules retain their flat layout and tracked-file baseline on upgrade.
   Turning it off uploads flat. Changing it on a rule clears the rule's local
   baseline.
5. The relative remote directory is stored per item in `transfer_items.remote_dir`
   so resume, retry, and `ReconcileOnStartup` use the same parent.
6. Reconciliation never creates folders. If the mirrored parent cannot be
   resolved, the item stays unconfirmed and is not uploaded again.
7. The destination picker is read-only: it lists folders with breadcrumbs, capped
   at 1000 entries with a truncation flag. Rainforest creates folders only to
   mirror local structure and never renames, moves, or deletes them.

## Consequences

Folder creation is the only remote mutation added besides uploads. Product scope
in `AGENTS.md` allows the read-only picker and mirroring, and still excludes a
folder manager.
