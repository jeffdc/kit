---
item: f7ca
"n": 1
passed: true
commit: d45bfdeda9d50d6c4429bb1f42efe0d21538b975
base: ba80c8b5299260d8ec908866ea7a043886642115
at: "2026-11-15T19:13:48Z"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-ae1ab27c30f2363f1
summary: 'Find in mull serve is now scoped and filtered. On any sheet in a project it searches only that project; a scope control in the palette''s head reads the project''s ID and ''Every project'', switched with ⌥P or a click (off a project, on All projects or the model, it searches every project and offers no scope). Tab and Shift+Tab cycle a status filter, Open → Finished → All, shown as a segmented control beside it and clickable: Open is live work (items not done or dropped, plans and phases not done, entries open or parked), Finished is done, dropped and settled records and the whole Archive. It replaces ''Search the archive too''; the foot says how many matches the filter left out (''3 more finished'' under Open, ''N more open'' under Finished). Focus is trapped in the palette. GET /api/search gains status= and project= and filters on the server (limit and counts right), returning open and finished counts and a finished flag per hit; phase hits now carry their state. On Jeff''s question whether both done and archived are needed: the model''s states are unchanged, and Find collapses them into Finished (archived hits still say ''archive''). Bench (large repo, before → after): search 11.3 → 10.9 ms in-process, 23.3 → 22.8 ms over HTTP; whole-branch load 243.2 → 242.0 ms; no page moved beyond noise.'
tour:
  - make dev from mull/, open http://localhost:6899/#/demo and press s.
  - 'Type a: the head shows Open | Finished | All (Open pressed) and demo | Every project (demo pressed); hits are demo''s open work with no project name in their muted line, and the foot ends ''3 more finished''.'
  - 'Press Tab: Finished is pressed; you see Add a favicon (DONE, ''item · archive''), its review (ACCEPTED, archive) and Rename the settings page (DROPPED, archive), and the foot says ''13 more open''.'
  - 'Press Tab again: All, 16 hits and no count in the foot. Shift+Tab goes back to Finished. Focus never leaves the input.'
  - 'Press ⌥P: Every project is pressed and each hit''s muted line starts with its project (demo). ⌥P again goes back to demo. Clicking the buttons does the same and leaves the cursor in the input.'
  - 'Type zzzz: ''Nothing finished in demo has that ID or those words in its title.'' Esc closes.'
  - 'Open All projects (#/) or The model and press s: no scope control, and the foot says ''every project''.'
  - 'Open The model: the ''Finished work is archived'' rule now says Find shows open work first and Tab turns to Finished and All.'
checks:
  tree: c341ffb32d78e62bece37ffb7f748277d51cd559
  commit: d45bfdeda9d50d6c4429bb1f42efe0d21538b975
  at: "2026-11-15T19:13:41Z"
  by: /Users/jeff/dev/kit/.claude/worktrees/agent-ae1ab27c30f2363f1
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 6061
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/design.md
        added: 1
        removed: 1
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 5
        removed: 3
      - status: M
        path: mull/internal/bench/bench.go
        added: 1
        removed: 1
      - status: M
        path: mull/internal/guide/json.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/mull/search.go
        added: 45
        removed: 18
      - status: M
        path: mull/internal/serve/search.go
        added: 42
        removed: 14
      - status: M
        path: mull/internal/serve/web/api.js
        added: 4
        removed: 3
      - status: M
        path: mull/internal/serve/web/app.js
        added: 8
        removed: 1
      - status: M
        path: mull/internal/serve/web/model.js
        added: 1
        removed: 1
      - status: M
        path: mull/internal/serve/web/search.js
        added: 88
        removed: 28
      - status: M
        path: mull/internal/serve/web/style.css
        added: 7
        removed: 2
  - layer: tests
    files:
      - status: M
        path: mull/internal/mull/aging_test.go
        added: 9
        removed: 2
      - status: M
        path: mull/internal/mull/archive_test.go
        added: 7
        removed: 5
      - status: M
        path: mull/internal/mull/search_test.go
        added: 103
        removed: 12
      - status: M
        path: mull/internal/serve/search_test.go
        added: 62
        removed: 1
