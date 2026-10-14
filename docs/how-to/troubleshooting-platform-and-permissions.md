# Troubleshoot Platform Installation and Permissions

This guide covers common installation, OS permission, and startup issues on macOS, Windows, and Linux.

## macOS Gatekeeper Warning on Unsigned Builds

If you download a CI dry-run build or an ad-hoc signed `.app` / `.dmg` that has not been notarized with an Apple Developer ID:

```sh
xattr -dr com.apple.quarantine /Applications/Rainforest.app
```

Official signed releases notarized with Developer ID open without quarantine prompts.

## Windows SmartScreen and WebView2 Runtime

- **WebView2 Runtime**: Rainforest uses Microsoft Edge WebView2 (preinstalled on Windows 10/11). If WebView2 is missing on a stripped Windows image, install the Evergreen WebView2 Runtime from Microsoft.
- **Credential Vault ACLs**: Rainforest restricts `credentials.json` to the current Windows user via `icacls`. Ensure your user profile directory (`%APPDATA%\rainforest` or `%LOCALAPPDATA%\rainforest`) is on an NTFS volume that supports standard user ACLs.

## Linux WebKitGTK Dependencies and `.deb` / `install.sh`

On Ubuntu/Debian systems, install the `.deb` package so `apt` resolves runtime libraries automatically:

```sh
sudo apt install ./rainforest_0.1.0_amd64.deb
```

Or use the user-local `install.sh` script included in the release archive (installs into `~/.local/rainforest.app`, which supports in-app auto-updates).

## Persistent User Data Directory Errors on Startup

Rainforest stores `rainforest.db` (SQLite state) and `credentials.json` in your OS user configuration/data directory (`~/Library/Application Support/com.thedavidweng.rainforest` on macOS, `%APPDATA%` on Windows, `$XDG_CONFIG_HOME/rainforest` or `~/.config/rainforest` on Linux).

For security and data durability, Rainforest **refuses** to fall back to `/tmp` or `%TEMP%` if your user home/config directory is unavailable or unwritable, and displays a **Rainforest Storage Error** dialog instead. Verify that `$HOME` / `$XDG_CONFIG_HOME` exists and is writable by your user account (`chmod 700`).

## Check Vault Permissions with Built-In Diagnostics

Open **Settings** → **Run Diagnostics**. Grade 1 (`vault_permissions`) verifies that `credentials.json` is mode `0600` inside a `0700` directory on Unix (or ACL-restricted on Windows) and provides the exact `chmod` / `icacls` remediation hint if permissions were broadened.
