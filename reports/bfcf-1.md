---
item: bfcf
"n": 1
passed: true
commit: 66646b17eff935a78e07d3296bf5831a9c535e7e
base: 263ad1c7c62c602a8aaf5b20f13e1050d1524630
at: "2026-11-11T01:28:00Z"
by: /Users/jeff/dev/kit
summary: 'mull serve has a model page: a help sheet on how mull works, at #/-/model, linked from the foot of the sheet index on every page. It shows the record hierarchy (project, Needs you, plan, phase, item, Unplanned, Backlog) as a tree diagram, the item stages from captured to done, the six Needs you tiers with what lands in each (matching the live tiers'' wording), the rules (no status in prose, order only where it matters, review always required, and so on), and the commands the human runs (accept, decide, do, commit, plan agree, triage, park and drop). It''s static, with no status. DESIGN.md records its pattern, replacing the known gap the finish review found.'
tour:
  - Run make build && ./mull serve and open http://localhost:6855/.
  - 'At the bottom of the sheet index, click ''The model: how mull works''. The button turns blue while you''re on the page.'
  - 'Read down: How it fits together (the hierarchy), One item (stages captured → committed → active → in review → done), Needs you (the six tiers, as named on every project''s overview), Rules, and What you run.'
  - 'Narrow the window under 860px: the rules and commands stack into one column, and the hierarchy diagram scrolls sideways in its box.'
  - 'Open #/-/anything: Not found.'
checks:
  tree: ac0f1516575d0eac98652b1a8a3616c79c863c3c
  commit: 66646b17eff935a78e07d3296bf5831a9c535e7e
  at: "2026-11-11T01:27:40Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 19704
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 5
        removed: 1
      - status: M
        path: mull/internal/serve/web/app.js
        added: 11
        removed: 6
      - status: M
        path: mull/internal/serve/web/html.js
        added: 4
        removed: 0
      - status: A
        path: mull/internal/serve/web/model.js
        added: 79
        removed: 0
      - status: M
        path: mull/internal/serve/web/style.css
        added: 16
        removed: 0
      - status: M
        path: mull/internal/serve/web/tree.js
        added: 1
        removed: 1
