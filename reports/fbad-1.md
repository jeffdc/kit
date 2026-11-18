---
item: fbad
"n": 1
passed: true
commit: c7c48e71113d70d9475fb33880955a4dc1124747
base: 07eab381032f72b52f23ddfa7cf9088ff18f6663
at: "2026-11-18T21:00:28Z"
by: /Users/jeff/dev/kit
summary: 'mull serve''s item lists follow rank: a phase''s Items, a plan''s Not in a phase, Unplanned, the sheet index and What''s next list open work in rank order, then done and dropped, unranked last (they were by stage). Those three tables start with a # column, an open item''s position (Jeff''s choice). The Plans column''s agents and the overview''s are in work order. ItemRow gains position, and the docs (DESIGN.md, mull guide json) say so. With this, a phase''s sheet shows the order mull rank gave, and a reorder is clouded like any change since rank is in the change feed (1917).'
tour:
  - 'Open a phase with several committed items (in kit: item-order''s phases, or rank a phase with mull rank first).'
  - 'The Items table starts with a # column: 1, 2, 3 down the open items in the order mull rank gave, whatever their stage; done and dropped rows follow with no number.'
  - 'Run mull rank on that phase with a different order: the table redraws in the new order with the moved rows clouded.'
  - On the plan's Status tab, What's next lists that phase's work in the same order (work under way and the first two ready ones), and the sheet index lists the phase's items in it too.
  - 'Unplanned and a plan''s Not in a phase have the same # column.'
checks:
  tree: 22e333d43cd9fc75eb4fbae0b3958838c2ae2e60
  commit: c7c48e71113d70d9475fb33880955a4dc1124747
  at: "2026-11-18T20:58:26Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 122405
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/guide/json.md
        added: 7
        removed: 7
      - status: M
        path: mull/internal/mull/pages.go
        added: 32
        removed: 18
      - status: M
        path: mull/internal/mull/rank.go
        added: 10
        removed: 3
      - status: M
        path: mull/internal/mull/schedule.go
        added: 6
        removed: 2
      - status: M
        path: mull/internal/serve/web/lists.js
        added: 5
        removed: 2
      - status: M
        path: mull/internal/serve/web/phase.js
        added: 1
        removed: 1
      - status: M
        path: mull/internal/serve/web/plan.js
        added: 4
        removed: 2
  - layer: tests
    files:
      - status: M
        path: mull/internal/mull/pages_test.go
        added: 2
        removed: 2
      - status: M
        path: mull/internal/mull/rank_test.go
        added: 60
        removed: 0
