# AGENTS.md

Guidance for any coding agent working in kit, a monorepo of small, independent tools. Each tool has its own directory, language and build. Read the tool's own CLAUDE.md or README before working in it.

## Tracking work in kit

kit's plans, work items, questions and backlog (bugs and ideas) are tracked with **mull**. The data lives on the `mull` branch; never check it out or edit it.
- **Plans:** `forage-rework` (Forage: server + PWA), `mull-next` (the mull rewrite and moving repos onto it), `mull-ui-edit` (editing from mull serve) and `mull-backlog-ui` (mull serve's backlog tools; still shaping). Closed plans (`mull-serve`, `mull-data`, `general`) are in the archive; don't add to them. Small work is unplanned (`--plan none`).
- **Hard checks:** `mull done` runs `make done-checks` at the repo root. That's vet, tests and fmt-check for flint, forage, mull and watchmen, so a finished item never leaves main red.
- **Layers:** `ui` (forage's PWA, `forage/internal/pwa/**`) and `tests`.

## Working with mull

This project's work is tracked with **mull**, a CLI whose data lives on the `mull` branch. Use only the CLI; never read or edit that branch. Commands print JSON.

1. **Start:** `mull context --item <id>` gives your brief: the task, the project's rules (RULES.md), the item's target branch, and queued work in its area. `mull start <id>` makes your branch and worktree and prints the path: work there. End every commit message with `Mull: <id>`.
2. **Follow the rules.** If your work bends, breaks or adds one, you'll say so at done, and it waits for review. Never change RULES.md or a ratchet file unless the human asks.
3. **Decide locally.** Note a choice worth keeping with `mull decided "<choice>" --item <id> --detail "<why>"`. It goes in your report, not to the human.
4. **Ask only** about a rule conflict or something hard to undo, and keep working: `mull ask "<question>" --item <id> --default "<what you'll do>" --by 2d`.
5. **Queue what you find.** Fix small problems in the area you're changing as part of your change. File the rest in one line: `mull bug "<what>" --area <path> --source "item <id>"`. A possibility, not a fault, is `mull idea`. Feedback the human hands over for later is a bug too, with `--source "human, <date>"`; file it when given, and never drop it.
6. **Finish:** `mull done <id> --summary "<one line>" --rules none` (or `--rules "R3: <how>"`), adding `--covers <ids>` for queued items your work also fixed and `--slowed "<what>"` if something slowed you. Done runs the checks once and scores risk. Low and medium risk lands on the target. High risk waits for review. If done says the target moved, run `git merge <target>` and finish again. Never claim done in prose.
7. **Don't** change plans, phases, checks or risk settings, or settle inbox entries, unless the human asks; then add `--on-request`.
8. `mull sync` after done.
