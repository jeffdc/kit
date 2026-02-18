# Add Enhancements & Batch Link

## Problem

The most common LLM workflow — add → append → link → docket add — takes four separate commands. This creates unnecessary round-trips for the primary consumer (other LLMs via Claude Code skills).

## Changes

### 1. `add` gets `--body`, link flags, and `--docket`

```
mull add "Title" --status raw --epic platform \
  --body "Description here" \
  --relates 1edb \
  --docket
```

New flags on `add`:
- `--body <text>` — sets the matter body at creation time
- `--relates <id>` (repeatable) — creates `relates` links
- `--blocks <id>` (repeatable) — creates `blocks` links
- `--needs <id>` (repeatable) — creates `needs` links
- `--parent <id>` — sets parent (single value, not repeatable)
- `--docket` — adds the new matter to the end of the docket

Side effect order: create → set body → create links → add to docket.

Fail-fast on errors. If linking fails, the matter and body already exist — the error tells you what to fix. No rollback.

Output: the matter JSON (with links populated), same as today. Docket addition is a silent side effect.

### 2. Batch targets on `link`

```
mull link 9ad7 relates 1edb 8ae6
```

Change args from `ExactArgs(3)` to `MinimumNArgs(3)`. Everything from position 2 onward is a target ID. Fail-fast on errors.

Output: array of link results instead of a single object when multiple targets are given; single object when one target (backward compatible).

## Files touched

- `cmd/add.go` — new flags and post-creation side effects
- `cmd/link.go` — variadic target args
- Tests for both

## Not doing

- All-or-nothing rollback on `add` side effects
- Continue-on-error for batch operations
- `--docket-after` or `--docket-note` on `add` (YAGNI — can use `docket move`/`docket add --note` after)