design: |-
  Server: rowsOf in pages.go, which every serve item list goes through (phase Items and Captured, plan Unfiled and Next, Unplanned, Backlog, the tree's phase and unfiled rows), sorted by stage then age; it now sorts by workLess (open work first, then rankLess), the within-group half of the CLI's workOrder, which I split out of rank.go so both share it. I didn't use the full workOrder there: each of those lists holds one group, and workOrder's phase schedule would be computed once per tree phase for nothing. The backlog re-sorts itself (newest first), so it's unchanged.

  Position: rowsOf also numbers open ranked rows per group as it walks the sorted list, one more than the open rows before it, a tie keeping the previous number (1, 1, 3). That's correct because every list serve draws includes all of its group's open work (the tree's listable filter only ages out done work), so no extra pass over s.items per row. ItemRow.position is omitempty: finished, captured and unranked rows have none.

  Agents: planStatus's agents and the overview's are now in workOrder (phase schedule, then rank), cheap since only active items are sorted.

  UI: col.pos() in lists.js is a right-aligned mono # column, used first in the phase Items, plan Not in a phase and Unplanned tables, not the Captured or Backlog tables (no rank). The phase header reads One session each · in rank order. What's next kept its rule (work under way plus the first two ready items) but now filters in place, so rank order holds instead of under-way-first.

  Rejected: numbers in the sheet index (its rows are tight, and order shows there anyway) and in What's next. Tests: TestServePagesFollowRank (phase page order and positions, What's next, agents, tree, Unplanned with equal and unranked items); TestOverviewTreeAndLists updated from active-first to rank order. make bench (large): overview 37.5 to 37.9 ms, tree 14.9 to 15.1, unplanned 12.2 to 11.7, phase 13.5 to 12.2, all noise. Checked in the dev server through headless Chromium driven over the DevTools protocol: the demo's Launch phase ranked 4012, 48b4, f86b, 7b94 shows 1 to 4 in that order with the active item second, and What's next matches.
decisions:
  - id: e555
    text: 'Jeff chose: mull serve''s item tables show a position number (1, 2, 3) on open items in rank order, equal ranks sharing one and unranked items none; done and dropped list after, unnumbered'
    detail: Settles the phase doc's open question; asked 2026-11-18
  - id: b107
    text: 'mull serve''s tables count positions from the list itself: one more than the open rows ranked before it, so equal ranks share one (1, 1, 3)'
    detail: Every list serve draws holds all of its group's open work, so no second pass over the items is needed; 1, 1, 3 keeps a number meaning how many come first
---

# Done report 1: mull serve lists items in rank order (fbad)

**Status:** PASSED  
**Commit:** c7c48e71113d  
**By:** /Users/jeff/dev/kit at 2026-11-18T21:00:28Z

## What was built

### Summary

mull serve's item lists follow rank: a phase's Items, a plan's Not in a phase, Unplanned, the sheet index and What's next list open work in rank order, then done and dropped, unranked last (they were by stage). Those three tables start with a # column, an open item's position (Jeff's choice). The Plans column's agents and the overview's are in work order. ItemRow gains position, and the docs (DESIGN.md, mull guide json) say so. With this, a phase's sheet shows the order mull rank gave, and a reorder is clouded like any change since rank is in the change feed (1917).

### UI tour

1. Open a phase with several committed items (in kit: item-order's phases, or rank a phase with mull rank first).
2. The Items table starts with a # column: 1, 2, 3 down the open items in the order mull rank gave, whatever their stage; done and dropped rows follow with no number.
3. Run mull rank on that phase with a different order: the table redraws in the new order with the moved rows clouded.
4. On the plan's Status tab, What's next lists that phase's work in the same order (work under way and the first two ready ones), and the sheet index lists the phase's items in it too.
5. Unplanned and a plan's Not in a phase have the same # column.

### Checks (run by mull)

- `make done-checks`: pass, 122.4s

## How it was built

### Design notes

Server: rowsOf in pages.go, which every serve item list goes through (phase Items and Captured, plan Unfiled and Next, Unplanned, Backlog, the tree's phase and unfiled rows), sorted by stage then age; it now sorts by workLess (open work first, then rankLess), the within-group half of the CLI's workOrder, which I split out of rank.go so both share it. I didn't use the full workOrder there: each of those lists holds one group, and workOrder's phase schedule would be computed once per tree phase for nothing. The backlog re-sorts itself (newest first), so it's unchanged.

Position: rowsOf also numbers open ranked rows per group as it walks the sorted list, one more than the open rows before it, a tie keeping the previous number (1, 1, 3). That's correct because every list serve draws includes all of its group's open work (the tree's listable filter only ages out done work), so no extra pass over s.items per row. ItemRow.position is omitempty: finished, captured and unranked rows have none.

Agents: planStatus's agents and the overview's are now in workOrder (phase schedule, then rank), cheap since only active items are sorted.

UI: col.pos() in lists.js is a right-aligned mono # column, used first in the phase Items, plan Not in a phase and Unplanned tables, not the Captured or Backlog tables (no rank). The phase header reads One session each · in rank order. What's next kept its rule (work under way plus the first two ready items) but now filters in place, so rank order holds instead of under-way-first.

Rejected: numbers in the sheet index (its rows are tight, and order shows there anyway) and in What's next. Tests: TestServePagesFollowRank (phase page order and positions, What's next, agents, tree, Unplanned with equal and unranked items); TestOverviewTreeAndLists updated from active-first to rank order. make bench (large): overview 37.5 to 37.9 ms, tree 14.9 to 15.1, unplanned 12.2 to 11.7, phase 13.5 to 12.2, all noise. Checked in the dev server through headless Chromium driven over the DevTools protocol: the demo's Launch phase ranked 4012, 48b4, f86b, 7b94 shows 1 to 4 in that order with the active item second, and What's next matches.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **e555** Jeff chose: mull serve's item tables show a position number (1, 2, 3) on open items in rank order, equal ranks sharing one and unranked items none; done and dropped list after, unnumbered
  Why: Settles the phase doc's open question; asked 2026-11-18
- **b107** mull serve's tables count positions from the list itself: one more than the open rows ranked before it, so equal ranks share one (1, 1, 3)
  Why: Every list serve draws holds all of its group's open work, so no second pass over the items is needed; 1, 1, 3 keeps a number meaning how many come first

### Changes by layer

**other**: 8 files, +66 −35
- M mull/docs/serve/DESIGN.md (+1 −0)
- M mull/internal/guide/json.md (+7 −7)
- M mull/internal/mull/pages.go (+32 −18)
- M mull/internal/mull/rank.go (+10 −3)
- M mull/internal/mull/schedule.go (+6 −2)
- M mull/internal/serve/web/lists.js (+5 −2)
- M mull/internal/serve/web/phase.js (+1 −1)
- M mull/internal/serve/web/plan.js (+4 −2)

**tests**: 2 files, +62 −2
- M mull/internal/mull/pages_test.go (+2 −2)
- M mull/internal/mull/rank_test.go (+60 −0)
