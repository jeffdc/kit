---
plan: supervised-work
phase: 13ee
rank: 10
title: A plan's target branch, with seen in git measured against it
stage: committed
created: "2026-11-22T15:37:02Z"
updated: "2026-11-22T15:37:02Z"
---

A plan field target (mull plan set <slug> target <branch>), falling back to config integration. Seen in git is measured per plan against its target, and the merge time is derived from the commit that brought the work in, never stored. view (JSON and --text), context and mull serve show each plan's target and how far it is ahead of release. Measure with make bench before and after: this adds per-plan reachability work.

Update docs/design.md, the guides (internal/guide) and mull guide json wherever behaviour or output changes. The plan's doc (mull plan doc supervised-work) has the whole picture and what stays out of mull.
