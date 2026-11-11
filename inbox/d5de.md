---
kind: decision
text: The touched-paths walk is kept per last-looked commit and extended when the tip moves on from the kept one; a path's time is the newest commit time across both walks
plan: mull-data
item: 4bf3
by: /Users/jeff/dev/kit
created: "2026-11-11T15:17:49Z"
---

A write then costs a walk of the new commits only. Taking the newest time matches the old first-in-log rule except under clock skew between commits.