design: |-
  Server first. mull.Search's archive bool became a status string (open, finished, all; "" is open, anything else invalid_argument). searchDocs, built once per snapshot in index(), now sets Hit.Finished beside Hit.Archived: archived, or a done/dropped item (finished()), a done plan, a done phase (phaseState, so phase hits also carry done/now/waits for their tag), or an entry settled by anything but park. Parked entries stay Open because they still wait for Jeff, matching held(), which already keeps parked entries' items out of the Archive. Search counts every match in each status before filtering and limiting, so Found carries open and finished totals instead of the old archived count. Ranking is untouched.

  GET /api/search parses status (400 on an unknown one) and an optional project (404 if no registered project has that ID), narrows paths to that one project before fanning out, and sums the per-project counts. Filtering on the server keeps the 30-hit limit and the counts honest for one project. archive=1 and the archived count are gone: the UI was their only reader and json.md had no row for the endpoint, so it now has one.

  UI: app.js gives search.js a scope function (findScope) that returns the route's project when it's a readable project, else null, so search.js doesn't need to know the registry. Each opening starts on that project and on Open. The filters are two .seg controls on a line under the input in the Panel Grey head (24px buttons, out of the tab order; mousedown is prevented so a click doesn't steal focus). The palette's keydown handles Tab (cycle when the input has focus, else return focus to the input), ⌥P by ev.code so the π it would type is suppressed, Esc from anywhere in the palette, and arrows/Enter in the input. A focusin listener added on open and removed on close pulls focus back if it leaves the band. The foot is a flex row: keys left, the left-out count right in tabular figures. The project name is dropped from hits' muted lines when scoped, as it would repeat on every row.

  Rejected: putting the filters on the input's line (too cramped at 680px, worse on a phone); making the count a button (the status control is right above it); counting matches in other projects while scoped (it would mean searching every project on every keystroke for a hint); ⌥A for scope (it reads as the status filter's All); keeping archive=1 as an alias (nothing else calls it).

  Trade-off recorded: with Tab cycling the filter, the hits' copy-ID buttons in Find are pointer-only now; idea e2b9 proposes a key to copy the selected hit's ID.

  Checked by driving headless Chromium over CDP against make dev: the Tab/Shift+Tab cycle, ⌥P, clicks keeping focus, the focus trap, empty states, All projects and the model with no scope control, and 400px width. Tests: TestSearchStatus (mull) covers each filter, counts, archived vs finished, parked entries, phase states and the invalid status; TestSearchEndpointScopeAndStatus (serve) covers project scope, status, counts, and 404/400. The archive and aging tests moved to the new filter.
decisions:
  - id: bb32
    text: 'Find''s Open filter is live work: items not done or dropped (captured included), plans and phases not done, and inbox entries open or parked; Finished is everything else: done and dropped items, done phases and plans, entries settled other than by parking, and the whole Archive whatever a record''s own stage'
    detail: Parked entries stay Open because they still wait for Jeff, as archivedUnplanned's held() already treats them; done work in an open plan is Finished, not archived, and keeps its done/dropped tag
  - id: "2179"
    text: Find's scope key is ⌥P (P for project), matched by key position so it never types π; A would read as the status filter's All
    detail: 'Scope and status both reset each time Find opens: the sheet''s project and Open'
  - id: 3c6b
    text: GET /api/search takes status=open|finished|all (default open) and project=<id>, filters on the server, and returns {query, status, project?, hits, open, finished}; archive=1 and the archived count are gone, and each hit gains finished
    detail: open and finished count every match whatever the filter, so the foot can say 'N more finished' (under Open) or 'N more open' (under Finished); an unknown status is 400, an unknown project 404
  - id: "9516"
    text: Phase hits in Find carry their state (done, now, waits) and show its tag, and a done phase counts as Finished
    detail: Before, phases had no state in search, so a finished phase couldn't be told apart or filtered
  - id: aab8
    text: 'Find traps focus: Tab and Shift+Tab in the input cycle the status, Tab from anywhere else in the palette returns to the input, and focus that leaves the palette is pulled back; the filter buttons are out of the tab order and don''t take focus on click'
    detail: So the hits' copy-ID buttons are now pointer-only in Find (an idea is captured for a key to copy the selected hit's ID)
  - id: de1b
    text: Searching one project, a hit's muted line leaves the project name out
    detail: It's the same on every row then; it's still shown when Find searches every project
  - id: 2ab9
    text: Find collapses done and archived into one Finished filter; the model's states and the derived Archive are unchanged
    detail: Jeff's answer to 'do we need both done and archived?' for the UI; an archived hit still says archive in its muted line, and the sheet index's Archive is as it was
