---
plan: mull-serve
phase: 482f
title: 'Last looked and revisions: what changed since you last looked'
stage: committed
docs:
  - mull/docs/serve/
created: "2026-11-10T19:18:49Z"
updated: "2026-11-10T19:18:49Z"
---

mull serve keeps per-project last-seen state in ~/.config/mull/ (never on the mull branch). The API marks records changed since then (from the mull branch history) and lists them as the revision schedule; POST marks a project seen.
