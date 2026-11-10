---
item: ba4c
"n": 1
passed: true
commit: 69a0f11a1c1e16f0394b3a57b25f28c8feec99f0
base: d3db7357e15a3b445cbdbceb899c7fdbcc8b8454
at: "2026-11-10T19:50:06Z"
by: /Users/jeff/dev/kit
summary: Item stages are now captured → committed → active → in-review → done (or dropped). Old done is in-review and old accepted is done, so done means reviewed. add creates committed items; start works from captured or committed; release and send-back return to committed. Readiness requires committed; captured items are backlog, never ready, excluded from phase totals and completion and counted separately in Progress (new keys captured, committed, in_review, done). Storage keeps the old names and translates at the model's YAML boundary until the 488d migration. list --state validates the stage. Nothing creates captured items yet; 462a adds the capture commands.
tour:
  - 'none (CLI only). To see it: go build in mull/, then ./mull context and note phase progress reads "n/m done · k in review"'
  - /mull list --state committed --phase 204f lists the Model v2 items; ./mull list --state open is refused with the list of stages
checks:
  tree: 925e36fb0df1192d943cf9d80145d4ec321b1ab5
  commit: 69a0f11a1c1e16f0394b3a57b25f28c8feec99f0
  at: "2026-11-10T19:50:04Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 2009
changes:
  - layer: other
    files:
      - status: M
        path: mull/cmd/commands.go
        added: 3
        removed: 3
      - status: M
        path: mull/docs/design.md
        added: 12
        removed: 9
      - status: M
        path: mull/internal/guide/json.md
        added: 2
        removed: 2
      - status: M
        path: mull/internal/guide/supervisor.md
        added: 2
        removed: 2
      - status: M
        path: mull/internal/model/model.go
        added: 51
        removed: 6
      - status: M
        path: mull/internal/mull/done.go
        added: 2
        removed: 2
      - status: M
        path: mull/internal/mull/graph.go
        added: 33
        removed: 23
      - status: M
        path: mull/internal/mull/inbox.go
        added: 10
        removed: 10
      - status: M
        path: mull/internal/mull/view.go
        added: 10
        removed: 7
      - status: M
        path: mull/internal/mull/work.go
        added: 9
        removed: 4
  - layer: tests
    files:
      - status: M
        path: mull/cmd/shapes_test.go
        added: 1
        removed: 1
      - status: M
        path: mull/internal/model/model_test.go
        added: 22
        removed: 0
      - status: M
        path: mull/internal/mull/done_test.go
        added: 2
        removed: 2
      - status: M
        path: mull/internal/mull/mull_test.go
        added: 105
        removed: 13
      - status: M
        path: mull/internal/mull/view_test.go
        added: 2
        removed: 2
---

# Done report 1: Item stages: captured, committed, active, in review, done (ba4c)

**Status:** PASSED  
**Commit:** 69a0f11a1c1e  
**By:** /Users/jeff/dev/kit at 2026-11-10T19:50:06Z

## Summary

Item stages are now captured → committed → active → in-review → done (or dropped). Old done is in-review and old accepted is done, so done means reviewed. add creates committed items; start works from captured or committed; release and send-back return to committed. Readiness requires committed; captured items are backlog, never ready, excluded from phase totals and completion and counted separately in Progress (new keys captured, committed, in_review, done). Storage keeps the old names and translates at the model's YAML boundary until the 488d migration. list --state validates the stage. Nothing creates captured items yet; 462a adds the capture commands.

## UI tour

1. none (CLI only). To see it: go build in mull/, then ./mull context and note phase progress reads "n/m done · k in review"
2. /mull list --state committed --phase 204f lists the Model v2 items; ./mull list --state open is refused with the list of stages

## Checks (run by mull)

- `make done-checks`: pass, 2.0s

## Changes by layer

**other**: 10 files, +134 −68
- M mull/cmd/commands.go (+3 −3)
- M mull/docs/design.md (+12 −9)
- M mull/internal/guide/json.md (+2 −2)
- M mull/internal/guide/supervisor.md (+2 −2)
- M mull/internal/model/model.go (+51 −6)
- M mull/internal/mull/done.go (+2 −2)
- M mull/internal/mull/graph.go (+33 −23)
- M mull/internal/mull/inbox.go (+10 −10)
- M mull/internal/mull/view.go (+10 −7)
- M mull/internal/mull/work.go (+9 −4)

**tests**: 5 files, +132 −18
- M mull/cmd/shapes_test.go (+1 −1)
- M mull/internal/model/model_test.go (+22 −0)
- M mull/internal/mull/done_test.go (+2 −2)
- M mull/internal/mull/mull_test.go (+105 −13)
- M mull/internal/mull/view_test.go (+2 −2)
