# Local Data and Credential Storage Model

This page explains how Rainforest resolves and secures its local persistent directory for SQLite operational state and Amazon session credentials.

## Persistent User Data Directory Resolution (`credstore.ResolveDataDir`)

On startup, Rainforest resolves its persistent data directory in the following order:

1. `mygo.App.Path(mygo.PathUserData)` when running inside the MyGo desktop runtime.
2. `os.UserConfigDir()` + `/rainforest`:
   - **macOS**: `~/Library/Application Support/rainforest` (or `com.thedavidweng.rainforest` via MyGo)
   - **Windows**: `%APPDATA%\rainforest`
   - **Linux**: `$XDG_CONFIG_HOME/rainforest` (or `~/.config/rainforest`)
3. `os.UserHomeDir()` + `/.config/rainforest`.

### No Silent Fallback to Temporary Directories

Storing credentials or backup state in `/tmp` or `%TEMP%` would expose sensitive cookies on shared systems and cause backup history (`rainforest.db`) to be wiped across reboots, leading to duplicate uploads or lost unconfirmed transfer records.

`credstore.ResolveDataDir` enforces three hard checks:

1. The resolved path must be non-empty and absolute (`filepath.IsAbs`).
2. The resolved path must **not** equal or reside inside `os.TempDir()`.
3. The directory must be created with `0700` permissions and pass platform permission enforcement (`enforceDirPermissions`).

If any check fails, Rainforest refuses to start and shows an explicit `Rainforest Storage Error` dialog rather than falling back to a temporary or relative directory.

## Credential Vault (`credentials.json`)

`internal/credstore.FileStore` stores:

- `region`: Explicit or unambiguously inferred Amazon marketplace code.
- `cookies`: Canonical hyphenated browser session cookies (`session-id`, `ubid-*`, `at-*`).
- Optional diagnostic endpoint overrides (`driveUrl`, `uploadUrl`, `thumbnailUrl`).

Writes are performed atomically via a temporary `.credentials-*.tmp` file in the same directory:

- **Unix (macOS / Linux)**: Enforces `0700` on the parent directory and `0600` on the vault file before and after rename.
- **Windows**: Executes `icacls` (`permissions_windows.go`) to strip inherited ACLs and grant full control exclusively to the current user's SID.

## Operational SQLite Database (`rainforest.db` / `state.db`)

`internal/store.Store` uses pure-Go `modernc.org/sqlite` (`CGO_ENABLED=0`) in `WAL` journal mode with foreign keys enabled (`PRAGMA foreign_keys = ON`) and five tables:

- `settings`: Singleton configuration row (`id = 1`).
- `backup_rules`: Configured local backup folders, schedules, and last scan status/error.
- `tracked_files`: Per-rule baseline of local relative path, byte size, `mtime_ns`, MD5, remote node ID, and confirmation status.
- `transfers`: Upload, backup, and download job summaries.
- `transfer_items`: Individual file records within each transfer.
