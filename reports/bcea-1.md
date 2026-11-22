---
item: bcea
"n": 1
passed: true
commit: 6d62d69ced9447416293bdb7b159729cd10aee1b
base: 03e9ce4452c8e6df85e23121114aac03f24c053a
at: "2026-11-22T16:42:29Z"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-ab627294f89289eee
summary: 'A plan may now add hard checks of its own: mull plan checks <slug> "<cmd>" ... replaces them (no commands clears them), kept in the plan''s record as checks and shown by plan show and plan list. mull done runs config''s checks, then the item''s plan''s, leaving out any command config already runs, as one list cached by tree and command list as before, so a config-only pass never stands in for the plan''s longer list. mull check --item <id> runs exactly what done would for that item, and mull check alone still runs config''s. With checks on neither config nor the plan, done still refuses without --override (the error now names plan checks too). batch has a plan checks op, the change feed reports a plan''s checks field (so mull serve''s revision list shows checks edited), and design.md, json.md and the agents guide (with kit''s AGENTS.md copy) are updated. This builds phase 8b6d''s second bullet beside f82a''s rules.'
tour:
  - Run make dev, then in a terminal cd mull/.dev/demo and use the mull binary make dev built (mull/mull), since the installed one predates this.
  - 'Run mull plan checks web ''true'' ''echo web e2e'': it prints the web plan with checks [true, echo web e2e].'
  - 'Run mull plan show web: the plan carries the same checks.'
  - 'Reload http://localhost:6899/#/demo: the revision list shows the web plan with checks edited.'
  - 'Back in the terminal, take a committed item in the web plan (mull ready --plan web), mull start it, and run mull check --item <id>: results list config''s true once, then echo web e2e (the plan''s copy of true isn''t run twice).'
  - 'Run mull check with no --item: only config''s true runs, and cached is false (a different command list).'
  - 'Run mull plan checks web with no commands: checks is gone from the printed plan.'
checks:
  tree: dd33359f16c6fa8840afb090674c8141340acc22
  commit: 6d62d69ced9447416293bdb7b159729cd10aee1b
  at: "2026-11-22T16:39:47Z"
  by: /Users/jeff/dev/kit/.claude/worktrees/agent-ab627294f89289eee
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 162097
changes:
  - layer: other
    files:
      - status: M
        path: AGENTS.md
        added: 1
        removed: 1
      - status: M
        path: mull/cmd/commands.go
        added: 14
        removed: 3
      - status: M
        path: mull/docs/design.md
        added: 9
        removed: 8
      - status: M
        path: mull/internal/guide/agents.md
        added: 1
        removed: 1
      - status: M
        path: mull/internal/guide/json.md
        added: 5
        removed: 3
      - status: M
        path: mull/internal/model/model.go
        added: 3
        removed: 0
      - status: M
        path: mull/internal/mull/done.go
        added: 88
        removed: 17
      - status: M
        path: mull/internal/mull/ops.go
        added: 16
        removed: 0
      - status: M
        path: mull/internal/mull/revision.go
        added: 1
        removed: 0
  - layer: tests
    files:
      - status: M
        path: mull/cmd/cmd_test.go
        added: 32
        removed: 0
      - status: M
        path: mull/cmd/shapes_test.go
        added: 3
        removed: 2
      - status: M
        path: mull/internal/mull/done_test.go
        added: 2
        removed: 2
      - status: A
        path: mull/internal/mull/plan_checks_test.go
        added: 120
        removed: 0
design: |-
  The plan's checks are a list field on the plan record (checks: in plans/<slug>.md frontmatter, model.Plan.Checks), not a file of their own like the doc and rules: they're a short list done reads, not prose with a history worth its own --log, and keeping them in the record means plan show, plan list, the serve API and the change feed carry them with no extra plumbing. The change feed reports them as a text field (no from/to), since commands can be long.

  In done.go, the old Check() split in two: hardChecks(cfg, plan) builds the command list (config's done_checks, then the plan's checks, skipping any already in config's, so make done-checks named in both never runs twice), and runChecks(commands) is the old body: clean tree, cached pass for the same tree and exact command list, run in order stopping at the first failure, record checks/<tree>.json. Done computes the list from the item's plan inside its existing read and calls runChecks; Check(itemID) does the same for mull check --item, and with no item it's config's alone, as before. The cache key is unchanged (tree plus whole list), so the item's note "cached by tree and command list as now" holds literally: a config-only pass on a tree doesn't count for a plan's longer list, and a check --item pass is reused by done. Both no-checks errors share one hint naming mull config checks and mull plan checks.

  mull plan checks <slug> [command...] mirrors mull config checks: each argument one command, trimmed, blanks dropped, none clears; an unchanged list writes nothing. It's a plain op (plan checks, fields slug and commands) so batch gets it, and the CLI prints the Plan. I didn't gate it with --on-request: config checks, plan doc and plan rules aren't gated either, and the protocol already says agents change plans only when asked. It works in any plan state, like doc and rules.

  Rejected: inferring the item for mull check from the worktree's active claim (implicit, and wrong with more than one claim); deduping or refusing duplicate commands when they're set rather than when they're run (config can change later, so it's only knowable at run time); showing the checks in mull serve or context --item, since neither shows config's checks today and the done report already lists every command that ran. Tests: internal/mull/plan_checks_test.go (order, dedupe, caching, plan scoping, unplanned items, a failing plan check keeping the item active, plan-only checks proving done, batch, change feed, clearing), a CLI test in cmd/cmd_test.go, and plan checks / check --item added to the shapes test.
