## Working with mull

This project's plans, work items, backlog and open questions live in **mull**: a CLI whose data sits on the `mull` git branch. Use only the CLI. Never read or edit mull's data or the `mull` branch directly.

- **Output.** Every command prints JSON, and errors are `{"code", "error"}` on stderr. `mull guide json` documents the shapes and error codes.
- **Batch.** To write more than a couple of records, use `mull batch` (JSON Lines on stdin, all or nothing) instead of a shell loop.
- **The record.** Plans hold phases, and phases hold items. An item goes captured → committed → active → in review → done, or is dropped. Captured items (bugs and ideas) are the backlog: they aren't work until the human commits them. Done means reviewed.

1. **Start of session:** run `mull context --item <id>`, or `mull context` if you have no item yet.
   - Read the decisions it lists, and don't re-decide anything the log already settles.
   - Read the item's `docs` and its `phase_doc` (the phase's design and when it's done). Docs hold intent only. Status comes from mull, never from a doc.
2. **One item per session.**
   - Claim it with `mull start <id>`. Your item is committed work (`mull ready`), or a captured item the human wants done now: starting commits it.
   - Work on a branch named `<id>-<short-name>`.
   - End every commit message with the trailer line `Mull: <id>`.
3. **Capture as you go,** one line each, without waiting for the end. Anyone captures; only the human commits.
   - `mull decided "<what you chose>" --item <id> --detail "<why, one line>"`: any choice the human didn't make for you.
   - `mull ask "<question>" --item <id>`: anything you need from the human. Keep working on whatever isn't blocked.
   - `mull bug "<what>" --source "item <id>"`: anything out of scope that's wrong (a bug, an inconsistency, half-finished work, an assumption you made).
   - `mull idea "<title>" --detail "<short note>" --source "<where>"`: a possibility worth keeping that isn't work yet, whether the human raised it or you noticed it. Keep to ideas worth the human's time.
   - Bugs and ideas go to the backlog, not to the human's inbox: that's only questions, decisions and reviews. The human goes through the backlog in triage (`mull guide triage`). Add `--plan` or `--phase` to a capture if it belongs to one, and `--category` if it helps.
   - **Feedback the human hands over for later** (something to fix, change or revisit, but not now):
     ```
     mull bug "<the feedback, in the human's words>" --source "human, <date or context>"
     ```
     - File it when it's given. Don't hold it for the end of the session.
     - Don't act on it in your current work unless the human says to, and never drop it.
     - It's feedback if it judges or asks to change existing work. It's an idea if it's a possibility that isn't work yet. If you can't tell which, ask the human.
4. **Don't change plans or phases (or their docs), and don't add items,** unless the human asks. Propose changes with `mull ask`.
   - Only the human creates and agrees a plan (`mull plan new ... --on-request`, `mull plan agree <slug> --on-request`). A plan that's still shaping takes captures and questions, but no phases or committed work.
   - Read a plan's or phase's doc with `mull plan doc <slug>` or `mull phase doc <id>`.
   - Order exists only between phases (`mull need <phase> <phase>`). Items inside a phase are unordered.
   - Never commit a captured item (`mull commit <id>`) on your own. When the human tells you to, add `--on-request`, with `--plan`, `--phase` or `--plan none` if they say where it goes. Retag a capture (`mull set <id> plan|phase <value>`) only on their word too.
5. **Finish only with `mull done`:**
   ```
   mull done <id> --summary "<what changed and how it fits>" --tour "<steps, or none>" --design -
   ```
   - Give the design notes on stdin: how you built it and why (the approach, the shape of the code, what you rejected). They're required, and "none" isn't accepted.
   - Give the tour inline, one step per line: what to click and what the human should see. Write "none" if no UI changed. Only one of `--summary`, `--tour` and `--design` can read stdin.
   - `done` runs the project's hard checks itself. If they fail, fix the problem and run `done` again, or run `mull release <id>` and say why.
   - After `done` the item is in review. It's done only once its review is accepted. Never say an item is done unless `mull done` succeeded.
6. **Hand over:**
   - Show the human (or the supervisor) the report from `mull report <id>`, and the tour. The review carries the decisions you recorded with `--item`, and accepting it ratifies them.
   - Run `mull accept <id> --on-request` only when the human tells you to. If they decide one of those decisions differently, run `mull decide <decision-id> "<their decision>" --on-request` first.
   - Settle any other inbox entry only on the human's explicit instruction, also with `--on-request`.
7. **Run `mull sync`** after `done` and at the end of the session.
