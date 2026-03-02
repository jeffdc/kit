---
name: verification-before-completion
description: Use when about to claim work is complete, fixed, or passing, before committing or creating PRs - requires running verification commands and confirming output before making any success claims; evidence before assertions always
---

# Verification Before Completion

## Overview

Claiming work is complete without verification is dishonesty, not efficiency.

**Core principle:** Evidence before claims, always.

**Violating the letter of this rule is violating the spirit of this rule.**

## The Iron Law

```
NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE
```

If you haven't run the verification command in this message, you cannot claim it passes.

## The Gate Function

```
BEFORE claiming any status or expressing satisfaction:

1. IDENTIFY: What command proves this claim?
2. RUN: Execute the FULL command (fresh, complete)
3. READ: Full output, check exit code, count failures
4. VERIFY: Does output confirm the claim?
   - If NO: State actual status with evidence
   - If YES: State claim WITH evidence
5. ONLY THEN: Make the claim

Skip any step = lying, not verifying
```

## Common Failures

| Claim | Requires | Not Sufficient |
|-------|----------|----------------|
| Tests pass | Test command output: 0 failures | Previous run, "should pass" |
| Linter clean | Linter output: 0 errors | Partial check, extrapolation |
| Build succeeds | Build command: exit 0 | Linter passing, logs look good |
| Bug fixed | Test original symptom: passes | Code changed, assumed fixed |
| Regression test works | Red-green cycle verified | Test passes once |
| Agent completed | VCS diff shows changes | Agent reports "success" |
| Requirements met | Line-by-line checklist | Tests passing |

## Red Flags - STOP

- Using "should", "probably", "seems to"
- Expressing satisfaction before verification ("Great!", "Perfect!", "Done!", etc.)
- About to commit/push/PR without verification
- Trusting agent success reports
- Relying on partial verification
- Thinking "just this once"
- **ANY wording implying success without having run verification**
- About to claim completion without running surface audit
- Concluding "nothing affected" without checking git diff

## Rationalization Prevention

| Excuse | Reality |
|--------|---------|
| "Should work now" | RUN the verification |
| "I'm confident" | Confidence ≠ evidence |
| "Just this once" | No exceptions |
| "Linter passed" | Linter ≠ compiler |
| "Agent said success" | Verify independently |
| "Partial check is enough" | Partial proves nothing |
| "Different words so rule doesn't apply" | Spirit over letter |
| "No surfaces affected" | Run the diff, check the map |
| "Too small to drift" | All changes go through the check |

## Key Patterns

**Tests:**
```
RUN test command -> SEE 34/34 pass -> "All tests pass"
NOT "Should pass now" / "Looks correct"
```

**Regression tests (TDD Red-Green):**
```
Write -> Run (pass) -> Revert fix -> Run (MUST FAIL) -> Restore -> Run (pass)
NOT "I've written a regression test" (without red-green verification)
```

**Build:**
```
RUN build -> SEE exit 0 -> "Build passes"
NOT "Linter passed" (linter doesn't check compilation)
```

**Requirements:**
```
Re-read plan -> Create checklist -> Verify each -> Report gaps or completion
NOT "Tests pass, phase complete"
```

**Agent delegation:**
```
Agent reports success -> Check VCS diff -> Verify changes -> Report actual state
NOT Trust agent report
```

## Surface Audit

After tests/build pass, before claiming completion: check whether all parallel representations of changed data were updated.

**The procedure:**

```
1. DETECT: git diff --name-only against base branch (full session, not just last commit)
2. MAP: For each changed file, identify parallel surfaces using the surface map
3. CHECK: Which mapped surfaces are NOT in the diff?
4. ASK: For each gap, ask the user — don't skip, don't auto-fix
```

The user decides what's in scope. You present evidence and ask.

**Surface map:** See `surface-map.md` in this directory for default mapping rules, example commands, and project-specific configuration format (a `## Surface Map` section in the project's CLAUDE.md).

**Anti-patterns — these are skill failures:**

| Anti-pattern | Why it fails |
|---|---|
| Mentally conclude "nothing affected" without running diff | Self-certification, not evidence |
| Check only the latest commit, miss earlier session commits | Drift accumulates across the session |
| Identify a gap and silently fix it | User decides scope, not agent |
| Skip surface check because changes seem "small" | All changes go through the check |

**Ambiguous cases:** If it's unclear whether a surface uses the changed code, ask rather than skip. False positives beat silent drift.

## When To Apply

**ALWAYS before:**
- ANY variation of success/completion claims
- ANY expression of satisfaction
- ANY positive statement about work state
- Committing, PR creation, task completion
- Moving to next task
- Delegating to agents

## The Bottom Line

**No shortcuts for verification.**

Run the command. Read the output. THEN claim the result.

This is non-negotiable.