design: |-
  A new module, model.js, renders the sheet as one string like the others: a title block with six boxed facts (records, stages, Needs you, backlog, where it's kept, who writes), then sections reusing the existing pieces: a mono pre for the hierarchy (as the prototype's model page), stage tags joined by arrows, a schedule table for the tiers (the hint as the title's meta line; "what lands here" drops at phone width as a .w column and isn't last), and .rules definition lists for the rules and commands.

  Route: #/-/model. A project's route is #/<project>, so #/model would shadow a project in a directory named model; "-" is reserved for sheets that belong to no project, and anything else under it is Not found. The tree ignores it (nothing to expand), and the rail foot now always renders: the not-on-mull list when there is one, then the model link, marked current on its own page.

  Content is static and written for Model v2 from docs/serve/README.md, mull guide and the CLI's own help: the tier titles and hints are copied from internal/mull/needsyou.go so the page names them as the overview does, and the commands are the human's (without --on-request, which is for agents). The prototype's 'Still open' list is left out: those questions are all decided. A comment in model.js says to keep it in step with the model.

  Rejected: generating the page from mull guide (the guides are written for agents and don't hold the human-facing rules); a route without the reserved prefix (it could shadow a project).

  Checked in Brave at 1440 light and 400 dark: no console errors, the fonts in use, the link's current state, the stacked lists and the scrolling diagram; #/-/nope is Not found.
decisions:
  - id: 7c0e
    text: 'The model page is #/-/model: ''-'' is reserved for sheets that belong to no project, and it''s linked from the sheet index''s foot on every page'
    detail: '#/model would collide with a project whose directory is named model; the prototype linked it from the rail foot'
  - id: fc4c
    text: 'The model page is static (model.js), with no live data: hierarchy, stages, the Needs you tiers with what lands in each, the rules, and the commands the human runs; its ''still open'' list from the prototype is left out'
    detail: It's a help page (human, 2026-11-11); those open questions are all decided now
---

# Done report 1: mull serve has no model page: direction.md's scope lists it and the prototype's rail foot links to it (#/model), but no item covers it (bfcf)

**Status:** PASSED  
**Commit:** 66646b17eff9  
**By:** /Users/jeff/dev/kit at 2026-11-11T01:28:00Z

## What was built

### Summary

mull serve has a model page: a help sheet on how mull works, at #/-/model, linked from the foot of the sheet index on every page. It shows the record hierarchy (project, Needs you, plan, phase, item, Unplanned, Backlog) as a tree diagram, the item stages from captured to done, the six Needs you tiers with what lands in each (matching the live tiers' wording), the rules (no status in prose, order only where it matters, review always required, and so on), and the commands the human runs (accept, decide, do, commit, plan agree, triage, park and drop). It's static, with no status. DESIGN.md records its pattern, replacing the known gap the finish review found.

### UI tour

1. Run make build && ./mull serve and open http://localhost:6855/.
2. At the bottom of the sheet index, click 'The model: how mull works'. The button turns blue while you're on the page.
3. Read down: How it fits together (the hierarchy), One item (stages captured → committed → active → in review → done), Needs you (the six tiers, as named on every project's overview), Rules, and What you run.
4. Narrow the window under 860px: the rules and commands stack into one column, and the hierarchy diagram scrolls sideways in its box.
5. Open #/-/anything: Not found.

### Checks (run by mull)

- `make done-checks`: pass, 19.7s

## How it was built

### Design notes

A new module, model.js, renders the sheet as one string like the others: a title block with six boxed facts (records, stages, Needs you, backlog, where it's kept, who writes), then sections reusing the existing pieces: a mono pre for the hierarchy (as the prototype's model page), stage tags joined by arrows, a schedule table for the tiers (the hint as the title's meta line; "what lands here" drops at phone width as a .w column and isn't last), and .rules definition lists for the rules and commands.

Route: #/-/model. A project's route is #/<project>, so #/model would shadow a project in a directory named model; "-" is reserved for sheets that belong to no project, and anything else under it is Not found. The tree ignores it (nothing to expand), and the rail foot now always renders: the not-on-mull list when there is one, then the model link, marked current on its own page.

Content is static and written for Model v2 from docs/serve/README.md, mull guide and the CLI's own help: the tier titles and hints are copied from internal/mull/needsyou.go so the page names them as the overview does, and the commands are the human's (without --on-request, which is for agents). The prototype's 'Still open' list is left out: those questions are all decided. A comment in model.js says to keep it in step with the model.

Rejected: generating the page from mull guide (the guides are written for agents and don't hold the human-facing rules); a route without the reserved prefix (it could shadow a project).

Checked in Brave at 1440 light and 400 dark: no console errors, the fonts in use, the link's current state, the stacked lists and the scrolling diagram; #/-/nope is Not found.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **7c0e** The model page is #/-/model: '-' is reserved for sheets that belong to no project, and it's linked from the sheet index's foot on every page
  Why: #/model would collide with a project whose directory is named model; the prototype linked it from the rail foot
- **fc4c** The model page is static (model.js), with no live data: hierarchy, stages, the Needs you tiers with what lands in each, the rules, and the commands the human runs; its 'still open' list from the prototype is left out
  Why: It's a help page (human, 2026-11-11); those open questions are all decided now

### Changes by layer

**other**: 6 files, +116 −8
- M mull/docs/serve/DESIGN.md (+5 −1)
- M mull/internal/serve/web/app.js (+11 −6)
- M mull/internal/serve/web/html.js (+4 −0)
- A mull/internal/serve/web/model.js (+79 −0)
- M mull/internal/serve/web/style.css (+16 −0)
- M mull/internal/serve/web/tree.js (+1 −1)