---

# Done report 1: Search Needs to be Smarter (f7ca)

**Status:** PASSED  
**Commit:** d45bfdeda9d5  
**By:** /Users/jeff/dev/kit/.claude/worktrees/agent-ae1ab27c30f2363f1 at 2026-11-15T19:13:48Z

## What was built

### Summary

Find in mull serve is now scoped and filtered. On any sheet in a project it searches only that project; a scope control in the palette's head reads the project's ID and 'Every project', switched with ⌥P or a click (off a project, on All projects or the model, it searches every project and offers no scope). Tab and Shift+Tab cycle a status filter, Open → Finished → All, shown as a segmented control beside it and clickable: Open is live work (items not done or dropped, plans and phases not done, entries open or parked), Finished is done, dropped and settled records and the whole Archive. It replaces 'Search the archive too'; the foot says how many matches the filter left out ('3 more finished' under Open, 'N more open' under Finished). Focus is trapped in the palette. GET /api/search gains status= and project= and filters on the server (limit and counts right), returning open and finished counts and a finished flag per hit; phase hits now carry their state. On Jeff's question whether both done and archived are needed: the model's states are unchanged, and Find collapses them into Finished (archived hits still say 'archive'). Bench (large repo, before → after): search 11.3 → 10.9 ms in-process, 23.3 → 22.8 ms over HTTP; whole-branch load 243.2 → 242.0 ms; no page moved beyond noise.

### UI tour

1. make dev from mull/, open http://localhost:6899/#/demo and press s.
2. Type a: the head shows Open | Finished | All (Open pressed) and demo | Every project (demo pressed); hits are demo's open work with no project name in their muted line, and the foot ends '3 more finished'.
3. Press Tab: Finished is pressed; you see Add a favicon (DONE, 'item · archive'), its review (ACCEPTED, archive) and Rename the settings page (DROPPED, archive), and the foot says '13 more open'.
4. Press Tab again: All, 16 hits and no count in the foot. Shift+Tab goes back to Finished. Focus never leaves the input.
5. Press ⌥P: Every project is pressed and each hit's muted line starts with its project (demo). ⌥P again goes back to demo. Clicking the buttons does the same and leaves the cursor in the input.
6. Type zzzz: 'Nothing finished in demo has that ID or those words in its title.' Esc closes.
7. Open All projects (#/) or The model and press s: no scope control, and the foot says 'every project'.
8. Open The model: the 'Finished work is archived' rule now says Find shows open work first and Tab turns to Finished and All.

### Checks (run by mull)

- `make done-checks`: pass, 6.1s

## How it was built

### Design notes

Server first. mull.Search's archive bool became a status string (open, finished, all; "" is open, anything else invalid_argument). searchDocs, built once per snapshot in index(), now sets Hit.Finished beside Hit.Archived: archived, or a done/dropped item (finished()), a done plan, a done phase (phaseState, so phase hits also carry done/now/waits for their tag), or an entry settled by anything but park. Parked entries stay Open because they still wait for Jeff, matching held(), which already keeps parked entries' items out of the Archive. Search counts every match in each status before filtering and limiting, so Found carries open and finished totals instead of the old archived count. Ranking is untouched.

GET /api/search parses status (400 on an unknown one) and an optional project (404 if no registered project has that ID), narrows paths to that one project before fanning out, and sums the per-project counts. Filtering on the server keeps the 30-hit limit and the counts honest for one project. archive=1 and the archived count are gone: the UI was their only reader and json.md had no row for the endpoint, so it now has one.

UI: app.js gives search.js a scope function (findScope) that returns the route's project when it's a readable project, else null, so search.js doesn't need to know the registry. Each opening starts on that project and on Open. The filters are two .seg controls on a line under the input in the Panel Grey head (24px buttons, out of the tab order; mousedown is prevented so a click doesn't steal focus). The palette's keydown handles Tab (cycle when the input has focus, else return focus to the input), ⌥P by ev.code so the π it would type is suppressed, Esc from anywhere in the palette, and arrows/Enter in the input. A focusin listener added on open and removed on close pulls focus back if it leaves the band. The foot is a flex row: keys left, the left-out count right in tabular figures. The project name is dropped from hits' muted lines when scoped, as it would repeat on every row.

