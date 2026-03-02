---
status: done
tags: [design]
created: 2026-03-02
updated: 2026-03-02
---

# Git workflow guardrails

# Git Workflow Guardrails

## Problem

Agents ignore CLAUDE.md git rules under pressure — push to main, force push, amend pushed commits, `git add .`. CLAUDE.md rules fade from context. Need mechanical enforcement for the dangerous stuff and skill-level reinforcement for the rest.

## Two Workflows

**1. Main (default, most work):** Commit on main, agent never pushes. Bug fixes, small features, planning.

**2. Branch (user-initiated):** Agent creates branch, can push to it. User merges when ready via finishing-a-development-branch skill.

## Deliverables

### 1. Pre-push hook (infrastructure)

Install `.git/hooks/pre-push` in kit repo (and document for other repos):
- Block pushes to main/master when `$CLAUDECODE` is set
- Block force pushes from anyone (detect history rewriting via merge-base ancestry check)
- Mechanical enforcement — no rationalization possible

### 2. Update `executing-plans` skill

Add branch creation workflow for when user chooses branch mode:
- `git checkout -b <branch-name>` at start of work
- Agent can push to this branch
- Reference finishing-a-development-branch for integration

Keep it light — a few lines, not a new section.

### 3. Update `finishing-a-development-branch` skill

- Confirm agent can push feature branches (option 2)
- Add explicit "never push to main" in Red Flags
- Add "never force push" and "never amend pushed commits" in Red Flags

### 4. Audit CLAUDE.md git rules

- Keep rules as documentation
- Note which ones are now mechanically enforced by the hook
- Don't rely on CLAUDE.md alone for critical rules

## Design Constraints

- No new skill — rules live in existing skills where actions happen
- Pre-push hook is infrastructure, not a skill
- Minimal ceremony — agents shouldn't burn context on git
- User decides main vs branch, not the agent
