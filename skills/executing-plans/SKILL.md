---
name: executing-plans
description: Use when you have an implementation plan to execute, either from a mull matter or provided directly
---

# Executing Plans

## Hard Constraints

These are not suggestions. They override your defaults. Violating any of them is a skill failure.

**1. Sub-agents are the default.**
Plans with more than 3 tasks: dispatch tasks to sub-agents using the Task tool. Do not work inline unless a task is trivial (< 10 lines changed). Do not set `isolation: "worktree"` — sub-agents work on the current branch directly.

**2. TDD is mandatory.**
Write the failing test FIRST. Then implement to make it pass. Never write implementation before the test. No exceptions. If a task has no testable behavior, say so explicitly and get confirmation before skipping the test.

**3. No worktrees unless the user says "worktree."**
Never set `isolation: "worktree"` on Task tool calls. Sub-agents work on the current branch. If you think you need isolation, ask the user.

**4. Commit only at the end.**
Do not commit after individual tasks or batches. Commit once during Step 5 (Finish), after full verification passes.

**5. Echo constraints before starting (Step 0).**
Before editing any code, state the constraints you are following and wait for the user to confirm. This is Step 0 of the process. You cannot skip it.

### Why these exist

Claude reads skills, understands them, and then falls back to defaults under momentum. These constraints exist because that pattern has happened repeatedly. The echo step (constraint 5) is a circuit breaker — it forces you to pause and re-commit to the rules before autopilot kicks in.

---

## Overview

Load the plan from a mull matter, review it critically, execute tasks using TDD in batches using sub-agents for complex steps or plans with many tasks, and report between batches for review. Ask the user if they want to use main or another branch.

**Core principle:** Batch execution with checkpoints. Stop when blocked, don't guess.

## Process

### Step 0: Echo Constraints

Before doing anything else, state the following to the user and wait for confirmation:

> **Constraints for this execution:**
> - Sub-agents for all non-trivial tasks (plans > 3 tasks)
> - TDD: failing test first, then implementation
> - No worktrees — sub-agents work on current branch
> - No commits until final verification
> - [any plan-specific constraints]
>
> **Confirm to proceed.**

Do not proceed to Step 1 until the user confirms.

### Step 1: Load and Review Plan

```bash
mull show <id> --md
```

Read the plan section. Review critically:
- Are tasks clear enough to execute?
- Are there gaps, ambiguities, or concerns?
- Do file paths and dependencies make sense?

**If concerns:** Raise them before starting. Don't proceed with a plan you don't understand.

**If no concerns:** Create task tracking items and proceed.

### Step 2: Execute Batch

**Default batch size: 3 tasks.** Adjust if the user prefers differently.

For each task:
1. Mark as in_progress
2. Implement using TDD — write failing test, make it pass, refactor
3. Run verifications as appropriate (test suite, linter)
4. Mark as completed

### Step 3: Report

When the batch is complete:
- Show what was implemented
- Show verification output (test results, any warnings)
- Say: **"Batch complete. Ready for feedback."**

Wait for the user. Don't start the next batch until they respond.

### Step 4: Continue

Based on feedback:
- Apply changes if needed
- Execute next batch
- Repeat until all tasks complete

### Step 5: Finish

After all tasks are done:

1. **Run full verification** — project test suite, linting, whatever the project uses
2. **Commit** any remaining uncommitted work
3. **Update the matter:**
   ```bash
   mull done <id>
   git add .mull/ && git commit -m "Complete <topic>"
   ```
4. **Report:** What was built, verification output, and status

That's it. If the user is on a feature branch and wants to merge or create a PR, they can invoke the finishing-a-development-branch skill.

## When to Stop and Ask

**Stop executing immediately when:**
- A task is unclear or ambiguous
- A test fails and you don't understand why
- You hit a missing dependency or unexpected state
- The plan has gaps that prevent continuing
- Verification fails repeatedly

**Ask for clarification rather than guessing.** Don't force through blockers.

## When to Revisit the Plan

**Return to review when:**
- User updates the plan based on feedback
- You discover the approach needs rethinking mid-execution
- A task reveals that later tasks need to change

Don't soldier on with a plan that's no longer accurate.

## Principles

- Review the plan critically before starting
- Follow plan tasks in order unless dependencies allow reordering
- TDD for all implementation — the plan names the tests, you red-green-refactor
- Between batches: report and wait — don't assume approval
- Stop when blocked, don't guess
- Never start implementation on main/master without explicit user consent
