# Application Settings and Defaults

Reference for all persistent settings stored in the SQLite `settings` table (`internal/store`) and exposed via the `Settings` service (`internal/gui`).

## Settings Fields and Defaults

| Field (`SettingsDTO`) | SQLite Column | Type | Default | Constraints & Behavior |
|---|---|---|---|---|
| `region` | `region` | `string` | `""` (inferred on cookie import) | Amazon marketplace code (`us`, `ca`, `uk`, `jp`, `de`, `fr`, `it`, `es`, `nl`, `se`, `pl`, `be`, `tr`, `au`). Synchronized with `credentials.json`. |
| `maxMetadataConcurrency` | `max_metadata_concurrency` | `int` | `4` | Clamped to `1`–`16`. Controls `aphotos.Options.MaxMetadataConcurrency` for metadata and download requests. |
| `maxUploadConcurrency` | `max_upload_concurrency` | `int` | `2` | Clamped to `1`–`8`. Controls `aphotos.Options.MaxUploadConcurrency` and worker pool size for upload/backup/download jobs. |
| `defaultConflictPolicy` | `default_conflict_policy` | `string` | `"skip"` | Allowed values: `"skip"`, `"rename"`, `"fail"`. Used when a transfer or backup rule omits an explicit conflict policy. |
| `launchAtLogin` | `launch_at_login` | `bool` | `false` | Configures OS login autostart (`~/Library/LaunchAgents` on macOS, `~/.config/autostart` on Linux, Registry `Run` key on Windows). |
| `closeToTray` | `close_to_tray` | `bool` | `true` | When `true`, closing the main window hides it to the system tray instead of quitting the process. |
| `notificationsEnabled` | `notifications_enabled` | `bool` | `true` | Enables desktop notifications when transfers reach a terminal stage (`completed`, `partial`, `unconfirmed`, `failed`). |
| `backupsStopped` | `backups_stopped` | `bool` | `false` | Global kill-switch that pauses all scheduled backup scans and cancels active backup transfers when `true`. |

## Graded `Doctor` Checks

`Settings.Doctor` and `Settings.DoctorWithWriteProbe(confirm)` return a 5-grade `DoctorReportDTO`:

| Grade | Name | Description |
|---|---|---|
| `1` | `vault_permissions` | Checks `0600` file / `0700` dir permissions on Unix or `icacls` user ACL on Windows for `credentials.json`. |
| `2` | `credentials_and_region` | Validates stored cookies (`session-id`, `ubid-*`, `at-*`) and explicit/inferred region. |
| `3` | `account_root_and_usage` | Verifies `GetRoot` and `GetUsage` against the configured regional `DriveURL`. |
| `4` | `search_read_probe` | Verifies `Search(limit=1)` read connectivity. |
| `5` | `write_probe` | Skipped by default; when invoked via `DoctorWithWriteProbe(true)`, creates and moves to trash a temporary `.rainforest-doctor-probe` folder. |