Rejected: putting the filters on the input's line (too cramped at 680px, worse on a phone); making the count a button (the status control is right above it); counting matches in other projects while scoped (it would mean searching every project on every keystroke for a hint); ⌥A for scope (it reads as the status filter's All); keeping archive=1 as an alias (nothing else calls it).

Trade-off recorded: with Tab cycling the filter, the hits' copy-ID buttons in Find are pointer-only now; idea e2b9 proposes a key to copy the selected hit's ID.

Checked by driving headless Chromium over CDP against make dev: the Tab/Shift+Tab cycle, ⌥P, clicks keeping focus, the focus trap, empty states, All projects and the model with no scope control, and 400px width. Tests: TestSearchStatus (mull) covers each filter, counts, archived vs finished, parked entries, phase states and the invalid status; TestSearchEndpointScopeAndStatus (serve) covers project scope, status, counts, and 404/400. The archive and aging tests moved to the new filter.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **bb32** Find's Open filter is live work: items not done or dropped (captured included), plans and phases not done, and inbox entries open or parked; Finished is everything else: done and dropped items, done phases and plans, entries settled other than by parking, and the whole Archive whatever a record's own stage
  Why: Parked entries stay Open because they still wait for Jeff, as archivedUnplanned's held() already treats them; done work in an open plan is Finished, not archived, and keeps its done/dropped tag
- **2179** Find's scope key is ⌥P (P for project), matched by key position so it never types π; A would read as the status filter's All
  Why: Scope and status both reset each time Find opens: the sheet's project and Open
- **3c6b** GET /api/search takes status=open|finished|all (default open) and project=<id>, filters on the server, and returns {query, status, project?, hits, open, finished}; archive=1 and the archived count are gone, and each hit gains finished
  Why: open and finished count every match whatever the filter, so the foot can say 'N more finished' (under Open) or 'N more open' (under Finished); an unknown status is 400, an unknown project 404
- **9516** Phase hits in Find carry their state (done, now, waits) and show its tag, and a done phase counts as Finished
  Why: Before, phases had no state in search, so a finished phase couldn't be told apart or filtered
- **aab8** Find traps focus: Tab and Shift+Tab in the input cycle the status, Tab from anywhere else in the palette returns to the input, and focus that leaves the palette is pulled back; the filter buttons are out of the tab order and don't take focus on click
  Why: So the hits' copy-ID buttons are now pointer-only in Find (an idea is captured for a key to copy the selected hit's ID)
- **de1b** Searching one project, a hit's muted line leaves the project name out
  Why: It's the same on every row then; it's still shown when Find searches every project
- **2ab9** Find collapses done and archived into one Finished filter; the model's states and the derived Archive are unchanged
  Why: Jeff's answer to 'do we need both done and archived?' for the UI; an archived hit still says archive in its muted line, and the sheet index's Archive is as it was

### Changes by layer

**other**: 11 files, +203 −72
- M mull/docs/design.md (+1 −1)
- M mull/docs/serve/DESIGN.md (+5 −3)
- M mull/internal/bench/bench.go (+1 −1)
- M mull/internal/guide/json.md (+1 −0)
- M mull/internal/mull/search.go (+45 −18)
- M mull/internal/serve/search.go (+42 −14)
- M mull/internal/serve/web/api.js (+4 −3)
- M mull/internal/serve/web/app.js (+8 −1)
- M mull/internal/serve/web/model.js (+1 −1)
- M mull/internal/serve/web/search.js (+88 −28)
- M mull/internal/serve/web/style.css (+7 −2)

**tests**: 4 files, +181 −20
- M mull/internal/mull/aging_test.go (+9 −2)
- M mull/internal/mull/archive_test.go (+7 −5)
- M mull/internal/mull/search_test.go (+103 −12)
- M mull/internal/serve/search_test.go (+62 −1)
