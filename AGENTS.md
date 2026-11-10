# AGENTS.md

Guidance for any coding agent working in kit, a monorepo of small, independent tools. Each tool has its own directory, language and build. Read the tool's own CLAUDE.md or README before working in it.

## Tracking work in kit

kit's plans, work items, questions and backlog (bugs and ideas) are tracked with **mull**. The data lives on the `mull` branch; never check it out or edit it.
- **Plans:** `forage-rework` (Forage: server + PWA) and `mull-next` (the mull rewrite and moving repos onto it). Small work is unplanned (`--plan none`).
- **Hard checks:** `mull done` runs `make done-checks` at the repo root. That's vet, tests and fmt-check for flint, forage, mull and watchmen, so a finished item never leaves main red.
- **Layers:** `ui` (forage's PWA, `forage/internal/pwa/**`; a UI tour is required when it changes) and `tests`.

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
   - `mull bug "<what>" --source "item <id>"`: anything out of scope **you** noticed that's wrong (a bug, an inconsistency, half-finished work, an assumption you made). It goes to the backlog, not to the human's inbox.
   - `mull idea "<title>" --detail "<short note>" --source "<where>"`: only for an idea **the human raises**. An idea isn't work. File your own findings as bugs, not ideas.
   - Add `--plan` or `--phase` to a capture if it belongs to one, and `--category` if it helps. Captures aren't work until the human commits them.
   - **Feedback the human hands over for later** (something to fix, change or revisit, but not now):
     ```
     mull bug "<the feedback, in the human's words>" --source "human, <date or context>"
     ```
     - File it when it's given. Don't hold it for the end of the session.
     - Don't act on it in your current work unless the human says to, and never drop it.
     - It's feedback if it judges or asks to change existing work. It's an idea if it's a possibility that isn't work yet. If you can't tell which, ask the human.
4. **Don't change plans or epics, and don't add items,** unless the human asks. Propose changes with `mull ask`.
   - Never commit a captured item (`mull commit <id>`) on your own. When the human tells you to, add `--on-request`, with `--plan`, `--phase` or `--plan none` if they say where it goes.
5. **Finish only with `mull done`:**
   ```
   mull done <id> --summary "<what changed and how it fits>" --tour "<steps, or none>" --design -
   ```
   - Give the design notes on stdin: how you built it and why (the approach, the shape of the code, what you rejected). They're required, and "none" isn't accepted.
   - Give the tour inline, one step per line: what to click and what the human should see. Write "none" if no UI changed. Only one of `--summary`, `--tour` and `--design` can read stdin.
   - `done` runs the project's hard checks itself. If they fail, fix the problem and run `done` again, or run `mull release <id>` and say why.
   - Never say an item is done unless `mull done` succeeded.
6. **Hand over:**
   - Show the human (or the supervisor) the report from `mull report <id>`, and the tour. The review carries the decisions you recorded with `--item`, and accepting it ratifies them.
   - Run `mull accept <id> --on-request` only when the human tells you to. If they decide one of those decisions differently, run `mull decide <decision-id> "<their decision>" --on-request` first.
   - Settle any other inbox entry only on the human's explicit instruction, also with `--on-request`.
7. **Run `mull sync`** after `done` and at the end of the session.
