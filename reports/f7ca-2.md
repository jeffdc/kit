---
item: f7ca
"n": 2
passed: true
commit: 4a9e16a10d65d191c3777075a6cb2391a7739ce3
base: 3d11f51ad207dfb24f1dc493b02e9b63f214d5e7
at: "2026-11-15T19:31:03Z"
by: /Users/jeff/dev/kit
summary: 'Taken in from review: Find''s scope key ⌥P (which typed π) is replaced by the arrow keys, as Jeff asked. In Find''s input on a project''s sheet, ← picks that project and → every project; ⌥/⌘/Shift arrows still move or select in the text. Everything else in f7ca is as first reviewed: Find defaults to the sheet''s project, and Tab cycles Open → Finished → All.'
tour:
  - 'make install, open kit''s Overview and press s; type a few letters: the scope control shows kit pressed and the foot reads ''←→ scope''.'
  - 'Press →: Every project is pressed and each hit''s muted line starts with its project. Press → again: nothing changes. Press ←: kit again.'
  - 'Press ⌥←: the caret jumps to the start of the query and the scope stays kit. Shift+→ selects a letter.'
  - Tab still cycles Open → Finished → All.
  - 'On All projects, press s and type: no scope control, and ← and → move the caret as usual.'
checks:
  tree: b4a6cee25ed39c53edbd02dd52e713df4a1ae3ac
  commit: 4a9e16a10d65d191c3777075a6cb2391a7739ce3
  at: "2026-11-15T19:30:42Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 20493
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 1
        removed: 1
      - status: M
        path: mull/internal/serve/web/search.js
        added: 11
        removed: 7
design: |-
  Jeff decided 2179 (⌥P for the scope) differently: it typed π, or ∏ with Shift, and he asked for the arrow keys.

  The palette's keydown handler takes plain ArrowLeft and ArrowRight from the input when Find opened on a project (home is set): ← sets scoped = true, → sets it false, and it searches again only if that changed. They pick a side rather than toggle, so they match the segmented control's layout (the project on the left, Every project on the right) and pressing one twice is harmless. Any modifier (⌥, ⌘, Ctrl, Shift) falls through, so word and line jumps and selection still work; off a project there's no scope, and the arrows are plain caret keys. The foot reads '←→ scope' and the control's label and title name the keys. DESIGN.md's Find Scope line says so and records why.

  Rejected: switching only when the caret is at the start or end of the query (the same key would do two things depending on an invisible caret position); a different ⌥ letter (any ⌥ letter types a character on a Mac keyboard layout, which is what went wrong).

  Checked in headless Chromium against make dev: → and ← switch the scope with the caret unmoved, ⌥← moves the caret without changing scope, and on All projects ← moves the caret.
decisions:
  - id: bb32
    text: 'Find''s Open filter is live work: items not done or dropped (captured included), plans and phases not done, and inbox entries open or parked; Finished is everything else: done and dropped items, done phases and plans, entries settled other than by parking, and the whole Archive whatever a record''s own stage'
    detail: Parked entries stay Open because they still wait for Jeff, as archivedUnplanned's held() already treats them; done work in an open plan is Finished, not archived, and keeps its done/dropped tag
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
  - id: 4df0
    text: 'Find''s scope takes plain ← and → in the input: ← picks the sheet''s project, → every project (not a toggle, matching the control''s left and right); ⌥P is gone. With ⌥, ⌘ or Shift the arrows still move or select in the text, and off a project they''re ordinary caret keys'
    detail: 'Jeff decided 2179 differently on review: ⌥P typed π and he asked for the arrow keys; giving up plain caret moves in a one-line query is the cost, and the modified arrows keep them'
---

# Done report 2: Search Needs to be Smarter (f7ca)

**Status:** PASSED  
**Commit:** 4a9e16a10d65  
**By:** /Users/jeff/dev/kit at 2026-11-15T19:31:03Z

## What was built

### Summary

Taken in from review: Find's scope key ⌥P (which typed π) is replaced by the arrow keys, as Jeff asked. In Find's input on a project's sheet, ← picks that project and → every project; ⌥/⌘/Shift arrows still move or select in the text. Everything else in f7ca is as first reviewed: Find defaults to the sheet's project, and Tab cycles Open → Finished → All.

### UI tour

1. make install, open kit's Overview and press s; type a few letters: the scope control shows kit pressed and the foot reads '←→ scope'.
2. Press →: Every project is pressed and each hit's muted line starts with its project. Press → again: nothing changes. Press ←: kit again.
3. Press ⌥←: the caret jumps to the start of the query and the scope stays kit. Shift+→ selects a letter.
4. Tab still cycles Open → Finished → All.
5. On All projects, press s and type: no scope control, and ← and → move the caret as usual.

### Checks (run by mull)

- `make done-checks`: pass, 20.5s

## How it was built

### Design notes

Jeff decided 2179 (⌥P for the scope) differently: it typed π, or ∏ with Shift, and he asked for the arrow keys.

The palette's keydown handler takes plain ArrowLeft and ArrowRight from the input when Find opened on a project (home is set): ← sets scoped = true, → sets it false, and it searches again only if that changed. They pick a side rather than toggle, so they match the segmented control's layout (the project on the left, Every project on the right) and pressing one twice is harmless. Any modifier (⌥, ⌘, Ctrl, Shift) falls through, so word and line jumps and selection still work; off a project there's no scope, and the arrows are plain caret keys. The foot reads '←→ scope' and the control's label and title name the keys. DESIGN.md's Find Scope line says so and records why.

Rejected: switching only when the caret is at the start or end of the query (the same key would do two things depending on an invisible caret position); a different ⌥ letter (any ⌥ letter types a character on a Mac keyboard layout, which is what went wrong).

Checked in headless Chromium against make dev: → and ← switch the scope with the caret unmoved, ⌥← moves the caret without changing scope, and on All projects ← moves the caret.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **bb32** Find's Open filter is live work: items not done or dropped (captured included), plans and phases not done, and inbox entries open or parked; Finished is everything else: done and dropped items, done phases and plans, entries settled other than by parking, and the whole Archive whatever a record's own stage
  Why: Parked entries stay Open because they still wait for Jeff, as archivedUnplanned's held() already treats them; done work in an open plan is Finished, not archived, and keeps its done/dropped tag
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
- **4df0** Find's scope takes plain ← and → in the input: ← picks the sheet's project, → every project (not a toggle, matching the control's left and right); ⌥P is gone. With ⌥, ⌘ or Shift the arrows still move or select in the text, and off a project they're ordinary caret keys
  Why: Jeff decided 2179 differently on review: ⌥P typed π and he asked for the arrow keys; giving up plain caret moves in a one-line query is the cost, and the modified arrows keep them

### Changes by layer

**other**: 2 files, +12 −8
- M mull/docs/serve/DESIGN.md (+1 −1)
- M mull/internal/serve/web/search.js (+11 −7)
