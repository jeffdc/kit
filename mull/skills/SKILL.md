---
name: mull
description: Use when discussing project ideas, features, or priorities - working on existing matters, capturing new ones, or consulting the docket/roadmap
---

# mull

Conversational wrapper around the `mull` CLI for capturing and refining matters.

## Orientation (always first)

1. Run `mull prime`
2. If `$ARGUMENTS`: `mull search <args>` — match → work on matter, no match → create new
3. No arguments → present landscape, ask what to work on

## Working on a Matter

`mull show <id>` and `mull graph <id>` to load context. Follow user's lead:
- `mull append <id> "<text>"` for details
- `mull set <id> <key> <value>` for metadata
- `mull link <id> <type> <id>` for relationships

## Creating a New Matter

1. `mull add "<title>" --status raw --epic <name>` (epic is optional)
2. One question at a time, `mull append` as details emerge
3. Check `mull prime` for relationships to existing matters

## Docket

- `mull docket` — the prioritized work queue
- `mull docket --invert` — matters NOT on the docket
- `mull epics` — list all epics with counts
- `mull list --epic <name>` — filter by epic
- When user asks "what next?": `mull docket` + `mull graph`. Present options conversationally.

## Statuses

Valid statuses: raw, refined, planned, done, dropped. No others accepted.

## Closing vs Deleting

- `mull done <id>` — marks as done, matter stays for reference. **This is almost always what you want.**
- `mull drop <id>` — decided against, matter stays for reference
- `mull rm <id>` — **permanent delete**, only for junk/mistakes

## Cross-Skill Capture

Active matter in context + related work completed elsewhere → `mull append` findings, `mull done <id>` when complete.

## Principles

- **Capture as you go** — don't wait until the end
- **Match user's energy** — a tickler is not a spec, don't over-process
- **Don't push toward planning** — only when user signals execution intent
