---
kind: decision
text: mull serve keeps an unmulled scan for 5 minutes, or until the set of directories changes, and scans on the next request after that; the page fetches it with the project list
plan: mull-serve
item: a5ac
by: /Users/jeff/dev/kit
created: "2026-11-11T01:41:31Z"
settled:
  action: accept
  note: with review e83b
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-11T01:48:16Z"
---

About 30 repos and a few git calls each: cheap enough on demand, and no watching of directories mull doesn't own
