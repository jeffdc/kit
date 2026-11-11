---
plan: mull-serve
phase: e7cb
title: 'Not on mull yet: list recently worked-on repos beside the registered ones that aren''t on mull, and repos with old .mull/ data that need converting'
stage: committed
kind: idea
category: ui
source: human, 2026-11-11, reviewing the prototype's rail foot
created: "2026-11-11T01:24:13Z"
updated: "2026-11-11T01:26:48Z"
---

The prototype's lower-left "Not on mull yet" list, made real (human, 2026-11-11). Agreed shape:
- Where to look: the parent directories of the registered repos (today ~/dev), one level deep. No configuration.
- What's listed: repos with old .mull/ data, always, as "Needs converting" (their intake: mull guide intake); other git repos with no mull branch only if they've had a commit in the last 60 days, as "Not on mull".
- Where it shows: the sheet index's foot, in those two groups, and rows at the bottom of All projects saying what to do (mull init, or the intake).
- mull serve reads the directories now and then; it never watches or writes them. A new API endpoint (document it in mull guide json).
Do it after the UI phase (e7cb) is finished.
