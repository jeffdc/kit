---
plan: mull-serve
epic: 204f
title: 'Item stages: captured, committed, active, in review, done'
state: open
docs:
  - mull/docs/serve/
created: "2026-11-10T19:18:49Z"
updated: "2026-11-10T19:18:49Z"
---

Rename done to 'in review' and accepted to 'done' (a done item is a reviewed item). Add 'captured' (backlog, not committed) and 'committed'. start works from captured or committed (the fast path). Readiness: committed, in a phase whose needs are done, or unplanned.
