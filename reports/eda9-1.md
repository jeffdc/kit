---
item: eda9
"n": 1
passed: true
commit: 02dcb3bc10a6d86c38c434964a921b157ed20baa
base: 8274523c54be658ec9d48f55d15f4f2d1a5c5dd4
at: "2026-11-12T00:54:25Z"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-adf0fdbb087d1a707
summary: 'The copy actions that were Jeff''s own steps now save from mull serve. On a captured item''s sheet, Drop is a write taking a reason (it goes into the notes) and Commit to a plan is gone: committing and triage happen with an agent, so there''s no picker and no Mark triaged. Agree this plan is a write that asks once more, which fixes d1c5. Capture, at the right of a project''s tabs and an open plan''s tabs, or the c key, opens a quick form under the tabs: Bug or Idea, a one-line title, an optional detail that becomes the notes; Enter in the title or Cmd+Enter saves, the toast names the new ID, and the draft is kept across closing, live re-renders and reloads. On a plan''s sheet the capture is tagged to that plan unless ''For this plan'' is cleared. The same Capture sits on the Backlog tab, project and plan, so it covers ffe0 too (left for Jeff to settle). Start it now, Discuss with an agent and Copy all N for an agent stay copies. The server offers plan agree, bug and idea in uiOps; captures from the page record ''<user.name> in mull serve'' as their source, and items dropped from the page note ''Dropped by <name>: <reason>''. The model sheet''s command reference and DESIGN.md say which steps save and which copy.'
tour:
  - make dev (DEV_PORT=6902 here); its demo now has a shaping plan, 'Pilot with two teams'.
  - 'Open the demo''s Backlog tab and press c: the Capture form opens under the tabs with the title focused. Pick Bug, type a title, press Enter: the toast says ''Captured bug <id>'' and the row appears in the backlog, not clouded.'
  - 'Press c, type half a title, press Esc, reload the page, press c: the draft is still there.'
  - 'Open Web app > Backlog, press Capture: ''For this plan'' is checked; save and the idea appears in this plan''s backlog. Clear the box to capture untagged.'
  - 'Open ''Pilot with two teams'': click Agree this plan, it asks ''Agree it?''; click again: the plan is active and Close and archive replaces it.'
  - 'Open a captured item''s sheet: only Start it now (a copy) and Drop (a write) are offered. Drop, write a reason, Cmd+Enter: the item is dropped and its notes say ''Dropped by <name>: <reason>''.'
  - 'Open The model: the ''What you run'' list names Capture, Agree this plan and Drop as doing these for you, and commit and triage as run with an agent.'
checks:
  tree: 5f011282b259b11e54fed6352651cd53a3e81629
  commit: 02dcb3bc10a6d86c38c434964a921b157ed20baa
  at: "2026-11-12T00:53:59Z"
  by: /Users/jeff/dev/kit/.claude/worktrees/agent-adf0fdbb087d1a707
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 26291
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 4
        removed: 3
      - status: M
        path: mull/internal/mull/inbox.go
        added: 5
        removed: 1
      - status: M
        path: mull/internal/mull/work.go
        added: 5
        removed: 1
      - status: M
        path: mull/internal/serve/ops.go
        added: 4
        removed: 1
      - status: M
        path: mull/internal/serve/web/app.js
        added: 73
        removed: 8
      - status: A
        path: mull/internal/serve/web/capture.js
        added: 92
        removed: 0
      - status: M
        path: mull/internal/serve/web/item.js
        added: 10
        removed: 4
      - status: M
        path: mull/internal/serve/web/model.js
        added: 6
        removed: 5
      - status: M
        path: mull/internal/serve/web/needs.js
        added: 2
        removed: 2
      - status: M
        path: mull/internal/serve/web/plan.js
        added: 6
        removed: 3
      - status: M
        path: mull/internal/serve/web/style.css
        added: 12
        removed: 3
      - status: M
        path: mull/internal/serve/web/title.js
        added: 4
        removed: 2
      - status: M
        path: mull/internal/serve/web/write.js
        added: 1
        removed: 1
      - status: M
        path: mull/scripts/dev-demo.sh
        added: 2
        removed: 0
  - layer: tests
    files:
      - status: M
        path: mull/internal/serve/ops_test.go
        added: 73
        removed: 0