---

# Done report 1: A plan's own hard checks (bcea)

**Status:** PASSED  
**Commit:** 6d62d69ced94  
**By:** /Users/jeff/dev/kit/.claude/worktrees/agent-ab627294f89289eee at 2026-11-22T16:42:29Z

## What was built

### Summary

A plan may now add hard checks of its own: mull plan checks <slug> "<cmd>" ... replaces them (no commands clears them), kept in the plan's record as checks and shown by plan show and plan list. mull done runs config's checks, then the item's plan's, leaving out any command config already runs, as one list cached by tree and command list as before, so a config-only pass never stands in for the plan's longer list. mull check --item <id> runs exactly what done would for that item, and mull check alone still runs config's. With checks on neither config nor the plan, done still refuses without --override (the error now names plan checks too). batch has a plan checks op, the change feed reports a plan's checks field (so mull serve's revision list shows checks edited), and design.md, json.md and the agents guide (with kit's AGENTS.md copy) are updated. This builds phase 8b6d's second bullet beside f82a's rules.

### UI tour

1. Run make dev, then in a terminal cd mull/.dev/demo and use the mull binary make dev built (mull/mull), since the installed one predates this.
2. Run mull plan checks web 'true' 'echo web e2e': it prints the web plan with checks [true, echo web e2e].
3. Run mull plan show web: the plan carries the same checks.
4. Reload http://localhost:6899/#/demo: the revision list shows the web plan with checks edited.
5. Back in the terminal, take a committed item in the web plan (mull ready --plan web), mull start it, and run mull check --item <id>: results list config's true once, then echo web e2e (the plan's copy of true isn't run twice).
6. Run mull check with no --item: only config's true runs, and cached is false (a different command list).
7. Run mull plan checks web with no commands: checks is gone from the printed plan.

### Checks (run by mull)

- `make done-checks`: pass, 162.1s

## How it was built

### Design notes

The plan's checks are a list field on the plan record (checks: in plans/<slug>.md frontmatter, model.Plan.Checks), not a file of their own like the doc and rules: they're a short list done reads, not prose with a history worth its own --log, and keeping them in the record means plan show, plan list, the serve API and the change feed carry them with no extra plumbing. The change feed reports them as a text field (no from/to), since commands can be long.

In done.go, the old Check() split in two: hardChecks(cfg, plan) builds the command list (config's done_checks, then the plan's checks, skipping any already in config's, so make done-checks named in both never runs twice), and runChecks(commands) is the old body: clean tree, cached pass for the same tree and exact command list, run in order stopping at the first failure, record checks/<tree>.json. Done computes the list from the item's plan inside its existing read and calls runChecks; Check(itemID) does the same for mull check --item, and with no item it's config's alone, as before. The cache key is unchanged (tree plus whole list), so the item's note "cached by tree and command list as now" holds literally: a config-only pass on a tree doesn't count for a plan's longer list, and a check --item pass is reused by done. Both no-checks errors share one hint naming mull config checks and mull plan checks.

mull plan checks <slug> [command...] mirrors mull config checks: each argument one command, trimmed, blanks dropped, none clears; an unchanged list writes nothing. It's a plain op (plan checks, fields slug and commands) so batch gets it, and the CLI prints the Plan. I didn't gate it with --on-request: config checks, plan doc and plan rules aren't gated either, and the protocol already says agents change plans only when asked. It works in any plan state, like doc and rules.

Rejected: inferring the item for mull check from the worktree's active claim (implicit, and wrong with more than one claim); deduping or refusing duplicate commands when they're set rather than when they're run (config can change later, so it's only knowable at run time); showing the checks in mull serve or context --item, since neither shows config's checks today and the done report already lists every command that ran. Tests: internal/mull/plan_checks_test.go (order, dedupe, caching, plan scoping, unplanned items, a failing plan check keeping the item active, plan-only checks proving done, batch, change feed, clearing), a CLI test in cmd/cmd_test.go, and plan checks / check --item added to the shapes test.

### Decisions made in the session

None recorded.

### Changes by layer

**other**: 9 files, +138 −33
- M AGENTS.md (+1 −1)
- M mull/cmd/commands.go (+14 −3)
- M mull/docs/design.md (+9 −8)
- M mull/internal/guide/agents.md (+1 −1)
- M mull/internal/guide/json.md (+5 −3)
- M mull/internal/model/model.go (+3 −0)
- M mull/internal/mull/done.go (+88 −17)
- M mull/internal/mull/ops.go (+16 −0)
- M mull/internal/mull/revision.go (+1 −0)

**tests**: 4 files, +157 −4
- M mull/cmd/cmd_test.go (+32 −0)
- M mull/cmd/shapes_test.go (+3 −2)
- M mull/internal/mull/done_test.go (+2 −2)
- A mull/internal/mull/plan_checks_test.go (+120 −0)
