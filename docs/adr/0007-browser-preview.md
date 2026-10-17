# 0007: Website Preview Uses the Native UI

## Status

Accepted.

## Context

Website screenshots drift from the app and cannot demonstrate navigation.

## Decision

Compile the same `internal/view` components to WebAssembly using MyGo's browser
runtime. The browser entry point injects sample DTOs through `view.Services`;
`internal/preview` contains data and service adapters, never UI components.
Preview mode disables file, account, transfer, backup and system mutations while
keeping navigation, details, folder browsing and language selection interactive.
No engine, database or credentials are initialized.

`mise run site-build` builds the website and browser binary together. Pages
rebuilds when Go code, dependencies, resources or locales change, so frontend
changes reach the preview without maintaining another interface.

## Consequences

The preview shares desktop layout and interaction code. It requires WebAssembly
and MyGo's canvas renderer; the first visit downloads the Go runtime and app.
Native SQLite registration is excluded from browser builds.