design: |-
  Every conversion uses the existing write machinery: Drop is a textWrite (its form sits under the item's action row), Agree this plan is a writeButton (armed confirm, as it can't be undone), and the server change is only three ops added to uiOps. No mull logic moved into the server.

  Capture is the one new piece, in its own module (capture.js) so the shared files change little. It's a text write with three fields, so it keeps its state in write.js's state map under capture:<project> (open, pending, error, and the draft: title, detail, kind, and whether the plan box was cleared). disarm() therefore closes it like any form and keeps the draft; save() in app.js is shared, with one small extension: done may be a function of the ops report, so the toast can name the new item's ID. The draft is mirrored to localStorage per project because Capture is where Jeff gets a thought down, and a page reload on a new build would otherwise lose it; it's cleared once saved. focusKey and the caret restore learned the capture fields so typing survives live re-renders. Enter in the title saves through the form's implicit submit (the submit is disabled while the title is empty); c opens it when focus isn't in a text field and Find is closed. TICK is now exported from write.js rather than copied.

  Where a capture goes: tagged to the plan whose sheet it's on (with a checked 'For this plan' box to clear), untagged from project sheets; no Capture on phase sheets (no tabs row; a phase tag is a triage call) or archived plans. Recorded as a decision.

  Recording who: settlements from the page carry by and human, but items have no by field. Rather than add one, mull fills what the record already has when the write is made as the human: a capture with no source gets '<name> in mull serve', and an item dropped with a reason notes 'Dropped by <name>: ...'. CLI writes are unchanged. The name is user.name from the repository config, as for settlements (idea 4e64 suggests using mull name).

  Out of scope, captured: bug 522d (a dropped captured item shows under Unplanned). ffe0 is covered by the Backlog tab's Capture and left for Jeff.

  Tested: Go tests for plan agree (and on-request refused), bug and idea captures (kinds, plan tag, detail as notes, source), and dropping a captured item with the name in its notes; make test, make vet, make fmt-check. In the dev UI (headless Chromium against make dev on 6902): capture by keyboard, Esc keeps the draft, reload keeps the draft, plan tagging and untagging, an empty title can't save, c is ignored while Find is open, captured items aren't clouded, Agree arms then saves, Drop with a reason, the model sheet and phone width, with no console errors.
decisions:
  - id: "9625"
    text: 'Capture is a write button at the right of the tabs row (project sheets and open plans), opened by it or by the c key; its form opens under the tabs: Bug/Idea, a one-line title (focused), an optional detail that becomes the notes; Enter in the title or Cmd+Enter saves; it keeps the last kind'
    detail: 'Few clicks and keyboard first: c, type, Enter. The form follows the text-write pattern (no arming; Esc or Cancel keeps the draft).'
  - id: c285
    text: Capture's draft is also kept per project in the browser's localStorage, cleared once saved, so a reload (a new build reloads the page) doesn't lose a thought half written
  - id: edbd
    text: Capture tags what it captures to the plan whose sheet it's on (a 'For this plan' box, checked, clears to untagged); on project sheets it goes to the backlog untagged; phase sheets have no Capture, and closed plans offer none
    detail: Jeff is usually looking at the plan a thought belongs to, and triage with an agent can retag it; the box makes the rare unrelated thought one click. Phases have no tabs row to hold the button, and a phase-level tag is a triage call.
  - id: 0bb6
    text: 'A bug or idea captured from mull serve records its source as ''<git user.name> in mull serve'' when none is given, and an item dropped from the page notes ''Dropped by <name>: <reason>'' (mull does both for writes made as the human)'
    detail: Items have no by field like settled entries; the source and the note are where the record already says who and where. The name is the git user.name, as for settlements from the page.
  - id: d0ed
    text: 'ffe0 (add new items from the backlog view) is covered by the same Capture: it sits in the Backlog tab''s actions, the project''s and each open plan''s (where it tags to the plan); ffe0 is left for Jeff to settle'
  - id: b558
    text: 'No other copy action is one of Jeff''s own steps: what''s left copying is Start it now and the Needs you agent prompts (Discuss with an agent, Copy all N for an agent), which hand work to an agent; the remaining command text (mull init, mull migrate, mull guide intake for unmulled repos) is guidance in prose, not buttons, and is configuration'
