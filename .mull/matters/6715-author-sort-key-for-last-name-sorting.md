---
status: raw
tags: [forage, sort]
created: 2026-03-06
updated: 2026-03-06
---

# Author sort key for last-name sorting

## Design

Add `sort_author` column to books table for last-name sorting.

**Schema**: `sort_author TEXT DEFAULT ''` with migration to backfill existing rows.

**Auto-populate**: On insert, derive sort key — "Frank Herbert" → "Herbert, Frank", "Homer" → "Homer". Override via `forage set <id> sort_author "Le Guin, Ursula K."`.

**Sorting**: TUI and PWA sort on `sort_author` instead of `author`. Display still shows full `author` everywhere.

**JSON**: Include `sort_author` in Book struct with `omitempty`.

**CLI**: `sort_author` becomes a valid key for `forage set`.

