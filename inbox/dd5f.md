---
kind: decision
text: 'A Change is {type, id, title, plan?, phase?, kind?, change: added|changed|removed, state?, fields?: [{field, from?, to?}], at}: from/to only for short fields (state, title, plan, phase, claim worktree, settled action, needs), never for text (notes, summary, entry text, detail, docs); at is the newest commit since the look that touched it, and changes are newest first'
plan: mull-serve
item: 3deb
by: /Users/jeff/dev/kit/.claude/worktrees/agent-aaa30f2ae60d58a47
created: "2026-11-10T23:01:05Z"
settled:
  action: accept
  note: with review e6e3
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-10T23:59:20Z"
---

Enough for the revision schedule's what-changed column (committed to in-review, a question settled decide) without shipping text the UI reads from the view endpoints anyway
