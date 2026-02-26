# Skills

Custom Claude Code skills — reference guides that teach Claude proven techniques, patterns, and workflows. Forked from [superpowers](https://github.com/anthropics/claude-plugins-official) v4.3.1 and adapted for my workflow.

Skills are loaded automatically based on context. When you start debugging, Claude loads the debugging skill; when you finish a feature, it loads the code review skill.

## Install

```bash
./install.sh
```

This does two things:

1. **Symlinks each skill** into `~/.claude/skills/` where Claude Code discovers them
2. **Adds a SessionStart hook** to `~/.claude/settings.json` that injects the `using-writs` bootstrapper into every conversation, so Claude checks for applicable skills automatically

Re-running is safe — skips existing correct symlinks, warns on conflicts.

Also disable the superpowers plugin in `~/.claude/settings.json` (set `superpowers@claude-plugins-official` to `false`) if you haven't already.

### Requirements

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
- `jq` (for hook installation)

## Skills

| Skill | When it fires |
|-------|--------------|
| **using-writs** | Every conversation (via SessionStart hook). Bootstraps skill awareness. |
| **brainstorming** | Before creative work — features, components, design decisions |
| **writing-plans** | When you have requirements for a multi-step task, before coding |
| **executing-plans** | When you have a plan to execute |
| **test-driven-development** | Before writing implementation code for any feature or bugfix |
| **systematic-debugging** | When hitting a bug, test failure, or unexpected behavior |
| **requesting-code-review** | After completing features or before merging |
| **receiving-code-review** | When acting on review feedback |
| **dispatching-parallel-agents** | When facing 2+ independent tasks |
| **using-git-worktrees** | When you need isolated workspaces (must be explicitly invoked) |
| **finishing-a-development-branch** | When implementation is done and you need to integrate |
| **verification-before-completion** | Before claiming work is complete |
| **user-profile** | Sets up `~/.claude/user-profile.md` so skills calibrate tone to your experience |
| **writing-skills** | When creating or editing skills |

## How it works

Skills are Markdown files that Claude Code loads via the `Skill` tool. The `using-writs` skill acts as a bootstrapper — injected at session start via a hook, it tells Claude to check for applicable skills before responding to any message. From there, skills chain naturally: starting a feature triggers brainstorming, which leads to planning, which leads to TDD, which leads to code review.

The `user-profile` skill creates a global profile at `~/.claude/user-profile.md` that other skills cross-reference against the current project's stack. This lets Claude calibrate tone automatically — peer review when you're strong in the stack, teaching mode when you're learning it.

## Mull integration

The workflow skills (brainstorming, writing-plans, executing-plans) integrate with [mull](../mull/), the idea and feature tracker in this repo. Key patterns:

- Designs and plans are written into mull matter bodies
- Matters are committed to git
- `mull dock <id>` when moving to implementation
- `mull done <id>` when work is complete

## Structure

```
skills/
  install.sh                          # Symlinks skills + installs hook
  hooks/session-start.sh              # Injects using-writs at startup
  <skill-name>/
    SKILL.md                          # Main skill file (required)
    supporting-file.*                 # Additional files (optional)
```
