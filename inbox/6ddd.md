---
kind: decision
text: do --reason on an integration review creates a committed fix-up item, ranked last in the phase, titled by the reason's first line (cut to 80 characters) with the full reason in its notes and from set to the review
plan: supervised-work
item: 8e3e
by: /Users/jeff/dev/kit/.claude/worktrees/agent-ab6d36b56640e3c63
created: "2026-11-22T18:08:11Z"
settled:
  action: accept
  note: with review 038a
  by: Jeff
  human: true
  at: "2026-11-23T13:55:38Z"
---

The reason says what to fix, so it's the best title; --plan or --phase must agree with the review's phase, and --as phase is refused
