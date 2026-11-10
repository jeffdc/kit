---
kind: decision
text: GET /api/projects/{id}/revision is {id, tip, since, baseline?, changes, seen_at}; POST /api/projects/{id}/seen takes an optional {commit} (the revision the page showed; the tip if absent), which must be the tip or an ancestor of it, and returns the same shape
plan: mull-serve
item: 3deb
by: /Users/jeff/dev/kit/.claude/worktrees/agent-aaa30f2ae60d58a47
created: "2026-11-10T23:01:06Z"
---

Marking the shown commit rather than the tip means a write landing between rendering and clicking stays clouded. Marking an earlier commit is allowed (it re-clouds), since it is only the human's own local state