---

# Done report 1: Audit every action in mull serve that copies a command instead of saving, then convert each of Jeff's own steps into a real write (eda9)

**Status:** PASSED  
**Commit:** 02dcb3bc10a6  
**By:** /Users/jeff/dev/kit/.claude/worktrees/agent-adf0fdbb087d1a707 at 2026-11-12T00:54:25Z

## What was built

### Summary

The copy actions that were Jeff's own steps now save from mull serve. On a captured item's sheet, Drop is a write taking a reason (it goes into the notes) and Commit to a plan is gone: committing and triage happen with an agent, so there's no picker and no Mark triaged. Agree this plan is a write that asks once more, which fixes d1c5. Capture, at the right of a project's tabs and an open plan's tabs, or the c key, opens a quick form under the tabs: Bug or Idea, a one-line title, an optional detail that becomes the notes; Enter in the title or Cmd+Enter saves, the toast names the new ID, and the draft is kept across closing, live re-renders and reloads. On a plan's sheet the capture is tagged to that plan unless 'For this plan' is cleared. The same Capture sits on the Backlog tab, project and plan, so it covers ffe0 too (left for Jeff to settle). Start it now, Discuss with an agent and Copy all N for an agent stay copies. The server offers plan agree, bug and idea in uiOps; captures from the page record '<user.name> in mull serve' as their source, and items dropped from the page note 'Dropped by <name>: <reason>'. The model sheet's command reference and DESIGN.md say which steps save and which copy.

### UI tour

1. make dev (DEV_PORT=6902 here); its demo now has a shaping plan, 'Pilot with two teams'.
2. Open the demo's Backlog tab and press c: the Capture form opens under the tabs with the title focused. Pick Bug, type a title, press Enter: the toast says 'Captured bug <id>' and the row appears in the backlog, not clouded.
3. Press c, type half a title, press Esc, reload the page, press c: the draft is still there.
4. Open Web app > Backlog, press Capture: 'For this plan' is checked; save and the idea appears in this plan's backlog. Clear the box to capture untagged.
5. Open 'Pilot with two teams': click Agree this plan, it asks 'Agree it?'; click again: the plan is active and Close and archive replaces it.
6. Open a captured item's sheet: only Start it now (a copy) and Drop (a write) are offered. Drop, write a reason, Cmd+Enter: the item is dropped and its notes say 'Dropped by <name>: <reason>'.
7. Open The model: the 'What you run' list names Capture, Agree this plan and Drop as doing these for you, and commit and triage as run with an agent.

### Checks (run by mull)

- `make done-checks`: pass, 26.3s

## How it was built

### Design notes

Every conversion uses the existing write machinery: Drop is a textWrite (its form sits under the item's action row), Agree this plan is a writeButton (armed confirm, as it can't be undone), and the server change is only three ops added to uiOps. No mull logic moved into the server.

Capture is the one new piece, in its own module (capture.js) so the shared files change little. It's a text write with three fields, so it keeps its state in write.js's state map under capture:<project> (open, pending, error, and the draft: title, detail, kind, and whether the plan box was cleared). disarm() therefore closes it like any form and keeps the draft; save() in app.js is shared, with one small extension: done may be a function of the ops report, so the toast can name the new item's ID. The draft is mirrored to localStorage per project because Capture is where Jeff gets a thought down, and a page reload on a new build would otherwise lose it; it's cleared once saved. focusKey and the caret restore learned the capture fields so typing survives live re-renders. Enter in the title saves through the form's implicit submit (the submit is disabled while the title is empty); c opens it when focus isn't in a text field and Find is closed. TICK is now exported from write.js rather than copied.

