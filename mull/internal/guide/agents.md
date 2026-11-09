## Working with mull

This project's plans, work items and open questions live in **mull**: a CLI whose data sits on the `mull` git branch. Use only the CLI. Never read or edit mull's data or the `mull` branch directly.

- **Output.** Every command prints JSON, and errors are `{"code", "error"}` on stderr. `mull guide json` documents the shapes and error codes.
- **Batch.** To write more than a couple of records, use `mull batch` (JSON Lines on stdin, all or nothing) instead of a shell loop.

1. **Start of session:** run `mull context --item <id>`, or `mull context` if you have no item yet. Read the decisions it lists, and don't re-decide anything the log already settles.
2. **One item per session.**
   - Claim it with `mull start <id>`.
   - Work on a branch named `<id>-<short-name>`.
   - End every commit message with the trailer line `Mull: <id>`.
3. **Capture as you go,** one line each, without waiting for the end:
   - `mull decided "<what you chose>" --item <id> --detail "<why, one line>"`: any choice the human didn't make for you.
   - `mull ask "<question>" --item <id>`: anything you need from the human. Keep working on whatever isn't blocked.
   - `mull loose "<what>" --item <id>`: anything out of scope **you** noticed (a bug, an inconsistency, half-finished work, an assumption you made).
   - `mull idea "<title>" --detail "<short note>" --source "<where>"` (add `--plan` if it belongs to one): only for an idea **the human raises**. An idea isn't work. File your own findings as loose ends, not ideas.
   - **Feedback the human hands over for later** (something to fix, change or revisit, but not now):
     ```
     mull loose "<the feedback, in the human's words>" --item <id> --source "human, <date or context>"
     ```
     - Use `--plan` when it isn't about one item.
     - File it when it's given. Don't hold it for the end of the session.
     - Don't act on it in your current work unless the human says to, and never drop it.
     - It's feedback if it judges or asks to change existing work. It's an idea if it's a possibility that isn't work yet. If you can't tell which, ask the human.
4. **Don't change plans or epics, and don't add items,** unless the human asks. Propose changes with `mull ask`.
   - Never promote an idea (`mull do <idea-id>`) on your own. When the human tells you to, add `--on-request`; use `--as epic` if they want an epic.
5. **Finish only with `mull done`:**
   ```
   mull done <id> --summary "<what changed and how it fits>" --tour -
   ```
   - Give the tour on stdin, one step per line: what to click and what the human should see. Write "none" if no UI changed.
   - `done` runs the project's hard checks itself. If they fail, fix the problem and run `done` again, or run `mull release <id>` and say why.
   - Never say an item is done unless `mull done` succeeded.
6. **Hand over:**
   - Show the human (or the supervisor) the report from `mull report <id>`, and the tour.
   - Run `mull accept <id> --on-request` only when the human tells you to.
   - Settle any other inbox entry only on the human's explicit instruction, also with `--on-request`.
7. **Run `mull sync`** after `done` and at the end of the session.
