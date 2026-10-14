# AGENTS.md

Instructions for every coding agent working in this repo. This is the only agent-instruction file — never create CLAUDE.md, .cursorrules, .windsurfrules, .clinerules, GEMINI.md, or any other per-tool variant (CI fails the build if one appears).

## Agent skills

### Issue tracker

Issues and specs live in GitHub Issues (`thedavidweng/rainforest`). See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical roles; label strings equal role names (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: root `GLOSSARY.md` / `CONTEXT.md` + `docs/adr/`. See `docs/agents/domain.md`.

## Code principles

1. Self-explanatory code, minimal comments. Write a comment only for a constraint the code cannot express. Decision rationale goes in `docs/adr/`, conventions go here. CI enforces a comment budget of 5% of non-test Go lines.
2. Product scope is strictly one-time upload, one-way folder backup, transfer queue, and lightweight recovery download. Never add cloud photo wall, gallery timeline, AI search, album/folder manager, or trash UI pages.
3. `internal/gui` and `internal/core` never import `github.com/egoist/mygo`. MyGo lives only in the thin desktop entrypoint (`cmd/rainforest` or root main).

## Verification

`mise run check` must pass before every push. CI runs the same gates.
