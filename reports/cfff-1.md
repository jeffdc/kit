---
item: cfff
"n": 1
passed: true
commit: b3739e6591c48de24c1019e995e5696ada79ab82
base: e9e017782cec5914263af74766e2ad860e47e3ed
at: "2026-11-16T14:34:57Z"
by: /Users/jeff/dev/kit
summary: New command mull phase delete <id> --on-request (and batch op 'phase delete') removes a phase no longer needed, like gallformers' 9d90. Only a phase with no committed work in any stage (committed, active, in review, done or dropped) can go; otherwise invalid_state names the items. Captured items tagged to it move up to its plan, other phases' needs on it are removed, and both are listed in the result; the phase's record and doc leave the mull branch, whose history keeps them. Documented in mull guide json and design.md.
checks:
  tree: 727a1f87c0c81dfd8e885377cc3477f8b051782c
  commit: b3739e6591c48de24c1019e995e5696ada79ab82
  at: "2026-11-16T14:33:05Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 111984
changes:
  - layer: other
    files:
      - status: M
        path: mull/cmd/commands.go
        added: 12
        removed: 1
      - status: M
        path: mull/docs/design.md
        added: 2
        removed: 1
      - status: M
        path: mull/internal/guide/json.md
        added: 2
        removed: 0
      - status: M
        path: mull/internal/mull/ops.go
        added: 12
        removed: 0
      - status: M
        path: mull/internal/mull/work.go
        added: 69
        removed: 0
  - layer: tests
    files:
      - status: M
        path: mull/internal/mull/mull_test.go
        added: 57
        removed: 0
design: |-
  Jeff chose delete-if-empty over a dropped state for phases. A dropped state would have needed every place that lists phases (the view, schedule, ready, context, pages, search, the needs graph, mull serve) to filter it out. Deleting an empty phase needs nothing new anywhere else: the record just isn't there, and the revision feed already reports removed records.

  phaseDelete (internal/mull/work.go) checks for work first, then --on-request, the same order commit uses, so an agent learns the phase isn't deletable before being told to ask. "Empty" means no item in a stage other than captured. A phase with done or dropped items is refused too: if all its items are done or dropped it's complete and belongs in Built, and deleting it would orphan those items' phase field. Captured items are backlog, so they're untagged to the plan (phase cleared, plan kept) rather than blocking the delete. Needs on it are removed from other phases, since a need on a phase that doesn't exist can never be met. The doc goes through putDoc with an empty doc, the same removal phase doc - uses.

  Rejected: auto-moving a phase's dropped items to the plan so it could be deleted (that rewrites records silently), and treating a deleted phase as a met need (that would unblock phases without anyone saying so; removing the need and listing it is just as loud and more honest).

  Tests: TestPhaseDelete covers the on-request guard, not_found, refusing committed and dropped items, the result's untagged and unneeded lists, the record and doc gone from the branch, and the batch op.
decisions:
  - id: "0724"
    text: A settled entry that made the deleted phase (do --as phase) keeps its phase ID as history; it isn't rewritten
    detail: It records what the entry made at the time; the mull branch's history still has the phase
  - id: 4f77
    text: 'mull phase delete is CLI and batch only (op ''phase delete'': id, on-request); mull serve doesn''t offer it yet'
    detail: Serve takes only the ops its UI offers; a delete button is UI work for mull-ui-edit if wanted
---

# Done report 1: Phases Can Not be Deleted (cfff)

**Status:** PASSED  
**Commit:** b3739e6591c4  
**By:** /Users/jeff/dev/kit at 2026-11-16T14:34:57Z

## What was built

### Summary

New command mull phase delete <id> --on-request (and batch op 'phase delete') removes a phase no longer needed, like gallformers' 9d90. Only a phase with no committed work in any stage (committed, active, in review, done or dropped) can go; otherwise invalid_state names the items. Captured items tagged to it move up to its plan, other phases' needs on it are removed, and both are listed in the result; the phase's record and doc leave the mull branch, whose history keeps them. Documented in mull guide json and design.md.

### UI tour

None given.

### Checks (run by mull)

- `make done-checks`: pass, 112.0s

## How it was built

### Design notes

Jeff chose delete-if-empty over a dropped state for phases. A dropped state would have needed every place that lists phases (the view, schedule, ready, context, pages, search, the needs graph, mull serve) to filter it out. Deleting an empty phase needs nothing new anywhere else: the record just isn't there, and the revision feed already reports removed records.

phaseDelete (internal/mull/work.go) checks for work first, then --on-request, the same order commit uses, so an agent learns the phase isn't deletable before being told to ask. "Empty" means no item in a stage other than captured. A phase with done or dropped items is refused too: if all its items are done or dropped it's complete and belongs in Built, and deleting it would orphan those items' phase field. Captured items are backlog, so they're untagged to the plan (phase cleared, plan kept) rather than blocking the delete. Needs on it are removed from other phases, since a need on a phase that doesn't exist can never be met. The doc goes through putDoc with an empty doc, the same removal phase doc - uses.

Rejected: auto-moving a phase's dropped items to the plan so it could be deleted (that rewrites records silently), and treating a deleted phase as a met need (that would unblock phases without anyone saying so; removing the need and listing it is just as loud and more honest).

Tests: TestPhaseDelete covers the on-request guard, not_found, refusing committed and dropped items, the result's untagged and unneeded lists, the record and doc gone from the branch, and the batch op.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **0724** A settled entry that made the deleted phase (do --as phase) keeps its phase ID as history; it isn't rewritten
  Why: It records what the entry made at the time; the mull branch's history still has the phase
- **4f77** mull phase delete is CLI and batch only (op 'phase delete': id, on-request); mull serve doesn't offer it yet
  Why: Serve takes only the ops its UI offers; a delete button is UI work for mull-ui-edit if wanted

### Changes by layer

**other**: 5 files, +97 −2
- M mull/cmd/commands.go (+12 −1)
- M mull/docs/design.md (+2 −1)
- M mull/internal/guide/json.md (+2 −0)
- M mull/internal/mull/ops.go (+12 −0)
- M mull/internal/mull/work.go (+69 −0)

**tests**: 1 files, +57 −0
- M mull/internal/mull/mull_test.go (+57 −0)
