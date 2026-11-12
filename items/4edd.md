---
title: mull set accepts an empty title for an item, plan or phase
stage: captured
kind: bug
category: cli
source: item a0e3
created: "2026-11-12T00:52:59Z"
updated: "2026-11-12T00:52:59Z"
---

Seen while building a0e3: Set (internal/mull/work.go) assigns title with no check, so mull set <id> title "" (or a batch line) leaves a record with no name. mull serve's title editor won't send an empty title, but mull itself should refuse it as it refuses an empty title on add.
