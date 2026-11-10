---
kind: decision
text: 'History is a --log flag on plan doc / phase doc: {type, id, versions: [{commit, at, doc}]}, newest first, one per commit that changed the doc file (doc empty where it was removed), read by a new store.History(path)'
plan: mull-serve
item: af14
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a1acf98c9b6a6892a
created: "2026-11-10T20:53:20Z"
---

History from the commits is the point of the separate file: the UI (482f) and agents need versions without reading the mull branch themselves, so a small read is worth it. Each version carries its full text so diffs can be made by the reader; no diff output in mull
