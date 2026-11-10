---
plan: mull-serve
phase: 482f
title: 'Last looked and revisions: what changed since you last looked'
stage: in-review
docs:
  - mull/docs/serve/
claim:
  worktree: /Users/jeff/dev/kit/.claude/worktrees/agent-aaa30f2ae60d58a47
  branch: 3deb-last-looked
  at: "2026-11-10T22:47:48Z"
  base: 5f1df6b2006a11f622f68c6c9ebf484deeb3d996
created: "2026-11-10T19:18:49Z"
updated: "2026-11-10T23:02:24Z"
---

mull serve keeps per-project last-seen state in ~/.config/mull/ (never on the mull branch). The API marks records changed since then (from the mull branch history) and lists them as the revision schedule; POST marks a project seen.
