# Regional Compatibility and Verification Status

Reference for Amazon marketplace regions supported by Rainforest via `github.com/thedavidweng/amazon-photos/pkg/aphotos`.

## Regional Routing Table

| Region Code | Marketplace | Drive API (`DriveURL`) | Upload Host (`UploadURL`) | Required Canonical Cookies | Evidence Status | Destination Capability |
|---|---|---|---|---|---|---|
| `us` | United States | `https://www.amazon.com/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-main`, `at-main` | `verified` | Account root, folders, and albums (`rootOnly: false`) |
| `ca` | Canada | `https://www.amazon.ca/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbca`, `at-acbca` | `experimental` | Account root only (`rootOnly: true`) |
| `uk` | United Kingdom | `https://www.amazon.co.uk/drive/v1` | `https://content-eu.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbuk`, `at-acbuk` | `experimental` | Account root only (`rootOnly: true`) |
| `jp` | Japan | `https://www.amazon.co.jp/drive/v1` | `https://content-jp.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbjp`, `at-acbjp` | `experimental` | Account root only (`rootOnly: true`) |
| `de`, `fr`, `it`, `es`, `nl`, `se`, `pl`, `be`, `tr` | Europe | `https://www.amazon.{tld}/drive/v1` | `https://content-eu.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acb{tld}`, `at-acb{tld}` | `experimental` | Account root only (`rootOnly: true`) |
| `au` | Australia | `https://www.amazon.com.au/drive/v1` | `https://content-na.drive.amazonaws.com/cdproxy/nodes` | `session-id`, `ubid-acbau`, `at-acbau` | `experimental` | Account root only (`rootOnly: true`) |

## Evidence Grading Policy

- **`verified`**: Backed by recorded live account verification against the production marketplace (`us`). Supports targeting custom folder/album node IDs as well as the account root.
- **`experimental`**: Derived from static protocol routing and community pull requests. In `experimental` regions, Rainforest displays an explicit `experimental` badge and restricts upload destinations to the account root (`rootOnly: true`) until live account verification is recorded for that marketplace. Unverified regions are never promoted to `verified`.
