---
item: ed9d
"n": 1
passed: true
commit: 8b5e77ae0ded5bb4a8122fde20ef3eb961f8554e
base: 516582f0e9f2e284084d857f21d390150537aed7
at: "2026-11-22T16:33:50Z"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a7c6b593278524845
summary: 'A plan can now have a target branch (mull plan set SLUG target BRANCH, or mull set; it falls back to config integration). Seen-in-history is now measured per item against its plan''s target: trailers and id-branch merges count only on that branch, unmerged work is commits on no target, and a new merged_at on an item''s history is derived from the target''s first-parent commit that brought the work in. view (JSON and --text), context and mull serve (plan sheet, the overview for plans with their own target, the item page) show each plan''s target and how far it is ahead of release. done''s fallback base uses the plan''s target. design.md, the json, intake and supervisor guides, and serve''s DESIGN.md are updated; the demo''s web plan now targets web-next; the bench seed takes -targets.'
tour:
  - 'Run make dev-reset (the demo needs reseeding: its web plan now targets a web-next branch), then open http://localhost:6899/#/demo'
  - 'In the Plans column, Web app has a Target row: web-next, with a muted ''2 commits ahead of main''. Pilot (shaping, no target of its own) has no Target row.'
  - 'Open Web app: the title block has a Target field reading web-next, ''2 commits ahead of main''.'
  - 'Open the item Digest job (in review): its commits field reads ''2 commits on web-next'' (the commit and its merge), with ''merged N ago'' muted beside it.'
  - 'Open the item Mailer (Web app, no commits): the same field reads ''no commits yet'', with ''lands on web-next'' muted.'
  - 'Open Fix the login redirect under Unplanned: the same field reads ''no commits yet'', with ''lands on main'' muted.'
checks:
  tree: 8034b6595e37e36ba4a891ac390ba2ac571885b0
  commit: 8b5e77ae0ded5bb4a8122fde20ef3eb961f8554e
  at: "2026-11-22T16:33:41Z"
  by: /Users/jeff/dev/kit/.claude/worktrees/agent-a7c6b593278524845
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 8179
changes:
  - layer: other
    files:
      - status: M
        path: mull/cmd/commands.go
        added: 15
        removed: 2
      - status: M
        path: mull/docs/design.md
        added: 16
        removed: 7
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 2
        removed: 2
      - status: M
        path: mull/internal/bench/seed.go
        added: 16
        removed: 1
      - status: M
        path: mull/internal/guide/intake.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/guide/json.md
        added: 9
        removed: 6
      - status: M
        path: mull/internal/guide/supervisor.md
        added: 1
        removed: 1
      - status: M
        path: mull/internal/model/model.go
        added: 3
        removed: 0
      - status: M
        path: mull/internal/mull/cache.go
        added: 4
        removed: 0
      - status: M
        path: mull/internal/mull/context.go
        added: 8
        removed: 2
      - status: M
        path: mull/internal/mull/done.go
        added: 7
        removed: 8
      - status: M
        path: mull/internal/mull/links.go
        added: 212
        removed: 51
      - status: M
        path: mull/internal/mull/ops.go
        added: 17
        removed: 0
      - status: M
        path: mull/internal/mull/pages.go
        added: 1
        removed: 3
      - status: M
        path: mull/internal/mull/revision.go
        added: 1
        removed: 0
      - status: M
        path: mull/internal/mull/schedule.go
        added: 2
        removed: 1
      - status: M
        path: mull/internal/mull/summary.go
        added: 1
        removed: 4
      - status: M
        path: mull/internal/mull/view.go
        added: 24
        removed: 6
      - status: M
        path: mull/internal/mull/work.go
        added: 26
        removed: 2
      - status: M
        path: mull/internal/serve/web/item.js
        added: 11
        removed: 4
      - status: M
        path: mull/internal/serve/web/plan.js
        added: 2
        removed: 1
      - status: M
        path: mull/internal/serve/web/plans.js
        added: 5
        removed: 2
      - status: M
        path: mull/internal/serve/web/title.js
        added: 12
        removed: 0
      - status: M
        path: mull/scripts/bench/main.go
        added: 3
        removed: 1
      - status: M
        path: mull/scripts/dev-demo.sh
        added: 9
        removed: 0
  - layer: tests
    files:
      - status: M
        path: mull/cmd/shapes_test.go
        added: 12
        removed: 3
      - status: M
        path: mull/internal/bench/bench_test.go
        added: 31
        removed: 0
      - status: A
        path: mull/internal/mull/target_test.go
        added: 203
        removed: 0
