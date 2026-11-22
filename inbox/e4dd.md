---
kind: decision
text: config checks and plan checks are refused without --on-request even when the list is unchanged or being cleared; plan checks reports an unknown plan (not_found) before the gate
plan: supervised-work
item: 73ba
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a62093bdce80ca670
created: "2026-11-22T17:05:14Z"
---

any write to the checks is a change to what judges agents' work, and not_found first matches plan agree's order
