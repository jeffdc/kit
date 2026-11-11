---
kind: decision
text: make bench reads this repository's real mull branch (read only) as well as the seeded one, with per-machine state pointed at a temp dir
plan: mull-data
item: dafa
by: /Users/jeff/dev/kit
created: "2026-11-11T14:42:39Z"
settled:
  action: accept
  note: with review 0cb6
  by: jeffdc
  human: true
  at: "2026-11-11T23:19:08Z"
---

The real branch is the baseline the human feels; nothing is written to it, and seen/registry go to a disposable XDG_CONFIG_HOME.
