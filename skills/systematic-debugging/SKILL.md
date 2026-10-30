---
name: systematic-debugging
description: Use when encountering any bug, test failure, or unexpected behavior, before proposing fixes
---

# Systematic Debugging

Find the root cause before fixing anything. Symptom fixes waste time and mask the real problem. This applies to simple bugs and emergencies too: guess-and-check is slower than investigating.

## Rules

1. **No fixes before root cause.** Don't propose, attempt or "just try" a fix until Phase 1 is done.
2. **One hypothesis at a time.** Change one thing per attempt; if you change two and it works, you don't know which one fixed it.
3. **Three strikes means stop.** If three fixes have failed, don't try a fourth. Stop and question the design with the user.

## Phase 1: Root cause

1. **Read the error completely:** the full stack trace, line numbers, codes. It often names the answer.
2. **Reproduce it reliably.** If you can't, gather more data rather than guessing.
3. **Check what changed:** git diff, recent commits, dependencies, config, environment.
4. **In multi-component systems** (API → service → database, CI → build → deploy), instrument each boundary: log what enters and leaves each component and check config propagates. Run once to see *where* it breaks, then investigate that component.
5. **Trace bad values backward** to where they originate and fix there, not where they surface. See `root-cause-tracing.md`.

## Phase 2: Compare

- Find similar code in the codebase that works, and list every difference from the broken code, however small.
- When following a reference implementation, read all of it before adapting it.
- Check what the broken code depends on: other components, settings, environment, assumptions.

## Phase 3: Hypothesis

- State it: "I think X is the root cause because Y."
- Test it with the smallest possible change.
- If it's wrong, form a new hypothesis; don't pile more fixes on top.
- If you don't understand something, say so and research or ask.

## Phase 4: Fix

1. Write a failing test that reproduces the bug.
2. Make one change that addresses the root cause. No "while I'm here" changes.
3. Confirm the test passes, nothing else broke, and the original symptom is gone.
4. If the fix doesn't work, go back to Phase 1 with what you learned. After three failed fixes, stop. Each fix revealing a new problem somewhere else is a sign of a design problem; discuss with the user before trying more.

## Supporting techniques (in this directory)

- `root-cause-tracing.md`: trace bugs backward through the call stack to the original trigger
- `defense-in-depth.md`: add validation at several layers once the root cause is found
- `condition-based-waiting.md`: replace arbitrary timeouts with condition polling
