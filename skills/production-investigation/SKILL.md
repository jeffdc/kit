---
name: production-investigation
description: Use when investigating production outages, crashes, unexpected behavior, or degraded performance — before proposing any fixes or mitigations
---

# Production Investigation

## Hard Constraints

These are not suggestions. They override your defaults. Violating any of them is a skill failure.

**1. Data before theories.**
Collect ALL evidence from the data collection checklist before forming any hypothesis. The urge to theorize after seeing one metric is the exact impulse this skill exists to override.

**2. Verify visual data with the user.**
When reading charts, dashboards, or screenshots, state what you see and ask the user to confirm. Never build a theory on an unconfirmed chart reading. Read tooltip values, not shapes.

**3. Verify claims against actual data.**
Before presenting any finding (yours or a subagent's), verify its key assumptions with a concrete query. One `SELECT COUNT(*)` kills more bad theories than ten paragraphs of reasoning. Subagents speculate convincingly — verify every quantitative claim.

**4. Confirm code paths were executed.**
Before analyzing whether a code path could cause the incident, confirm it was actually called. Check request logs for triggering routes, methods, and user agents. "Could cause" is not "did cause."

**5. Measure blast radius with numbers, not impressions.**
Never call a change "trivial" or "massive" without stating the actual scope. "88 files, 15k insertions" is a fact. "Trivial" is an opinion that closes your mind. Check ALL recent deploys by actual scope, not just the most recent.

**6. Echo constraints before starting.**
Before investigating, state: "Investigation mode: data before theories, verify charts with user, confirm code paths were executed, no fixes until root cause identified." This is not optional.

**7. Maintain the investigation doc.**
Create the investigation doc (Phase 0) and update it as you go. Every finding, timeline entry, and ruling gets written to the doc immediately — not reconstructed at the end.

---

## Overview

Investigate production incidents by collecting evidence, building a timeline, and systematically ruling out causes — before proposing fixes.

**Core principle:** The evidence tells you what happened. Your job is to listen, not to narrate.

## When to Use

- Production outage or crash
- Unexplained restarts or process deaths
- Degraded performance or elevated error rates
- Unexpected behavior in production that doesn't reproduce locally
- Post-incident review of a resolved outage

## Phase 0: Prior Investigations & Working Doc

### Check for prior investigations

Before collecting new data, check `docs/investigations/` for previous incident reports. Look for similar symptoms, same service/machine, or recurring patterns. Prior investigations may have already ruled out causes or deployed partial fixes — don't repeat that work.

### Create the investigation doc

Create `docs/investigations/<YYYYMMDD>-<slug>.md` immediately using this template:

```markdown
# P<severity>: <Title> - <Date>

## Status: Open — investigating

## Summary
<Fill in as investigation progresses>

## Impact
- **Duration**: TBD
- **Scope**: TBD

## Machine Info

## Timeline (all <timezone>)
| Time | Event |
|------|-------|

## What We Ruled Out

## Root Cause Analysis

## Actions Taken

## What We Still Don't Know

## Related
```

## Phase 1: Data Collection

Gather ALL of these before analyzing anything. Missing data leads to bad theories.

| Source | What to get | What it tells you |
|--------|-------------|-------------------|
| Machine events | Process/container event log | Crash timestamps, exit codes, restart history |
| Recent deploys | Deploy/release history | What changed and when |
| Deploy diffs | `git diff` between release commits | Actual scope of changes (file count, line count) |
| Request logs | Download and analyze with scripts | Traffic patterns, gaps, errors, slow requests |
| Application metrics | Dashboards with tooltip values | Memory, CPU, connections |
| Platform status | Vendor status page | Known incidents affecting your infrastructure |
| Error logs | Application log stream or files | Application errors, warnings, stack traces |

Download logs for analysis — streaming logs are for watching, not investigating.

### Platform-Specific Commands (Fly.io)

- `fly machine status <id>` — event log with exit codes
- `fly releases --json` — deploy history
- `fly ssh sftp get <log-path>` — download request logs (don't use `fly logs`)
- Grafana dashboards — memory, CPU, CPU quota
- status.flyio.net — platform incidents

Adapt to your platform. The checklist sources are universal; the commands vary.

## Phase 2: Timeline Construction

Build a timeline from hard evidence only. No interpretation yet.

**For each event, record:**
- Exact timestamp (with timezone)
- Source of the timestamp (which log, which metric)
- What happened (factual, not interpreted)

**The timeline must answer:**
1. When did the incident start? (last normal data point)
2. When did it end? (first recovery data point)
3. What was the gap? (duration with zero data)
4. What events occurred during the gap? (machine events, deploys, etc.)

## Phase 3: Ruling Out Causes

Work through each category systematically. For each one, state whether it's **ruled out**, **plausible**, or **confirmed** — with evidence.

| Category | Ruled out when... |
|----------|-------------------|
| Memory | Metrics show stable usage well below limits, no OOM flag |
| CPU / throttling | Utilization below baseline, credits healthy |
| Traffic spike | Request volume normal or below historical baseline |
| Code change | No relevant code path was executed, or change is provably unrelated |
| Write contention | No write operations occurred during the incident window |
| Platform issue | Vendor confirms no incidents (check, but don't fully trust) |
| External dependency | No calls to external services timed out or failed |

**"I didn't find it" is not "it's ruled out."** State which evidence rules it out, or say it remains plausible.

## Phase 4: Honest Assessment

After completing Phase 3, categorize your conclusion:

- **Root cause confirmed:** Evidence shows exactly what happened and why.
- **Root cause probable:** Evidence strongly suggests a cause but can't be confirmed. State what additional data would confirm it.
- **Root cause undetermined:** All observable causes ruled out. State what remains below your observability layer and recommend next steps.

**Do not stretch a "probable" into a "confirmed."** "I don't know" with a clear list of what you ruled out is more valuable than a wrong diagnosis.

## Phase 5: Mitigations

Only after completing Phases 1-4. Label every action explicitly:

| Label | Meaning | Example |
|-------|---------|---------|
| **Mitigation** | Reduces impact regardless of cause | Health watchdog, faster restarts |
| **Fix** | Addresses the specific root cause | Fixing a memory leak that caused OOM |
| **Hygiene** | Improves code quality, unrelated to incident | Fixing N+1 query found during investigation |
| **Unnecessary** | Based on a disproven theory | CPU bump based on misread chart |

## Red Flags — STOP and Check Yourself

| Red flag | What to do instead |
|----------|-------------------|
| Theorizing after seeing one metric | Finish the data collection checklist first |
| "The chart shows..." (no tooltip values) | Read actual values, ask user to confirm |
| "This is a trivial/massive change" | State file count and line count |
| "This could cause..." (no data check) | Run the query, check actual cardinality |
| "This function is dangerous" (didn't check if it ran) | Check request logs for the triggering route |
| Proposing a fix before completing Phase 3 | Finish ruling out causes first |
| Building on a subagent's unverified finding | Verify the key assumption with one query |
| Saying "confirmed" when you mean "plausible" | Use the honest assessment categories |
| All app metrics are clean but still searching app-level | Say "undetermined" and recommend next steps |
| "Just try redeploying / bumping resources" | That's a guess, not a diagnosis — label it as mitigation |