design: |-
  ## Approach

  A plan gets one new stored field, `target` (YAML `target:`, omitted when empty). Everything else is derived on read: which branch each item is measured against, whether it has landed, when it was merged, and how far each target is ahead of release. Nothing about landing is stored.

  ## Shape of the code

  - `internal/mull/links.go` holds it all. `state.integration()` (config's, else main) and `state.targetOf(slug)` (the plan's target, else integration, with where it came from) are the only places that decide a branch. `readTargets` reads every distinct target's tip (one `for-each-ref` already done by the caller) and its ahead-of-release count (cached `rev-list --count`). `state.target(slug, targets)` gives the public `Target {branch, from, found, ahead_of_release}` used by Board, PlanStatus and context's BriefPlan.
  - `readGit` now loops over the target branches instead of reading integration once. For each target it reads the history from its tip and credits a trailer or an id-branch merge only to items whose plan targets that branch. Unlinked commits are still collected only on integration. Unmerged work is read per non-target branch with `git log <branch> --not <every target tip>`, so a branch merged into its target and left behind doesn't count, and the git-process count per branch is unchanged.
  - `merged_at` comes from `broughtIn`: walk the target's first-parent line, then oldest first, claim each first-parent commit and everything its other parents reach that isn't claimed yet. The claimed set stays closed under parents, so each walk stops at older history, and the whole pass is linear in the commits read. An item's `merged_at` is the latest of those dates over its linked commits.
  - Each target's history (commits, by-ID index, brought-in dates) is cached per tip in a small LRU (`targetHistories`, 16), like `gitLogs`. The first measurement without it showed the large board up from 33.8 to 46.7 ms from rebuilding the index on every read; with it, it's back at 34.5.
  - `plan set` is `set` limited to plans (a new op, so batch matches the command), and `set` learned `target`. `checkTarget` uses `git check-ref-format --branch`, trims `refs/heads/`, and refuses `mull`.
  - `done`'s fallback base (no claim base) is now the merge base with the item's plan's target.
  - Callers that looked up `g.Items[id]` with a nil fallback now use `state.gitOf`, so an item with no commits still reports the target it will be measured against.

  ## UI

  `targetField` in title.js renders a target (mono branch, muted note: the integration branch / not found in git / N commits ahead of / level with release). The plan sheet's title block always shows it; the overview's Plans column shows a Target row only for a plan with its own target, since the project title block already shows integration. The item page's Git field now says "N commits on <target> · merged 2h ago", "not on <target> yet · N unmerged", or "no commits yet, lands on <target>". The old field said "no commits yet" even when there were unmerged commits; that's fixed in passing, since the field had to change anyway.

  ## Bench (make bench, in-process medians, ms)

  | page | before | after (default large) | large, 2 plans with own targets |
  |---|---|---|---|
  | board (mull view) | 33.8 | 34.5 | 37.2 |
  | overview | 40.1 | 41.5 | 45.3 |
  | plan metrics-23 | 26.3 | 28.8 | 32.2 |
  | item c65f | 25.5 | 25.4 | 27.7 |
  | projects (summary) | 23.2 | 23.8 | 24.4 |

  Git process counts per call are unchanged on every page (a target's log is cached by tip, like integration's). Runs vary by about 10% (pages this change doesn't touch, like tree and backlog, moved as much). The targets column is a repo seeded with the new `bench seed -targets 2`; the named sizes keep 0 targets, so the default seed is unchanged for anyone comparing numbers.

  ## Rejected

  - Measuring unmerged per item against its own target only: that needs a log per (branch, target) pair. Excluding every target tip keeps one log per branch; the cost is that work on another plan's target is neither seen nor unmerged for an item, which item 1d17's problem rules can flag.
  - Reusing `last_landed` as the merge time: it's the linked commit's own date, already in the JSON contract, so `merged_at` is a new field.
  - Repeating the target on every plan in the overview, and storing the merge time (the plan says never).
  - Changing the done-not-in-git threshold: that's item 1d17.
decisions:
  - id: 1af8
    text: Unmerged work is linked commits on no target branch (each branch is read with --not every target's tip); target branches and release are never read as unmerged branches
    detail: A branch merged into its plan's target and left behind must not show as unmerged; one log per branch, as before. Work that reached another plan's target (or integration) is neither seen nor unmerged for an item
  - id: 5d62
    text: 'Unlinked commits are still counted on config''s integration branch only, and view''s top-level integration and ahead_of_release stay config''s; each Board (and PlanStatus and context''s plans) gets target: {branch, from, found, ahead_of_release}'
    detail: Unplanned work lands on integration, so the board for it gets integration as its target
  - id: 6c36
    text: 'The merge time is a new merged_at on an item''s git: the date of the target''s first-parent commit that brought in its latest linked commit; last_landed keeps its old meaning'
    detail: Derived each read from the first-parent line, never stored; last_landed is in the JSON contract already
  - id: a78c
    text: A plan's target is set with a new mull plan set <slug> title|summary|target <value> (batch op plan set), and mull set <slug> target works too; like title and summary it needs no --on-request
    detail: The phase doc names mull plan set; plan set is set restricted to plans, so the two can't disagree
  - id: b3de
    text: view --text opens every plan block with a target line; mull serve shows Target on every plan sheet, but in the overview's Plans column only for a plan with its own target
    detail: The project title block already shows the integration branch and its ahead count, so repeating it on every plan in the overview is noise
  - id: b462
    text: The done-not-in-git problem keeps its three-day wait, now measured on the item's target; making accepted-but-not-in-target a problem at once is left to item 1d17
    detail: 1d17 is the next item in this phase and owns that rule
  - id: b77b
    text: The bench seed takes -targets n (0 to 2 active plans get a target branch cut from main when the plan starts); the named sizes keep 0, so make bench numbers stay comparable
    detail: Measures the per-plan reachability work without changing the default seed other workers compare against
  - id: d08b
    text: A target is any name git check-ref-format --branch accepts, except mull; a leading refs/heads/ is dropped, "" clears it, and the branch needn't exist yet (the view says branch not found)
    detail: A plan's target branch is often created after the plan is set up
  - id: d70e
    text: done's fallback base, when a claim has no recorded base, is the merge base with the item's plan's target rather than integration
    detail: Where the work goes is where its changes are measured from
---

# Done report 1: A plan's target branch, with seen in git measured against it (ed9d)

**Status:** PASSED  
**Commit:** 8b5e77ae0ded  
**By:** /Users/jeff/dev/kit/.claude/worktrees/agent-a7c6b593278524845 at 2026-11-22T16:33:50Z

## What was built

### Summary

A plan can now have a target branch (mull plan set SLUG target BRANCH, or mull set; it falls back to config integration). Seen-in-history is now measured per item against its plan's target: trailers and id-branch merges count only on that branch, unmerged work is commits on no target, and a new merged_at on an item's history is derived from the target's first-parent commit that brought the work in. view (JSON and --text), context and mull serve (plan sheet, the overview for plans with their own target, the item page) show each plan's target and how far it is ahead of release. done's fallback base uses the plan's target. design.md, the json, intake and supervisor guides, and serve's DESIGN.md are updated; the demo's web plan now targets web-next; the bench seed takes -targets.

### UI tour

1. Run make dev-reset (the demo needs reseeding: its web plan now targets a web-next branch), then open http://localhost:6899/#/demo
2. In the Plans column, Web app has a Target row: web-next, with a muted '2 commits ahead of main'. Pilot (shaping, no target of its own) has no Target row.
3. Open Web app: the title block has a Target field reading web-next, '2 commits ahead of main'.
4. Open the item Digest job (in review): its commits field reads '2 commits on web-next' (the commit and its merge), with 'merged N ago' muted beside it.
5. Open the item Mailer (Web app, no commits): the same field reads 'no commits yet', with 'lands on web-next' muted.
6. Open Fix the login redirect under Unplanned: the same field reads 'no commits yet', with 'lands on main' muted.

### Checks (run by mull)

- `make done-checks`: pass, 8.2s

## How it was built

### Design notes

## Approach

A plan gets one new stored field, `target` (YAML `target:`, omitted when empty). Everything else is derived on read: which branch each item is measured against, whether it has landed, when it was merged, and how far each target is ahead of release. Nothing about landing is stored.

## Shape of the code

- `internal/mull/links.go` holds it all. `state.integration()` (config's, else main) and `state.targetOf(slug)` (the plan's target, else integration, with where it came from) are the only places that decide a branch. `readTargets` reads every distinct target's tip (one `for-each-ref` already done by the caller) and its ahead-of-release count (cached `rev-list --count`). `state.target(slug, targets)` gives the public `Target {branch, from, found, ahead_of_release}` used by Board, PlanStatus and context's BriefPlan.
- `readGit` now loops over the target branches instead of reading integration once. For each target it reads the history from its tip and credits a trailer or an id-branch merge only to items whose plan targets that branch. Unlinked commits are still collected only on integration. Unmerged work is read per non-target branch with `git log <branch> --not <every target tip>`, so a branch merged into its target and left behind doesn't count, and the git-process count per branch is unchanged.
- `merged_at` comes from `broughtIn`: walk the target's first-parent line, then oldest first, claim each first-parent commit and everything its other parents reach that isn't claimed yet. The claimed set stays closed under parents, so each walk stops at older history, and the whole pass is linear in the commits read. An item's `merged_at` is the latest of those dates over its linked commits.
- Each target's history (commits, by-ID index, brought-in dates) is cached per tip in a small LRU (`targetHistories`, 16), like `gitLogs`. The first measurement without it showed the large board up from 33.8 to 46.7 ms from rebuilding the index on every read; with it, it's back at 34.5.
- `plan set` is `set` limited to plans (a new op, so batch matches the command), and `set` learned `target`. `checkTarget` uses `git check-ref-format --branch`, trims `refs/heads/`, and refuses `mull`.
- `done`'s fallback base (no claim base) is now the merge base with the item's plan's target.
- Callers that looked up `g.Items[id]` with a nil fallback now use `state.gitOf`, so an item with no commits still reports the target it will be measured against.

## UI

`targetField` in title.js renders a target (mono branch, muted note: the integration branch / not found in git / N commits ahead of / level with release). The plan sheet's title block always shows it; the overview's Plans column shows a Target row only for a plan with its own target, since the project title block already shows integration. The item page's Git field now says "N commits on <target> · merged 2h ago", "not on <target> yet · N unmerged", or "no commits yet, lands on <target>". The old field said "no commits yet" even when there were unmerged commits; that's fixed in passing, since the field had to change anyway.

## Bench (make bench, in-process medians, ms)

| page | before | after (default large) | large, 2 plans with own targets |
|---|---|---|---|
| board (mull view) | 33.8 | 34.5 | 37.2 |
| overview | 40.1 | 41.5 | 45.3 |
| plan metrics-23 | 26.3 | 28.8 | 32.2 |
| item c65f | 25.5 | 25.4 | 27.7 |
| projects (summary) | 23.2 | 23.8 | 24.4 |

Git process counts per call are unchanged on every page (a target's log is cached by tip, like integration's). Runs vary by about 10% (pages this change doesn't touch, like tree and backlog, moved as much). The targets column is a repo seeded with the new `bench seed -targets 2`; the named sizes keep 0 targets, so the default seed is unchanged for anyone comparing numbers.

## Rejected

- Measuring unmerged per item against its own target only: that needs a log per (branch, target) pair. Excluding every target tip keeps one log per branch; the cost is that work on another plan's target is neither seen nor unmerged for an item, which item 1d17's problem rules can flag.
- Reusing `last_landed` as the merge time: it's the linked commit's own date, already in the JSON contract, so `merged_at` is a new field.
- Repeating the target on every plan in the overview, and storing the merge time (the plan says never).
- Changing the done-not-in-git threshold: that's item 1d17.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **1af8** Unmerged work is linked commits on no target branch (each branch is read with --not every target's tip); target branches and release are never read as unmerged branches
  Why: A branch merged into its plan's target and left behind must not show as unmerged; one log per branch, as before. Work that reached another plan's target (or integration) is neither seen nor unmerged for an item
- **5d62** Unlinked commits are still counted on config's integration branch only, and view's top-level integration and ahead_of_release stay config's; each Board (and PlanStatus and context's plans) gets target: {branch, from, found, ahead_of_release}
  Why: Unplanned work lands on integration, so the board for it gets integration as its target
- **6c36** The merge time is a new merged_at on an item's git: the date of the target's first-parent commit that brought in its latest linked commit; last_landed keeps its old meaning
  Why: Derived each read from the first-parent line, never stored; last_landed is in the JSON contract already
- **a78c** A plan's target is set with a new mull plan set <slug> title|summary|target <value> (batch op plan set), and mull set <slug> target works too; like title and summary it needs no --on-request
  Why: The phase doc names mull plan set; plan set is set restricted to plans, so the two can't disagree
- **b3de** view --text opens every plan block with a target line; mull serve shows Target on every plan sheet, but in the overview's Plans column only for a plan with its own target
  Why: The project title block already shows the integration branch and its ahead count, so repeating it on every plan in the overview is noise
- **b462** The done-not-in-git problem keeps its three-day wait, now measured on the item's target; making accepted-but-not-in-target a problem at once is left to item 1d17
  Why: 1d17 is the next item in this phase and owns that rule
- **b77b** The bench seed takes -targets n (0 to 2 active plans get a target branch cut from main when the plan starts); the named sizes keep 0, so make bench numbers stay comparable
  Why: Measures the per-plan reachability work without changing the default seed other workers compare against
- **d08b** A target is any name git check-ref-format --branch accepts, except mull; a leading refs/heads/ is dropped, "" clears it, and the branch needn't exist yet (the view says branch not found)
  Why: A plan's target branch is often created after the plan is set up
- **d70e** done's fallback base, when a claim has no recorded base, is the merge base with the item's plan's target rather than integration
  Why: Where the work goes is where its changes are measured from

### Changes by layer

**other**: 25 files, +408 −104
- M mull/cmd/commands.go (+15 −2)
- M mull/docs/design.md (+16 −7)
- M mull/docs/serve/DESIGN.md (+2 −2)
- M mull/internal/bench/seed.go (+16 −1)
- M mull/internal/guide/intake.md (+1 −0)
- M mull/internal/guide/json.md (+9 −6)
- M mull/internal/guide/supervisor.md (+1 −1)
- M mull/internal/model/model.go (+3 −0)
- M mull/internal/mull/cache.go (+4 −0)
- M mull/internal/mull/context.go (+8 −2)
- M mull/internal/mull/done.go (+7 −8)
- M mull/internal/mull/links.go (+212 −51)
- M mull/internal/mull/ops.go (+17 −0)
- M mull/internal/mull/pages.go (+1 −3)
- M mull/internal/mull/revision.go (+1 −0)
- M mull/internal/mull/schedule.go (+2 −1)
- M mull/internal/mull/summary.go (+1 −4)
- M mull/internal/mull/view.go (+24 −6)
- M mull/internal/mull/work.go (+26 −2)
- M mull/internal/serve/web/item.js (+11 −4)
- M mull/internal/serve/web/plan.js (+2 −1)
- M mull/internal/serve/web/plans.js (+5 −2)
- M mull/internal/serve/web/title.js (+12 −0)
- M mull/scripts/bench/main.go (+3 −1)
- M mull/scripts/dev-demo.sh (+9 −0)

**tests**: 3 files, +246 −3
- M mull/cmd/shapes_test.go (+12 −3)
- M mull/internal/bench/bench_test.go (+31 −0)
- A mull/internal/mull/target_test.go (+203 −0)
