---
item: cab7
"n": 1
passed: true
commit: 7b19a309d8e5ff0a32d176346565b17a535a7b50
base: f894f1b4dbae1d5518721b1290c5dc9ad5e2d487
at: "2026-11-22T17:27:17Z"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a2f4161f2e304f600
summary: 'A plan now has a preview: the URL that serves its target branch, set with mull plan set <slug> preview <url> (or set, or a plan set batch line; "" clears it). mull context --item hands the worker both its item''s target (the Target object) and the plan''s preview, so a worker knows where its work lands and where Jeff will see it. context''s plans, mull view (a preview line under each plan''s target in --text) and the revision feed show it too. mull records it only: it never runs, fetches or checks the server. design.md, the json guide, the agents guide (and kit''s AGENTS.md copy), intake and supervisor guides are updated.'
checks:
  tree: 9b5bd6b6dd4c64d787f7288a0654a06340ef5dc3
  commit: 7b19a309d8e5ff0a32d176346565b17a535a7b50
  at: "2026-11-22T17:27:08Z"
  by: /Users/jeff/dev/kit/.claude/worktrees/agent-a2f4161f2e304f600
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 8232
changes:
  - layer: other
    files:
      - status: M
        path: AGENTS.md
        added: 1
        removed: 0
      - status: M
        path: mull/cmd/commands.go
        added: 4
        removed: 3
      - status: M
        path: mull/docs/design.md
        added: 9
        removed: 4
      - status: M
        path: mull/internal/guide/agents.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/guide/intake.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/guide/json.md
        added: 5
        removed: 5
      - status: M
        path: mull/internal/guide/supervisor.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/model/model.go
        added: 4
        removed: 0
      - status: M
        path: mull/internal/mull/context.go
        added: 14
        removed: 8
      - status: M
        path: mull/internal/mull/ops.go
        added: 1
        removed: 1
      - status: M
        path: mull/internal/mull/revision.go
        added: 1
        removed: 0
      - status: M
        path: mull/internal/mull/view.go
        added: 7
        removed: 3
      - status: M
        path: mull/internal/mull/work.go
        added: 25
        removed: 2
  - layer: tests
    files:
      - status: M
        path: mull/cmd/shapes_test.go
        added: 15
        removed: 3
      - status: A
        path: mull/internal/mull/preview_test.go
        added: 84
        removed: 0
design: |-
  How: preview mirrors the plan target from ed9d. model.Plan gains Preview (yaml/json preview, omitempty), so the record, plan show, plan list, set and serve's plan API carry it with no extra work. Mull.set takes a new plan field "preview", validated by checkPreview: trimmed, then net/url must parse it with an http or https scheme and a non-empty host; "" clears it. A bare host:port (localhost:4000) parses as scheme "localhost", so it's refused, which matters because 743b will resolve tour paths against this URL. The value is otherwise stored as given (no trailing-slash normalisation); joining paths is 743b's concern.

  Context: ContextItem gains target (s.target(i.Plan, targets), using the targets already read for the plans list, so nothing more is read) and preview?. BriefPlan and the view's PlanBoard gain preview?, and View.Text prints "  preview <url>" under the target line. planChange in revision.go diffs preview like target, so a change to it shows in mull serve's change feed.

  Rejected: a config-level preview fallback (the phase doc puts the preview on the plan only, and unlike a target nothing measures against a default); a protocol line in context (the field is self-describing and the agents guide explains it); showing the preview in mull serve's UI now (743b uses it there, so I left the UI alone and the tour is none).

  Tests: internal/mull/preview_test.go covers set (trim, refused forms, batch plan set, a phase refusing the field, clearing) and context/view hand-over (a plan with target and preview; a plan without, and unplanned work, falling back to integration with no preview; the view text line). cmd/shapes_test.go documents preview on Plan, Board and context plans, and now checks context --item's item shape including target and preview.

  Bench: not run. No serve page's read path changed (the bench times serve pages); context and view only copy a string already in the loaded plan record, with no new reads.
decisions:
  - id: "0087"
    text: 'A preview has no config fallback: unplanned work, and a plan that sets none, has no preview'
    detail: The phase doc gives the preview to the plan only; a target falls back to integration because git measures against it, but nothing needs a default preview
  - id: 0a70
    text: context --item gains target (the Target object, as the plans list has it) and preview? on the item; context's plans, view's boards and view --text (a preview line under the target) show the preview too
    detail: The worker gets both without searching the plans list; the supervisor sees previews in mull view
  - id: 9c78
    text: 'mull serve shows no preview yet: its plan API carries it in the Plan record, and 743b, which resolves tours against it, is where the UI uses it'
    detail: Keeps this item to recording and handing over; no UI change, so no tour
  - id: a960
    text: A plan's preview must be an absolute http or https URL with a host; it's trimmed and otherwise stored as given, and "" clears it
    detail: Tours will be resolved against it (743b), so a bare host:port or a path is refused rather than guessed at; mull never fetches it
