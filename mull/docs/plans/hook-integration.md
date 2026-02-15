# Hook-Based Integration for Mull

Support both skill-based and hook-based Claude Code integration, letting users choose their preferred approach.

## Background

Currently mull's workflow instructions live entirely in `skills/SKILL.md`, requiring explicit invocation each session. Beads solves this with `bd prime` outputting workflow context via Claude Code hooks on `SessionStart`/`PreCompact`. We want to support both: skill for superpowers users, hooks for everyone else.

## Changes

### 1. Add `--context` flag to `mull prime`

**File:** `cmd/prime.go`

Add a `--context` flag that wraps the JSON data with markdown workflow instructions — the same knowledge currently in the skill. Without the flag, prime behaves exactly as today (pure JSON, backward compatible).

With `--context`, output becomes:

```markdown
# Mull — Matter Tracking

## Landscape

<JSON data same as today>

## Workflow

- `mull show <id>` + `mull graph <id>` to load context
- `mull add "<title>" --status raw` to capture new ideas
- `mull append <id> "<text>"` for details as they emerge
- `mull set <id> <key> <value>` for metadata
- `mull link <id> <type> <id>` for relationships
- `mull docket` + `mull graph` to consult priorities

## Principles

- Capture as you go — don't wait until the end
- Match user's energy — a tickler is not a spec
- Don't push toward planning unless user signals execution intent
```

Silent exit (exit 0, no stderr) when `.mull/` doesn't exist and `--context` is set. This enables global hooks without errors in non-mull projects.

### 2. Safe exit before auto-init

**File:** `cmd/prime.go` (or `cmd/root.go` minimally)

Currently `PersistentPreRunE` in root.go auto-creates `.mull/` on any command. For `prime --context` to silently exit in non-mull repos, it needs to detect the absence *before* auto-init fires.

Approach: in prime's `RunE`, when `--context` is set, check for `.mull/` existence first. If missing, `os.Exit(0)` silently. This avoids changing root.go's auto-init for other commands.

### 3. Add `mull onboard` command

**File:** `cmd/onboard.go` (new)

Prints setup instructions. Two subcommands:

- `mull onboard hooks` — prints the JSON snippet for `~/.claude/settings.json`
- `mull onboard skill` — prints instructions for symlinking the skill file

No subcommand prints both options with explanation of the tradeoffs.

### 4. Skill stays as-is

`skills/SKILL.md` is unchanged. The `--context` output duplicates the same workflow knowledge in a hook-friendly format.

## Files

| File | Action |
|------|--------|
| `cmd/prime.go` | Add `--context` flag, silent exit logic, markdown wrapper |
| `cmd/onboard.go` | New — prints setup instructions for both integration paths |
| `cmd/root.go` | Minor tweak if prime needs to bypass auto-init |

## Verification

1. `mull prime` in a mull project — same JSON as before (no regression)
2. `mull prime --context` in a mull project — markdown with embedded JSON + workflow
3. `mull prime --context` outside a mull project — silent exit, exit code 0
4. `mull onboard` — prints both integration options
5. `mull onboard hooks` — prints hook JSON snippet
6. `mull onboard skill` — prints skill symlink instructions
7. `make test` passes
