---
kind: decision
text: A burst of file events is gathered for 150ms from its first event, then each touched repo mull branch is read again, and a project is announced only if its tip moved
plan: mull-serve
item: 35f5
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a20996ce714774813
created: "2026-11-10T23:17:05Z"
settled:
  action: accept
  note: with review e6da
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-10T23:59:20Z"
---

One mull write is a few events within milliseconds. Timing from the first event (not resetting on each) means a steady stream of writes still refreshes. Reading the tip filters out lock files, failed compare-and-swaps and pack-refs that change nothing
