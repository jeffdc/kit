---
kind: decision
text: New dependency github.com/fsnotify/fsnotify v1.9.0 (with golang.org/x/sys) for the live-update watch
plan: mull-serve
item: 35f5
by: /Users/jeff/dev/kit/.claude/worktrees/agent-a20996ce714774813
created: "2026-11-10T23:16:56Z"
settled:
  action: accept
  note: with review e6da
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-10T23:59:20Z"
---

The item names it. It is the standard Go file watcher (kqueue on macOS, inotify on Linux), and polling each repo ref would cost a git call per repo per tick
