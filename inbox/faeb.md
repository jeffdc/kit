---
kind: decision
text: Disk size is the total of the worktree's regular files (a Go walk, not du), in decimal units; the view and mull serve measure only worktrees that pass every other test and reuse a measurement of the same path and HEAD for five minutes, while mull worktrees measures every row afresh
plan: supervised-work
item: "1499"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a4ce3602e3548b182
created: "2026-11-22T17:34:23Z"
settled:
  action: accept
  by: Jeff
  human: true
  at: "2026-11-22T17:35:26Z"
---

mull serve reads the overview on every write, and walking gigabytes of dependencies each time would be too slow
