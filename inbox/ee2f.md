---
kind: decision
text: 'Docs render through the vendored marked with raw HTML shown as text; only http(s), mailto and # links stay links, other targets (file paths, javascript:) become dotted text with the target as a tooltip, and images become their alt text'
plan: mull-serve
item: f31c
by: /Users/jeff/dev/kit
created: "2026-11-11T00:50:37Z"
---

Docs are written by agents and the page can POST to mull serve; nothing in a doc should run or navigate somewhere unexpected
