---
plan: mull-serve
epic: 204f
title: 'Item stages: captured, committed, active, in review, done'
state: done
docs:
  - mull/docs/serve/
claim:
  worktree: /Users/jeff/dev/kit
  branch: main
  at: "2026-11-10T19:46:13Z"
  base: d3db7357e15a3b445cbdbceb899c7fdbcc8b8454
created: "2026-11-10T19:18:49Z"
updated: "2026-11-10T19:50:06Z"
---

Rename done to 'in review' and accepted to 'done' (a done item is a reviewed item). Add 'captured' (backlog, not committed) and 'committed'. start works from captured or committed (the fast path). Readiness: committed, in a phase whose needs are done, or unplanned.
