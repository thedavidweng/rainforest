# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in **Rainforest**, please report it privately:

1. **Do not open a public GitHub issue.**
2. Use [GitHub Private Vulnerability Reporting](https://github.com/thedavidweng/rainforest/security/advisories/new).
3. Include a clear description of the vulnerability, reproduction steps, affected platform(s), and potential impact.
4. You will receive an initial response within 7 days.

## Scope

Rainforest manages sensitive Amazon Photos browser session cookies (`session-id`, `ubid-*`, `at-*`) and performs local-to-cloud media backups and recovery downloads. Security issues in scope include:

- **Credential exposure**: Leakage of raw session cookies (`session-id`, `ubid-*`, `at-*`), `x-amzn-sessionid`, tokens, or pre-signed `contentRedirection` URLs in IPC error payloads, diagnostic reports, logs, or UI views.
- **Insecure credential storage**: Weak file or directory permissions on the local credential vault (`internal/credstore`).
- **Path traversal or arbitrary file overwrite**: Unsafe handling of remote filenames or `Content-Disposition` headers during recovery downloads.
- **Unintended destructive operations**: Any code path in one-way backup or reconciliation that deletes or trashes remote cloud media when local files are deleted or external drives disconnect.
- **TLS or host-routing bypass**: Disabling HTTPS certificate verification or sending Amazon session cookies to untrusted third-party hosts.

## Security Design

- **OS-Protected Credential Vault (`internal/credstore`)**:
  - On Unix (macOS and Linux), `credentials.json` is written atomically via a temporary file with `0600` file permissions inside a `0700` parent directory.
  - On Windows, `applyPlatformPermissions` restricts access to the current user via `icacls` (`SID:(F)` with inheritance removed).
- **WebView Cookie Isolation ([MyGo #343](https://github.com/egoist/mygo/issues/343))**:
  - Amazon session cookies are never injected into or read from the system WebView (`WKWebView`, `WebKitGTK`, or `WebView2`). All authenticated API calls are made directly by the Go backend via `github.com/thedavidweng/amazon-photos/pkg/aphotos`.
- **Centralized Redaction (`aphotos.Redact`)**:
  - Every `internal/gui` error (`toGUIError`), engine transfer error, and diagnostic check result is scrubbed with `aphotos.Redact` before crossing the MyGo IPC boundary or being stored in SQLite.
- **Verified Atomic Downloads**:
  - Recovery downloads stream via the SDK to a temporary `.part` file, sanitize filenames against path traversal (`..`, `/`, `\`, Windows reserved names), and verify expected byte size and MD5 checksum before atomic rename.
- **TLS Enforced & No Telemetry**:
  - Standard Go TLS certificate validation is always enforced (`InsecureSkipVerify` is never used).
  - Rainforest collects zero telemetry and communicates solely with the configured regional Amazon Photos / Drive endpoints.
