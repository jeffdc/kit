---
kind: decision
text: 'migrate output: {dry_run, format_before, format, changed, commit?, counts, records: [{type,id,path,from?,changes}], captured, needs, warnings}, with --text for the human; it is not a batch op'
plan: mull-serve
item: 488d
by: /Users/jeff/dev/kit/.claude/worktrees/agent-ae1995ea8ca4c288d
created: "2026-11-10T22:08:35Z"
---

Counts plus every record with its changes, and the converted loose ends listed for re-kinding; documented in guide json and enforced by shapes_test.
