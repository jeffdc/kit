---
item: 9dab
"n": 1
passed: true
commit: 89b819f9cda00fbcaa47dce827ee05d95bb6c581
base: b26a0a33566b2cdfd9c25688844ee6b1d0ac2b9d
at: "2026-11-18T20:25:12Z"
by: /Users/jeff/dev/kit
summary: 'mull serve''s Needs you list is now a CSS container: at 600px or less (a phase page''s side column, whatever the window width) its rows put Discuss with an agent, Answer and More under the text, and a decision group''s rows put their actions under the decision, instead of squeezing the text to one letter a line. Tier hints hide there too. DESIGN.md says so beside the ≤860px rules.'
tour:
  - Open a phase that has a question on one of its items, in a window wider than 860px.
  - In the Needs you column on the right, the question's text wraps normally across the column, with Discuss with an agent, Answer and More on a line below it.
  - 'Widen the window, or open the project overview, where Needs you is wide: the buttons sit beside the text as before.'
checks:
  tree: ef286814047b582175f158dc0a537cb3f5d324f8
  commit: 89b819f9cda00fbcaa47dce827ee05d95bb6c581
  at: "2026-11-18T20:24:53Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 18841
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 1
        removed: 1
      - status: M
        path: mull/internal/serve/web/style.css
        added: 11
        removed: 1
design: |-
  Cause: .it is a grid of 34px 62px minmax(0, 1fr) auto. The auto column holds the actions (up to 380px), and the text column, minmax(0, 1fr) with overflow-wrap: anywhere, takes what's left, so in the phase page's roughly 310px Needs you column it fell to a letter's width. The stacking that fixes it existed only in the ≤860px media query, which looks at the viewport, not the column.

  Fix: .sch (the schedule's root) gets container-type: inline-size, and an @container (max-width: 600px) block repeats the narrow layout: rows lose the actions column and put .act in column 3 under the text; decision rows in a group do the same with .li-acts; tier hints hide. The block sits after the base .it and .grp li rules so it wins on equal specificity (my first placement lost to them). 600px leaves the text about 240px beside the actions at the threshold. I kept the media query rather than replacing it, so narrow viewports behave exactly as before.

  Checked by rendering the real style.css with a Needs you row and a decision group at 310px and 640px in headless Chromium: before, the text went one letter a line at 310px; after, it wraps normally with the buttons below, and 640px is unchanged. The headless browser couldn't load the live dev page (it waits on the event stream), so the static page stood in for it.
---

# Done report 1: mull serve: a Needs you row in a narrow column (the phase page's Needs you) squeezes its text to one letter per line behind its buttons (9dab)

**Status:** PASSED  
**Commit:** 89b819f9cda0  
**By:** /Users/jeff/dev/kit at 2026-11-18T20:25:12Z

## What was built

### Summary

mull serve's Needs you list is now a CSS container: at 600px or less (a phase page's side column, whatever the window width) its rows put Discuss with an agent, Answer and More under the text, and a decision group's rows put their actions under the decision, instead of squeezing the text to one letter a line. Tier hints hide there too. DESIGN.md says so beside the ≤860px rules.

### UI tour

1. Open a phase that has a question on one of its items, in a window wider than 860px.
2. In the Needs you column on the right, the question's text wraps normally across the column, with Discuss with an agent, Answer and More on a line below it.
3. Widen the window, or open the project overview, where Needs you is wide: the buttons sit beside the text as before.

### Checks (run by mull)

- `make done-checks`: pass, 18.8s

## How it was built

### Design notes

Cause: .it is a grid of 34px 62px minmax(0, 1fr) auto. The auto column holds the actions (up to 380px), and the text column, minmax(0, 1fr) with overflow-wrap: anywhere, takes what's left, so in the phase page's roughly 310px Needs you column it fell to a letter's width. The stacking that fixes it existed only in the ≤860px media query, which looks at the viewport, not the column.

Fix: .sch (the schedule's root) gets container-type: inline-size, and an @container (max-width: 600px) block repeats the narrow layout: rows lose the actions column and put .act in column 3 under the text; decision rows in a group do the same with .li-acts; tier hints hide. The block sits after the base .it and .grp li rules so it wins on equal specificity (my first placement lost to them). 600px leaves the text about 240px beside the actions at the threshold. I kept the media query rather than replacing it, so narrow viewports behave exactly as before.

Checked by rendering the real style.css with a Needs you row and a decision group at 310px and 640px in headless Chromium: before, the text went one letter a line at 310px; after, it wraps normally with the buttons below, and 640px is unchanged. The headless browser couldn't load the live dev page (it waits on the event stream), so the static page stood in for it.

### Decisions made in the session

None recorded.

### Changes by layer

**other**: 2 files, +12 −2
- M mull/docs/serve/DESIGN.md (+1 −1)
- M mull/internal/serve/web/style.css (+11 −1)