Where a capture goes: tagged to the plan whose sheet it's on (with a checked 'For this plan' box to clear), untagged from project sheets; no Capture on phase sheets (no tabs row; a phase tag is a triage call) or archived plans. Recorded as a decision.

Recording who: settlements from the page carry by and human, but items have no by field. Rather than add one, mull fills what the record already has when the write is made as the human: a capture with no source gets '<name> in mull serve', and an item dropped with a reason notes 'Dropped by <name>: ...'. CLI writes are unchanged. The name is user.name from the repository config, as for settlements (idea 4e64 suggests using mull name).

Out of scope, captured: bug 522d (a dropped captured item shows under Unplanned). ffe0 is covered by the Backlog tab's Capture and left for Jeff.

Tested: Go tests for plan agree (and on-request refused), bug and idea captures (kinds, plan tag, detail as notes, source), and dropping a captured item with the name in its notes; make test, make vet, make fmt-check. In the dev UI (headless Chromium against make dev on 6902): capture by keyboard, Esc keeps the draft, reload keeps the draft, plan tagging and untagging, an empty title can't save, c is ignored while Find is open, captured items aren't clouded, Agree arms then saves, Drop with a reason, the model sheet and phone width, with no console errors.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **9625** Capture is a write button at the right of the tabs row (project sheets and open plans), opened by it or by the c key; its form opens under the tabs: Bug/Idea, a one-line title (focused), an optional detail that becomes the notes; Enter in the title or Cmd+Enter saves; it keeps the last kind
  Why: Few clicks and keyboard first: c, type, Enter. The form follows the text-write pattern (no arming; Esc or Cancel keeps the draft).
- **c285** Capture's draft is also kept per project in the browser's localStorage, cleared once saved, so a reload (a new build reloads the page) doesn't lose a thought half written
- **edbd** Capture tags what it captures to the plan whose sheet it's on (a 'For this plan' box, checked, clears to untagged); on project sheets it goes to the backlog untagged; phase sheets have no Capture, and closed plans offer none
  Why: Jeff is usually looking at the plan a thought belongs to, and triage with an agent can retag it; the box makes the rare unrelated thought one click. Phases have no tabs row to hold the button, and a phase-level tag is a triage call.
- **0bb6** A bug or idea captured from mull serve records its source as '<git user.name> in mull serve' when none is given, and an item dropped from the page notes 'Dropped by <name>: <reason>' (mull does both for writes made as the human)
  Why: Items have no by field like settled entries; the source and the note are where the record already says who and where. The name is the git user.name, as for settlements from the page.
- **d0ed** ffe0 (add new items from the backlog view) is covered by the same Capture: it sits in the Backlog tab's actions, the project's and each open plan's (where it tags to the plan); ffe0 is left for Jeff to settle
- **b558** No other copy action is one of Jeff's own steps: what's left copying is Start it now and the Needs you agent prompts (Discuss with an agent, Copy all N for an agent), which hand work to an agent; the remaining command text (mull init, mull migrate, mull guide intake for unmulled repos) is guidance in prose, not buttons, and is configuration

### Changes by layer

**other**: 14 files, +226 −34
- M mull/docs/serve/DESIGN.md (+4 −3)
- M mull/internal/mull/inbox.go (+5 −1)
- M mull/internal/mull/work.go (+5 −1)
- M mull/internal/serve/ops.go (+4 −1)
- M mull/internal/serve/web/app.js (+73 −8)
- A mull/internal/serve/web/capture.js (+92 −0)
- M mull/internal/serve/web/item.js (+10 −4)
- M mull/internal/serve/web/model.js (+6 −5)
- M mull/internal/serve/web/needs.js (+2 −2)
- M mull/internal/serve/web/plan.js (+6 −3)
- M mull/internal/serve/web/style.css (+12 −3)
- M mull/internal/serve/web/title.js (+4 −2)
- M mull/internal/serve/web/write.js (+1 −1)
- M mull/scripts/dev-demo.sh (+2 −0)

**tests**: 1 files, +73 −0
- M mull/internal/serve/ops_test.go (+73 −0)
