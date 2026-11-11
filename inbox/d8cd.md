---
kind: decision
text: 'The UI''s routes are hash routes mirroring the API''s paths: #/<project>, #/<project>/plans/<slug>, /phases/<id>, /items/<id>, /unplanned, /backlog'
plan: mull-serve
item: e3f5
by: /Users/jeff/dev/kit
created: "2026-11-11T00:20:09Z"
settled:
  action: accept
  note: with review decd
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-11T00:42:24Z"
---

No server change needed (the file server can't answer deep paths), and the URL reads like the endpoint it shows
