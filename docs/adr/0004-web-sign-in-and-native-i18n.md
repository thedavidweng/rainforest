# 0004: Native web sign-in and shared MyGo catalogs

## Status

Accepted. Updates the framework version and authentication UI in ADR 0002.

## Context

Manual browser Cookie export is difficult for ordinary desktop users. MyGo
0.5.1 provides native Cookie APIs (including HttpOnly) and built-in ICU catalogs
shared with its TypeScript runtime. Public Amazon OAuth does not grant Photos
session access. A successful read must not be mistaken for write verification.

## Decision

Use a separate HTTPS Amazon window with no trusted origins. Capture only the
selected marketplace's required cookies in Go, verify them with `GetRoot`, clear
the app's Amazon browser cookies, then commit the new vault. Serialize account
changes; cancel and failure preserve the previous account. The CLI separately
provides hidden interactive Cookie paste and retains file import for automation.

Use MyGo 0.5.1 catalogs for English and Simplified Chinese. OS locale and Amazon
marketplace are independent. React subscribes to MyGo locale changes. Generated
Go/TypeScript catalogs and bindings are checked in and checked for drift.

## Consequences

Amazon owns password/MFA/challenge screens. The application still relies on
unofficial session-authenticated APIs, and Amazon may reject embedded browsers.
Such rejection is visible; the app never silently changes authentication methods.
Read success does not change regional evidence or confirm any remote write.
Mock tests cover the application boundary, not Amazon's current login policy.
