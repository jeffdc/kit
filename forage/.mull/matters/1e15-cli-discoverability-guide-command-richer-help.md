---
status: raw
tags: [forage]
created: 2026-03-02
updated: 2026-03-03
---

# CLI discoverability: guide command + richer help

## Design: CLI discoverability

Two changes to make forage self-documenting for both humans and LLMs.

### 1. `forage guide` command

New subcommand that prints a structured plain-text reference covering:

- Quick start — add a book, list, show, rate, mark read
- Statuses — the five valid statuses and what they mean
- Fields — every field, type, valid values, which are settable via `set`
- Commands — each command with usage, example, and output shape
- Workflows — common sequences (import → curate → read → rate → export)
- LLM usage — `prime` for context snapshots, JSON output contract

Plain text/markdown to stdout. No pagination, no flags.

### 2. Enriched `--help` on each command

Expand Cobra `Long` field on every command with:

- Valid values (statuses, settable keys, rating range)
- One or two usage examples
- Brief output shape description

3-8 lines max per command. Guide has full picture; help has quick reference.

### Out of scope

- Man pages or external docs
- `--format` flag on guide
- Generated-from-code approach (guide is a string literal)

