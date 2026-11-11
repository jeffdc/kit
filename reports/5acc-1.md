---
item: 5acc
"n": 1
passed: true
commit: 38505524591314d9ddb8fc17475669fa64cfd90d
base: 73dbffb72a2e1eae260cc0dbbcab176f04628584
at: "2026-11-11T13:33:34Z"
by: /Users/jeff/dev/kit
summary: 'Text on the mull serve sheets no longer wraps early: prose (notes, docs, ledes, the model sheet, the note box) runs to 120ch instead of 68 to 90, and on plan, phase and item sheets the title block''s fields fit their values so the name takes the rest of the width.'
tour:
  - 'Open an item with long notes on a wide window: the notes run to about 120 characters a line instead of stopping near 76.'
  - 'Open a plan''s Plan doc tab: the prose column is up to 120ch wide, with the status margin beside it.'
  - 'Open an item, phase or plan with a long name (e.g. Drop or park a Needs you entry with a reason): the name sits on one line at 1280px and wider, and the Stage, ID, Kind, Claim, From and Git fields are only as wide as their values.'
checks:
  tree: 325e5c8720bd48efda52196b199c5b071b5bd707
  commit: 38505524591314d9ddb8fc17475669fa64cfd90d
  at: "2026-11-11T13:33:14Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 19767
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 3
        removed: 3
      - status: M
        path: mull/internal/serve/web/style.css
        added: 8
        removed: 6
design: |-
  How: two CSS changes and DESIGN.md to match. Every ch cap on prose (.notes 76ch, .blk's doc column 68ch, .lede and .model .measure 72ch, .note 90ch) is 120ch, the human's normal measure; the Prose entry in DESIGN.md records it as their choice and why. The wide title block (.tb.wide, used for plans, phases and items) was minmax(380px, 3fr) plus three 1fr field columns, so the name got half the sheet and short values got the rest; it's now minmax(380px, 1fr) plus three fit-content(260px) columns: fields as wide as their values up to 260px, the name everything else. Project sheets (.tb without wide) keep their columns, as their fields carry longer values.

  Measured in headless Chromium: the 38-character title from the human's screenshot is one line at 1600, 1440 and 1280px (name box 974, 814 and 654px; fields 83 to 137px).
---

# Done report 1: Prose and titles on the sheets wrap for no reason: widen prose to 120ch, and give long names the room in the title block (5acc)

**Status:** PASSED  
**Commit:** 385055245913  
**By:** /Users/jeff/dev/kit at 2026-11-11T13:33:34Z

## What was built

### Summary

Text on the mull serve sheets no longer wraps early: prose (notes, docs, ledes, the model sheet, the note box) runs to 120ch instead of 68 to 90, and on plan, phase and item sheets the title block's fields fit their values so the name takes the rest of the width.

### UI tour

1. Open an item with long notes on a wide window: the notes run to about 120 characters a line instead of stopping near 76.
2. Open a plan's Plan doc tab: the prose column is up to 120ch wide, with the status margin beside it.
3. Open an item, phase or plan with a long name (e.g. Drop or park a Needs you entry with a reason): the name sits on one line at 1280px and wider, and the Stage, ID, Kind, Claim, From and Git fields are only as wide as their values.

### Checks (run by mull)

- `make done-checks`: pass, 19.8s

## How it was built

### Design notes

How: two CSS changes and DESIGN.md to match. Every ch cap on prose (.notes 76ch, .blk's doc column 68ch, .lede and .model .measure 72ch, .note 90ch) is 120ch, the human's normal measure; the Prose entry in DESIGN.md records it as their choice and why. The wide title block (.tb.wide, used for plans, phases and items) was minmax(380px, 3fr) plus three 1fr field columns, so the name got half the sheet and short values got the rest; it's now minmax(380px, 1fr) plus three fit-content(260px) columns: fields as wide as their values up to 260px, the name everything else. Project sheets (.tb without wide) keep their columns, as their fields carry longer values.

Measured in headless Chromium: the 38-character title from the human's screenshot is one line at 1600, 1440 and 1280px (name box 974, 814 and 654px; fields 83 to 137px).

### Decisions made in the session

None recorded.

### Changes by layer

**other**: 2 files, +11 −9
- M mull/docs/serve/DESIGN.md (+3 −3)
- M mull/internal/serve/web/style.css (+8 −6)
