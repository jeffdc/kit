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

Dispatch a code-reviewer subagent using the Task tool with the template at `code-reviewer.md` in this directory. The subagent handles the interactive flow — scope selection, git range, review, and output formatting.

Pass any context you have (what was implemented, requirements) but don't worry about git SHAs — the subagent figures those out.

## Acting on Feedback

- Fix **Critical** issues immediately
- Fix **Important** issues before proceeding
- Note **Nitpick** issues for later
- Push back if reviewer is wrong (with reasoning)

## Red Flags

**Never:**
- Skip review because "it's simple"
- Ignore Critical issues
- Proceed with unfixed Important issues

**If reviewer wrong:**
- Push back with technical reasoning
- Show code/tests that prove it works
- Request clarification

See template at: `requesting-code-review/code-reviewer.md`
