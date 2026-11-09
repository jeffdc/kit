# Intake: bringing a project into mull

You're setting this project up in mull, **together with the human**. The human makes every decision; you read, propose, and record what they approve. Work in small steps, and show your proposal before writing anything.

Use the mull CLI for everything you record. Read old material directly: old `.mull/matters/*.md` files, docs, and git history are plain files you may read.

## 1. Look around (read only)
- **Old mull data:** read `.mull/matters/*.md` (YAML frontmatter plus a body), `.mull/docket.yml`, and `.mull/archive/` if present. Note each matter's status, epic, links (`parent`, `needs`, `blocks`, `relates`) and size.
- **Hub documents:** look for a matter or doc that tracks a large effort (an "overview", "plan", "punchlist" or "roadmap"). It's the best source for plans and epics.
- **git:**
  - `git branch -a`, `git worktree list`, and `git log --oneline -50` on the main and integration branches;
  - which branches are ahead of the integration branch, and how stale they are;
  - uncommitted changes.
- **The project's own rules and checks:** AGENTS.md / CLAUDE.md, Makefile targets, CI config.

## 2. Set up mull (ask first)
Propose, and wait for approval:
- the integration branch (where work lands) and the release branch;
- the hard checks `mull done` should run. Ideally that's one make target, e.g. `make done-checks`. If none exists, propose one, but don't create it without approval;
- layers: path globs for `ui`, `tests` and the other main areas of the code.

Then run:
```
mull init --integration <branch> --release <branch>
mull config checks "<command>"
mull config layer ui "<glob>" ...
mull config layer tests "<glob>" ...
```

## 3. Plans and epics (propose, then record)
- **Plans.** Propose one plan per large effort (from the hub documents), using a plain-English slug. Small, unrelated work goes in the standing `general` plan.
- **Epics.** For each plan, propose epics: big pieces, in roughly the order they must happen. Add `--needs` between epics only where one truly must finish first.
- **Show the human the whole outline** (plans, then epics, with needs) and adjust until they approve. Then record it:
  ```
  mull plan new <slug> "<title>" --summary "<two lines>"
  mull epic add "<title>" --plan <slug> --summary "<one line>" --needs <epic-id>
  ```

## 4. Items for work in flight or ready
- **What gets an item.** Only work that's in flight, ready, or clearly next. Each item must be something **one agent can finish in one session**; split anything bigger.
- **Fields.** Give each item a clear title, `--epic`, `--needs` where there's a real dependency, and `--doc` for any design document an agent must read.
- **Notes.** Notes are short. Don't copy long prose; link the document instead.
- **Source.** Each item's notes must cite where it came from, e.g. "From matter 9449, 'Order to release' step 3".
- **Status from the old data is a claim.** Where old notes say something is done, check git before treating it as done. If you can't confirm it, file it as a question.

## 5. The inbox
File what needs the human, citing sources with `--source`:
- **Decisions agents made that the human never reviewed:** `mull decided "<decision>" --item <id> --source "<matter>, <heading>"`. Look for sections like "for review", "decisions made" and "assumed".
- **Open questions:** `mull ask "<question>" --source ...`.
- **Loose ends:** `mull loose "<what>" --source ...`. That covers:
  - known bugs and findings that never became work;
  - stale branches and worktrees (name, age, and commits ahead);
  - uncommitted changes;
  - contradictions between the old status fields and what the hub documents claim.

## 6. The old backlog
Matters with no place in any plan are **not migrated automatically**. List them for the human, grouped by theme, with a one-line summary each. The human decides, usually in batches:
- items in `general`;
- loose ends;
- or left behind.

They stay readable in `.mull/` either way. Don't delete the old `.mull/` directory. That's a separate decision for the human.

## 7. Finish
- Run `mull view` and `mull inbox`, and walk the human through them.
- Fix anything they say is wrong.
- Add the agent protocol (`mull guide agents`) to the project's AGENTS.md, if the human agrees.
- Run `mull sync`.
