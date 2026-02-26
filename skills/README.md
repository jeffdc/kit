# Writs

Custom Claude Code skills, forked from [superpowers](https://github.com/anthropics/claude-plugins-official) v4.3.1 and adapted for my workflow.

## What changed from superpowers

**Core workflow skills (heavy rewrites):**
- **brainstorming** — Design goes into mull matter instead of `docs/plans/` files. Ceremony scales to scope. No forced chain to writing-plans.
- **writing-plans** — Plan lives in mull matter. Tasks are meaningful units of work, not 2-5 minute micro-steps. TDD is assumed, not spelled out step-by-step.
- **executing-plans** — Reads plan from mull matter. No worktree requirement. Finishes with verification and `mull done`, no chain to another skill.
- **finishing-a-development-branch** — Solo-developer options (merge/PR/keep). No worktree cleanup. No auto-deletion.

**TDD (moderate edits):**
- **test-driven-development** — Added Elixir/ExUnit examples. Phoenix-specific guidance (context first, LiveView second). Project commands: `mix test`, `go test`.

**Dispatcher:**
- **using-writs** — Replaces `using-superpowers`. References skills by plain name. No worktree or subagent-driven-development references.

**Light-touch forks (namespace cleanup, removed superpowers cross-refs):**
- systematic-debugging (+ supporting files)
- dispatching-parallel-agents
- verification-before-completion
- receiving-code-review
- requesting-code-review
- writing-skills

**Standalone opt-in:**
- **using-git-worktrees** — Available when needed, but no skill invokes it automatically.

**Dropped entirely:**
- subagent-driven-development (executing-plans covers the use case)

## Install

```bash
./install.sh
```

Creates symlinks from `~/.claude/skills/<name>` to the skill directories here. Safe to re-run — skips existing correct symlinks, warns on conflicts.

Also disable the superpowers plugin in `~/.claude/settings.json` (set `superpowers@claude-plugins-official` to `false`).

## Mull integration

The workflow skills (brainstorming, writing-plans, executing-plans) integrate with [mull](../mull/), the idea and feature tracker in this repo. Key patterns:

- Designs and plans are written into mull matter bodies
- Matters are committed to git
- `mull dock <id>` when moving to implementation
- `mull done <id>` when work is complete
