---
name: requesting-code-review
description: Use when completing tasks, implementing major features, or before merging to verify work meets requirements
---

# Requesting Code Review

Dispatch a code-reviewer subagent to catch issues before they cascade.

**Core principle:** Review early, review often.

## When to Request Review

**Mandatory:**
- After completing a major feature
- Before merge to main

**Optional but valuable:**
- When stuck (fresh perspective)
- Before refactoring (baseline check)
- After fixing complex bug

## How to Request

**1. Get git SHAs:**
```bash
BASE_SHA=$(git rev-parse HEAD~1)  # or origin/main
HEAD_SHA=$(git rev-parse HEAD)
```

**2. Dispatch code-reviewer subagent:**

Use Task tool with the code-reviewer template at `code-reviewer.md` in this directory.

**Placeholders:**
- `{WHAT_WAS_IMPLEMENTED}` — What you just built
- `{PLAN_OR_REQUIREMENTS}` — What it should do
- `{BASE_SHA}` — Starting commit
- `{HEAD_SHA}` — Ending commit
- `{DESCRIPTION}` — Brief summary

**3. Act on feedback:**
- Fix Critical issues immediately
- Fix Important issues before proceeding
- Note Minor issues for later
- Push back if reviewer is wrong (with reasoning)

## Example

```
[Just completed: Add the purge command]

BASE_SHA=$(git log --oneline -10 | grep "before purge" | head -1 | awk '{print $1}')
HEAD_SHA=$(git rev-parse HEAD)

[Dispatch code-reviewer subagent]
  WHAT_WAS_IMPLEMENTED: Purge command for batch-deleting old terminal matters
  PLAN_OR_REQUIREMENTS: Task 3 from matter #42
  BASE_SHA: a7981ec
  HEAD_SHA: 3df7661
  DESCRIPTION: Added purge subcommand with age filter and dry-run mode

[Subagent returns]:
  Strengths: Clean tests, good error handling
  Issues:
    Important: Missing confirmation prompt for non-dry-run
    Minor: Could log count of deleted items
  Assessment: Ready with fixes

[Fix confirmation prompt]
[Continue]
```

## Red Flags

**Never:**
- Skip review because "it's simple"
- Ignore Critical issues
- Proceed with unfixed Important issues
- Argue with valid technical feedback

**If reviewer wrong:**
- Push back with technical reasoning
- Show code/tests that prove it works
- Request clarification

See template at: `requesting-code-review/code-reviewer.md`
