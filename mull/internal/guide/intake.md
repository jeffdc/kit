# Intake: bringing a project into mull

You're setting this project up in mull, **together with the human**. The human makes every decision; you read, propose, and record what they approve. Work in small steps, and show your proposal before writing anything.

Use the mull CLI for everything you record. Read old material directly: old `.mull/matters/*.md` files, docs, and git history are plain files you may read.

**Record with `mull batch`, not shell loops.** Write the approved records as JSON Lines, and check them with `mull batch --dry-run < file.jsonl`. Then run `mull batch < file.jsonl`. A batch is all or nothing: if any line fails, nothing is written and every failed line is reported, so fix those and run it again. Field names are the command's flag names, and `mull guide json` lists every op's fields. Give a line a `"ref"`, and later lines can use `"@ref"` for its ID. That's how epics, items that need other new items, and inbox entries about new items go in one batch.

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

Then run `mull init --integration <branch> --release <branch>`. Config can go in the first batch:
```
{"op": "config checks", "commands": ["make done-checks"]}
{"op": "config layer", "name": "ui", "globs": ["lib/app_web/**", "assets/**"]}
{"op": "config layer", "name": "tests", "globs": ["test/**"]}
```

## 3. Plans and epics (propose, then record)
- **Plans.** Propose one plan per large effort (from the hub documents), using a plain-English slug. Small, unrelated work goes in the standing `general` plan.
- **Epics.** For each plan, propose epics: big pieces, in roughly the order they must happen. Add `--needs` between epics only where one truly must finish first.
- **Show the human the whole outline** (plans, then epics, with needs) and adjust until they approve. Then record it in a batch (see the example below).

## 4. Items for work in flight or ready
- **What gets an item.** Only work that's in flight, ready, or clearly next. Each item must be something **one agent can finish in one session**; split anything bigger.
- **Fields.** Give each item a clear title, `--epic`, `--needs` where there's a real dependency, and `--doc` for any design document an agent must read.
- **Notes.** Notes are short. Don't copy long prose; link the document instead.
- **Source.** Each item's notes must cite where it came from, e.g. "From matter 9449, 'Order to release' step 3".
- **Status from the old data is a claim.** Where old notes say something is done, check git before treating it as done. If you can't confirm it, file it as a question.

## 5. The inbox
File what needs the human in a batch, citing sources with `source`:
- **Decisions agents made that the human never reviewed:** `decided` lines, with `item` or `plan`, and the reasoning in `detail`. Look for sections like "for review", "decisions made" and "assumed".
- **Open questions:** `ask` lines.
- **Loose ends:** `loose` lines. That covers:
  - known bugs and findings that never became work;
  - stale branches and worktrees (name, age, and commits ahead);
  - uncommitted changes;
  - contradictions between the old status fields and what the hub documents claim.

## An example batch
Plan, epics, items and inbox entries in one go. `@` refs point at records created earlier in the same batch:
```
{"op": "plan new", "ref": "p", "slug": "launch", "title": "Public launch", "summary": "Billing and onboarding, then the switch."}
{"op": "epic add", "ref": "billing", "plan": "@p", "title": "Billing", "summary": "Stripe first."}
{"op": "epic add", "ref": "onboard", "plan": "@p", "title": "Onboarding", "needs": ["@billing"]}
{"op": "add", "ref": "hook", "epic": "@billing", "title": "Stripe webhook handler", "notes": "From matter 9449, 'Order to release' step 3.", "doc": ["docs/billing.md"]}
{"op": "add", "ref": "inv", "epic": "@billing", "title": "Invoices page", "needs": ["@hook"]}
{"op": "decided", "item": "@hook", "text": "Store raw webhook payloads", "detail": "Replay after outages", "source": "matter 50ff, For review"}
{"op": "ask", "plan": "@p", "text": "Monthly or annual plans first?", "source": "matter 50ff, Open"}
{"op": "loose", "plan": "@p", "text": "Branch quercus-leafminer-key: 2 commits ahead, last touched 2026-09-08", "source": "git"}
{"op": "idea", "text": "Mobile-first ID tool", "detail": "Field use on phones.", "source": "matter 32a5"}
```
- An item's plan comes from its epic, so `add` needs only `epic`.
- An inbox entry needs `item` or `plan` when the repo has more than one plan.
- The results (one per line, each with its `id`) are the record of what was created.

## 6. The old backlog
Matters with no place in any plan are **not migrated automatically**. List them for the human, grouped by theme, with a one-line summary each. The human decides, usually in batches:
- **ideas:** things worth keeping that aren't work yet. File them as `idea` lines with the matter as `source` and a short note in `detail`. Most old backlog belongs here;
- items in `general`;
- loose ends;
- or left behind.

They stay readable in `.mull/` either way. Don't delete the old `.mull/` directory. That's a separate decision for the human.

## 7. Finish
- Run `mull view` and `mull inbox`, and walk the human through them.
- Fix anything they say is wrong.
- Add the agent protocol (`mull guide agents`) to the project's AGENTS.md, if the human agrees.
- Run `mull sync`.
