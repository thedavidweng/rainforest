# Rainforest

<p align="center">
  <a href="https://github.com/thedavidweng/rainforest/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/thedavidweng/rainforest/ci.yml?branch=main&style=flat-square&label=ci" alt="CI"></a>
  <a href="https://github.com/thedavidweng/rainforest/releases"><img src="https://img.shields.io/github/v/release/thedavidweng/rainforest?style=flat-square" alt="Release"></a>
  <a href="https://github.com/thedavidweng/rainforest/blob/main/LICENSE"><img src="https://img.shields.io/github/license/thedavidweng/rainforest?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/go-%3E%3D1.27-blue?style=flat-square" alt="Go">
</p>

> **Disclaimer**: Unofficial desktop client; Not affiliated with, endorsed by, or sponsored by Amazon.com, Inc.

**Rainforest** is a lightweight, cross-platform desktop client for **one-time upload, one-way automatic folder backup, transfer queue monitoring, and verified recovery download** for Amazon Photos—**NOT** a web gallery clone.

Built on [`github.com/thedavidweng/amazon-photos`](https://github.com/thedavidweng/amazon-photos) and [`egoist/mygo`](https://github.com/egoist/mygo) v0.4.8 (`CGO_ENABLED=0`), Rainforest layers a React + TypeScript interface over `internal/gui`, `internal/engine`, `internal/store`, and `internal/credstore` without WebView cookie races ([MyGo #343](https://github.com/egoist/mygo/issues/343)) or custom-scheme video streaming pitfalls ([MyGo #161](https://github.com/egoist/mygo/issues/161)).

## Highlights

- **One-Time Upload with Preflight Inspection**: Drag-and-drop or native file/folder picker showing photo/video counts, total bytes, skipped unsupported files, local preview thumbnails, and conflict policy (`skip`, `rename`, `fail`) before starting.
- **Strictly One-Way Folder Backup**: Incremental local-to-cloud backup (`Photos only` or `Photos + Videos`, `Immediate` or `Interval`) with `source_unavailable` protection for unmounted external drives and mid-write file stability checks. Local deletions never delete cloud media.
- **Persistent Transfer Queue & Reconciliation**: SQLite-backed queue (`internal/store`) with pause, resume, cancel, retry, and MD5-verified startup reconciliation (`ReconcileOnStartup`) so interrupted transfers are never blindly re-POSTed.
- **Verified Recovery Download**: Recover original files by confirmed transfer history, remote folder/album, or node ID with size/MD5 verification and local file conflict protection.
- **OS-Protected Credential Vault**: Stores session cookies in `internal/credstore` (`0600` on Unix / `icacls` on Windows) with explicit regional routing (`verified` US; `experimental` CA, UK, JP, EU) and `aphotos.Redact` sanitization.

## Quickstart

### Install

Download the latest installer or archive for your platform (`macOS` Apple Silicon / Intel, `Windows` x64 / ARM64, `Linux` x64 / ARM64 `.deb` or `.tar.gz` with `install.sh`) from [GitHub Releases](https://github.com/thedavidweng/rainforest/releases).

The macOS v0.1.0 release contains `.app` archives, not a `.dmg` installer. The inspected Apple Silicon bundle is ad-hoc signed, without a Developer ID signature or stapled notarization ticket. First-install Gatekeeper approval and the complete in-app update flow have not been verified. See [release acceptance status](docs/reference/platforms-and-packages.md#v010-release-acceptance).

### Build from Source

```bash
git clone https://github.com/thedavidweng/rainforest.git
cd rainforest
mise install
bun install --cwd frontend

# Run all read-only Go, bindings drift, and frontend quality gates
mise run check

# Package desktop bundle via MyGo
bunx mygo build -skip-dmg -skip-notarize
```

## Documentation

Organized following the [Diátaxis](https://diataxis.fr/) framework:

### Tutorials (Learning-Oriented)

- [Connect Your Account and Upload Your First Photos](docs/tutorials/getting-started-upload.md)
- [Create Your First Automatic Backup Rule](docs/tutorials/first-backup-rule.md)

### How-To Guides (Task-Oriented)

- [Configure Backup Sources and Schedules](docs/how-to/configure-backups.md)
- [Pause, Resume, and Retry Transfers](docs/how-to/manage-transfer-queue.md)
- [Recover Files from the Cloud](docs/how-to/recover-files.md)
- [Resolve Unconfirmed Transfers](docs/how-to/resolve-unconfirmed-transfers.md)
- [Troubleshoot Platform Installation and Permissions](docs/how-to/troubleshooting-platform-and-permissions.md)

### Reference (Information-Oriented)

- [Supported Operating Systems and Installers](docs/reference/platforms-and-packages.md)
- [Application Settings and Defaults](docs/reference/settings.md)
- [Supported Media Types](docs/reference/supported-media-types.md)
- [Backup Scanning Rules and Transfer States](docs/reference/backup-and-transfer-states.md)
- [Regional Compatibility and Verification Status](docs/reference/regions.md)

### Explanation (Understanding-Oriented)

- [Rainforest Product Scope and Boundaries](docs/explanation/product-scope.md)
- [MyGo, React, and Go SDK Architecture](docs/explanation/architecture.md)
- [One-Way Backup and File Safety Design](docs/explanation/one-way-backup-and-file-safety.md)
- [Upload Reconciliation, Deduplication, and Recovery](docs/explanation/reconciliation-and-recovery.md)
- [Local Data and Credential Storage Model](docs/explanation/storage-and-credentials.md)

### Project & Architecture Records

- [ADR 0001: Rainforest Core Engine & State Store](docs/adr/0001-rainforest-architecture.md)
- [ADR 0002: MyGo Desktop Shell & Frontend Seam](docs/adr/0002-mygo-desktop-shell.md)
- [Domain Context](CONTEXT.md) & [Glossary](GLOSSARY.md)
- [Contributing](CONTRIBUTING.md) & [Security Policy](SECURITY.md)
- [Changelog](CHANGELOG.md)

## Acknowledgments & Credits

- [`thedavidweng/amazon-photos`](https://github.com/thedavidweng/amazon-photos) (MIT) — Go SDK (`pkg/aphotos`) and CLI
- [`thedavidweng/tg-drive`](https://github.com/thedavidweng/tg-drive) (AGPL-3.0-only) — Architectural inspiration for the GUI facade and frontend testing seam
- [`egoist/mygo`](https://github.com/egoist/mygo) v0.4.8 (MIT) — Pure-Go desktop application framework
- Upstream Amazon Photos protocol research: [`trevorhobenshield/amazon_photos`](https://github.com/trevorhobenshield/amazon_photos), [`kpfitzgerald/amazon-photos-mcp`](https://github.com/kpfitzgerald/amazon-photos-mcp), and [`vanessa/immich-amazon-sync`](https://github.com/vanessa/immich-amazon-sync)

## License

Copyright (C) 2026 Davy Weng and contributors.

Licensed under the **GNU Affero General Public License v3.0 only (`AGPL-3.0-only`)** — see [LICENSE](LICENSE).
