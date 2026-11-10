---
kind: decision
text: The UI is embedded from internal/serve/web as plain files (index.html, app.js module, style.css) served with Cache-Control no-cache; the registry is re-read on every API request
plan: mull-serve
item: 9cb0
by: /Users/jeff/dev/kit
created: "2026-11-10T22:38:26Z"
---

No build step, a new binary's files are picked up at once, and repos registered while serve runs appear without a restart
