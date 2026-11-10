---
kind: decision
text: 'add keeps a hidden --needs flag (and the batch field) only to refuse it with a message pointing at phase needs; need/unneed results are always {type: phase, phase}'
plan: mull-serve
item: 7bf0
by: /Users/jeff/dev/kit/.claude/worktrees/agent-aff50ef581ecde344
created: "2026-11-10T20:31:52Z"
settled:
  action: accept
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-10T20:42:45Z"
---

Dropping the flag would fail with a generic unknown-flag usage error; agents with old guides get a clear reason instead. The type field is kept so the result shape stays compatible.