---

# Done report 1: A plan's preview URL, handed to workers (cab7)

**Status:** PASSED  
**Commit:** 7b19a309d8e5  
**By:** /Users/jeff/dev/kit/.claude/worktrees/agent-a2f4161f2e304f600 at 2026-11-22T17:27:17Z

## What was built

### Summary

A plan now has a preview: the URL that serves its target branch, set with mull plan set <slug> preview <url> (or set, or a plan set batch line; "" clears it). mull context --item hands the worker both its item's target (the Target object) and the plan's preview, so a worker knows where its work lands and where Jeff will see it. context's plans, mull view (a preview line under each plan's target in --text) and the revision feed show it too. mull records it only: it never runs, fetches or checks the server. design.md, the json guide, the agents guide (and kit's AGENTS.md copy), intake and supervisor guides are updated.

### UI tour

None given.

### Checks (run by mull)

- `make done-checks`: pass, 8.2s

## How it was built

### Design notes

How: preview mirrors the plan target from ed9d. model.Plan gains Preview (yaml/json preview, omitempty), so the record, plan show, plan list, set and serve's plan API carry it with no extra work. Mull.set takes a new plan field "preview", validated by checkPreview: trimmed, then net/url must parse it with an http or https scheme and a non-empty host; "" clears it. A bare host:port (localhost:4000) parses as scheme "localhost", so it's refused, which matters because 743b will resolve tour paths against this URL. The value is otherwise stored as given (no trailing-slash normalisation); joining paths is 743b's concern.

Context: ContextItem gains target (s.target(i.Plan, targets), using the targets already read for the plans list, so nothing more is read) and preview?. BriefPlan and the view's PlanBoard gain preview?, and View.Text prints "  preview <url>" under the target line. planChange in revision.go diffs preview like target, so a change to it shows in mull serve's change feed.

Rejected: a config-level preview fallback (the phase doc puts the preview on the plan only, and unlike a target nothing measures against a default); a protocol line in context (the field is self-describing and the agents guide explains it); showing the preview in mull serve's UI now (743b uses it there, so I left the UI alone and the tour is none).

Tests: internal/mull/preview_test.go covers set (trim, refused forms, batch plan set, a phase refusing the field, clearing) and context/view hand-over (a plan with target and preview; a plan without, and unplanned work, falling back to integration with no preview; the view text line). cmd/shapes_test.go documents preview on Plan, Board and context plans, and now checks context --item's item shape including target and preview.

Bench: not run. No serve page's read path changed (the bench times serve pages); context and view only copy a string already in the loaded plan record, with no new reads.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **0087** A preview has no config fallback: unplanned work, and a plan that sets none, has no preview
  Why: The phase doc gives the preview to the plan only; a target falls back to integration because git measures against it, but nothing needs a default preview
- **0a70** context --item gains target (the Target object, as the plans list has it) and preview? on the item; context's plans, view's boards and view --text (a preview line under the target) show the preview too
  Why: The worker gets both without searching the plans list; the supervisor sees previews in mull view
- **9c78** mull serve shows no preview yet: its plan API carries it in the Plan record, and 743b, which resolves tours against it, is where the UI uses it
  Why: Keeps this item to recording and handing over; no UI change, so no tour
- **a960** A plan's preview must be an absolute http or https URL with a host; it's trimmed and otherwise stored as given, and "" clears it
  Why: Tours will be resolved against it (743b), so a bare host:port or a path is refused rather than guessed at; mull never fetches it

### Changes by layer

**other**: 13 files, +74 −26
- M AGENTS.md (+1 −0)
- M mull/cmd/commands.go (+4 −3)
- M mull/docs/design.md (+9 −4)
- M mull/internal/guide/agents.md (+1 −0)
- M mull/internal/guide/intake.md (+1 −0)
- M mull/internal/guide/json.md (+5 −5)
- M mull/internal/guide/supervisor.md (+1 −0)
- M mull/internal/model/model.go (+4 −0)
- M mull/internal/mull/context.go (+14 −8)
- M mull/internal/mull/ops.go (+1 −1)
- M mull/internal/mull/revision.go (+1 −0)
- M mull/internal/mull/view.go (+7 −3)
- M mull/internal/mull/work.go (+25 −2)

**tests**: 2 files, +99 −3
- M mull/cmd/shapes_test.go (+15 −3)
- A mull/internal/mull/preview_test.go (+84 −0)
