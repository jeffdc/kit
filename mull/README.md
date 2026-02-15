# mull

Track ideas and features for solo projects. Mull is a CLI tool that stores matters (ideas, features, tasks) as markdown files alongside your code, designed to work naturally with AI coding assistants.

## Why

Solo projects accumulate ideas faster than you can act on them. You need somewhere to put "dark mode would be nice" that isn't a sticky note, a GitHub issue (too heavy), or a comment in code (lost forever).

Mull gives you:

- **Markdown files with YAML frontmatter** in a `.mull/` directory, version controlled with your project
- **A prioritized docket** so you know what to work on next
- **Dependency tracking** so you know what's blocked
- **JSON output everywhere** so AI assistants can read and write matters fluently

The primary interface is conversational -- you talk about ideas with your AI assistant and it captures them as you go. But every command works fine from the terminal too.

## Install

Requires Go 1.21+.

```bash
git clone <repo-url>
cd mull
make install
```

This puts the `mull` binary in your `$GOBIN` (usually `~/go/bin`).

## Quick start

```bash
# Capture an idea
mull add "Add RSS feed" --tag content --effort small --epic v2-launch

# See what you've got
mull list

# Flesh it out
mull append ab3f "Should support Atom format. Auto-generate from post metadata."
mull set ab3f status refined

# Link related work
mull link ab3f needs c7d1

# Prioritize
mull docket add ab3f
mull docket add c7d1 --after ab3f

# What should I work on?
mull docket
mull graph

# Close it when done
mull done ab3f
```

## Claude Code integration

Mull integrates with Claude Code two ways. Pick one.

### Option 1: Hooks (recommended)

Mull's workflow context is injected automatically at session start. No manual invocation needed.

```bash
mull onboard hooks --install
```

This adds hooks to `~/.claude/settings.json` that run `mull prime --context` on `SessionStart` and `PreCompact`. The hooks exit silently in projects without a `.mull/` directory, so they're safe to install globally.

To remove:

```bash
mull onboard hooks --uninstall
```

### Option 2: Skill

A Claude Code skill you invoke on demand with `/mull`.

```bash
# From the mull source directory
ln -s "$(pwd)/skills/SKILL.md" ~/.claude/skills/mull.md
```

Run `mull onboard` to see both options with details.

## Commands

| Command | What it does |
|---------|-------------|
| `mull add "<title>"` | Create a matter (`--tag`, `--status`, `--effort`, `--epic`) |
| `mull show <id>` | View a matter with full body |
| `mull list` | List active matters (`--status`, `--tag`, `--effort`, `--epic`, `--all`) |
| `mull search <query>` | Full-text search across titles and bodies |
| `mull set <id> <key> <value>` | Update metadata |
| `mull append <id> "<text>"` | Add to the body |
| `mull link <id> <type> <id>` | Add relationship (relates, blocks, needs, parent) |
| `mull unlink <id> <type> <id>` | Remove relationship |
| `mull done <id>` | Mark as done (removes from docket) |
| `mull drop <id>` | Mark as dropped (removes from docket) |
| `mull rm <id>` | Permanently delete (use sparingly) |
| `mull docket` | View the prioritized work queue |
| `mull docket --invert` | View matters NOT on the docket |
| `mull docket add <id>` | Add to docket (`--after <id>` to position) |
| `mull docket rm <id>` | Remove from docket |
| `mull docket move <id>` | Reorder (`--after <id>`) |
| `mull epics` | List all epics with matter counts |
| `mull graph [id]` | Dependency graph (all or centered on one matter) |
| `mull doctor` | Check data integrity (`--fix` to repair) |
| `mull prime` | Token-efficient JSON snapshot for LLM context |
| `mull prime --context` | Snapshot wrapped with workflow instructions (for hooks) |
| `mull onboard` | Setup instructions for Claude Code integration |

All commands output JSON to stdout. Errors go to stderr as `{"error": "message"}`.

`list`, `docket --invert`, and `epics` exclude done/dropped matters by default. Use `--all` to include them.

## Data layout

```
your-project/
  .mull/
    matters/
      ab3f-add-rss-feed.md
      c7d1-dark-mode.md
    docket.yml
```

Each matter is a markdown file:

```markdown
---
status: refined
tags: [content, low-effort]
effort: small
epic: v2-launch
created: 2026-02-13
updated: 2026-02-14
needs: [c7d1]
---

# Add RSS feed

Let people subscribe to new posts. Should support Atom format.
Auto-generate from post metadata.
```

## Matter lifecycle

```
raw --> refined --> planned --> done
                       \
                        --> dropped
```

- **raw** -- just captured
- **refined** -- fleshed out, clear enough to act on
- **planned** -- has a plan, ready for implementation
- **done** -- shipped
- **dropped** -- decided against

## Relationships

Four types, all managed with `mull link` / `mull unlink`:

- **relates** -- loose association (bidirectional)
- **blocks / needs** -- dependency (bidirectional inverse: A blocks B means B needs A)
- **parent** -- grouping (one-way)

Bidirectional links are kept in sync atomically. If writing one side fails, the other is rolled back.

## Epics

Epics are lightweight labels for grouping related matters by theme. Set with `--epic` on create or `mull set <id> epic <name>` later. No extra files or lifecycle -- an epic is just a string.

```bash
mull add "Dark mode" --epic ui-overhaul
mull epics                    # list all epics with counts
mull list --epic ui-overhaul  # filter by epic
```

## Closing vs deleting

- `mull done <id>` -- marks as done, keeps the file for reference. This is almost always what you want.
- `mull drop <id>` -- decided against, keeps the file for reference.
- `mull rm <id>` -- permanent delete. Only for junk or mistakes.

Both `done` and `drop` automatically remove the matter from the docket. Done/dropped matters are hidden from `list`, `docket --invert`, `epics`, and `prime` by default.
