# Intake: bringing a project into mull

You're setting this project up in mull, **together with the human**. The human makes every decision; you read, propose, and record what they approve. Work in small steps, and show your proposal before writing anything.

Use the mull CLI for everything you record. Read old material directly: old `.mull/matters/*.md` files, docs, and git history are plain files you may read.

**The new record stands on its own.** Once the intake is done, an agent working here reads mull, not `.mull/`. The old `.mull/` directory stays where it is, untouched, as a read-only archive (files in the repo may still refer to it), but nothing open in mull may depend on it. So carry content over; don't point at it. Citing a matter says where something came from. It never stands in for what the matter says, and a one-line summary of a long matter isn't a carry-over.

**Record with `mull batch`, not shell loops.** Write the approved records as JSON Lines, and check them with `mull batch --dry-run < file.jsonl`. Then run `mull batch < file.jsonl`. A batch is all or nothing: if any line fails, nothing is written and every failed line is reported, so fix those and run it again. Field names are the command's flag names, and `mull guide json` lists every op's fields. Give a line a `"ref"`, and later lines can use `"@ref"` for its ID. That's how phases that need other new phases, items in new phases, and inbox entries about new items go in one batch.

## 0. Already on mull?
If the repo already has a `mull` branch (`mull view` works), its records may come from before Model v2: epics, loose ends, inbox ideas, needs on items, a `general` plan. Bring them up to date first, with the human:
- Run `mull migrate --dry-run` and walk the human through what it would change.
- Once they agree, run `mull migrate --on-request` (the real run is the human's call; it writes one commit). Then carry on below, skipping `mull init`, and fill in what the repo still lacks.

## 1. Look around (read only)
- **Old `.mull/` data:** read `.mull/matters/*.md` (YAML frontmatter plus a body), `.mull/docket.yml`, and `.mull/archive/` if present. Note each matter's status, epic, links (`parent`, `needs`, `blocks`, `relates`) and size.
- **Hub documents:** look for a matter or doc that tracks a large effort (an "overview", "plan", "punchlist" or "roadmap"). It's the best source for plans, phases and their docs.
- **git:**
  - `git branch -a`, `git worktree list`, and `git log --oneline -50` on the main and integration branches;
  - which branches are ahead of the integration branch, and how stale they are;
  - uncommitted changes.
- **The project's own rules and checks:** AGENTS.md / CLAUDE.md, Makefile targets, CI config.

## 2. Set up mull (ask first)
Propose, and wait for approval:
- the integration branch (where work lands) and the release branch;
- for a plan whose work lands on a branch of its own (a long-running feature or integration branch, not the integration branch), that branch as the plan's target: a `plan set` line (`"field": "target"`) after its `plan new`, or `mull plan set <slug> target <branch>`. Seen in git is measured against each plan's target;
- the hard checks `mull done` should run. Ideally that's one make target, e.g. `make done-checks`. If none exists, propose one, but don't create it without approval;
- layers: path globs for `ui`, `tests` and the other main areas of the code.

Then run `mull init --integration <branch> --release <branch>` (unless the repo already has a `mull` branch). Config can go in the first batch:
```
{"op": "config checks", "commands": ["make done-checks"]}
{"op": "config layer", "name": "ui", "globs": ["lib/app_web/**", "assets/**"]}
{"op": "config layer", "name": "tests", "globs": ["test/**"]}
```

## 3. Plans and phases (propose, then record)
- **Plans.** Propose one plan per large effort (from the hub documents), using a plain-English slug. Small, unrelated work is unplanned (`--plan none`).
- **Phases.** For each plan, propose phases: big pieces, in roughly the order they must happen. Add `--needs` between phases only where one truly must finish first. Needs exist between phases only: items inside a phase are ranked (in the order they're added) but never wait on each other, so work that must wait for other work goes in a later phase.
- **Docs.** Propose a doc for each plan (the picture, when it's done, and why the order) and for each phase (its design and when it's done). Carry over everything the hub documents say about what to build and why: the design, the decisions and their reasons, constraints, interfaces, and how to tell it's done. Leave out only status and history (what's done or in progress, and who did what when), since the records say that. A long doc is fine; a summary that sends the reader back to the matter isn't.
- **Show the human the whole outline** (plans, then phases, with needs) and adjust until they approve. Then record it in a batch (see the example below). A short doc fits a `plan doc` or `phase doc` line; for a long one, write the file and run `mull plan doc <slug> - < doc.md` or `mull phase doc <id> - < doc.md` after the batch.
- **Creating.** Only the human creates a plan: once they've approved the outline, each `plan new` line carries `"on-request": true`.
- **Agreeing.** A new plan starts `shaping`: it takes captures and questions, but no phases or committed items until the human agrees it. When they've agreed a plan, add a `plan agree` line with `"on-request": true` right after its `plan new`, before its phases and items. Leave a plan they're still shaping with only its doc, captures and questions.

## 4. Items for work in flight or ready
- **What gets an item.** Only work that's in flight, ready, or clearly next. Each item must be something **one agent can finish in one session**; split anything bigger.
- **Fields.** Give each item a clear title, `--phase`, and `--doc` for any design document an agent must read. Items have no needs; an old matter's `needs` on another matter becomes phase order (or a note) when the two can't run side by side.
- **Notes carry the work.** An item's notes hold everything an agent needs from the old data to do it: what to build, the specifics (files, names, interfaces, data, edge cases), the decisions already made and why, and how to tell it's done. Use the matter's own words where they say it best. From a long hub matter, take the sections about this item rather than the whole matter. Leave out status and history. Notes are markdown and may be as long as the work needs. A design document that lives in the repo, outside `.mull/`, can be linked with `--doc` instead of copied.
- **Source.** Each item's notes also cite where it came from, e.g. "From matter 9449, 'Order to release' step 3". The citation is provenance, never the content.
- **Status from the old data is a claim.** Where old notes say something is done, check git before treating it as done. If you can't confirm it, file it as a question.

## 5. Needs you and the backlog
Needs you is only questions, decisions and reviews. Bugs and ideas are captured items in the backlog, which the human goes through later in triage (`mull guide triage`). File both in a batch, citing sources with `source`. Like items, each one carries what the matter says about it (in `detail` for decisions and captures, in the text for questions), so the human can settle or triage it without opening `.mull/`:
- **Decisions agents made that the human never reviewed:** `decided` lines, with `item` or `plan`, and the reasoning in `detail`. Look for sections like "for review", "decisions made" and "assumed".
- **Open questions:** `ask` lines.
- **Loose ends:** `bug` lines (the backlog, not Needs you). That covers:
  - known bugs and findings that never became work;
  - stale branches and worktrees (name, age, and commits ahead);
  - uncommitted changes;
  - contradictions between the old status fields and what the hub documents claim.

## An example batch
Plan, docs, phases, items, Needs you entries and captures in one go. `@` refs point at records created earlier in the same batch:
```
{"op": "plan new", "ref": "p", "slug": "launch", "title": "Public launch", "summary": "Billing and onboarding, then the switch.", "on-request": true}
{"op": "plan agree", "slug": "@p", "on-request": true}
{"op": "plan doc", "slug": "@p", "doc": "Paid accounts, then the public switch.\n\nDone when anyone can sign up and pay. Billing comes first because onboarding sells a paid plan."}
{"op": "phase add", "ref": "billing", "plan": "@p", "title": "Billing", "summary": "Stripe first."}
{"op": "phase add", "ref": "onboard", "plan": "@p", "title": "Onboarding", "needs": ["@billing"]}
{"op": "phase doc", "id": "@billing", "doc": "Stripe Checkout and webhooks. Done when a payment marks the account paid."}
{"op": "add", "ref": "hook", "phase": "@billing", "title": "Stripe webhook handler", "notes": "Handle Stripe's checkout.session.completed and invoice.paid at POST /webhooks/stripe: verify the signature with STRIPE_WEBHOOK_SECRET, store the raw payload in webhook_events (unique on the event id, so a replay is a no-op), then mark the account paid. Answer 200 once stored; work that fails is retried from webhook_events, not by Stripe. Done when a test payment marks a test account paid, and replaying the same event changes nothing.\n\nFrom matter 9449, 'Order to release' step 3.", "doc": ["docs/billing.md"]}
{"op": "add", "ref": "welcome", "phase": "@onboard", "title": "Welcome flow for paid accounts"}
{"op": "decided", "item": "@hook", "text": "Store raw webhook payloads", "detail": "Replay after outages", "source": "matter 50ff, For review"}
{"op": "ask", "plan": "@p", "text": "Monthly or annual plans first?", "source": "matter 50ff, Open"}
{"op": "bug", "plan": "@p", "title": "Branch quercus-leafminer-key: 2 commits ahead, last touched 2026-09-08", "source": "git"}
{"op": "idea", "title": "Mobile-first ID tool", "detail": "An ID tool laid out for phones in the field: one question per screen, big tap targets, and the photo taken in the app. Raised by two users after the spring walks; nothing designed yet.", "source": "matter 32a5"}
```
- An item's plan comes from its phase, so `add` needs only `phase`.
- An inbox entry needs `item` or `plan` when the repo has more than one plan. Captures (`bug`, `idea`) don't.
- The results (one per line, each with its `id`) are the record of what was created.

## 6. The old backlog
Matters with no place in any plan are **not carried over automatically**. List them for the human, grouped by theme, with a one-line summary each. The human decides, usually in batches:
- **ideas:** things worth keeping that aren't work yet. File them as `idea` lines with the matter as `source`, what the matter says about it in `detail` (enough to triage it and act on it later without opening `.mull/`), and the theme as `category`. Most old backlog belongs here;
- unplanned items (`add` with `plan: "none"`);
- bugs (`bug` lines, with a `category` too);
- or left behind.

Don't delete, move or edit the old `.mull/` directory: it stays as a read-only archive, since files in the repo may refer to it. What's left behind lives only there; what's carried over must not need it.

## 7. Finish
- **Check that the record stands on its own.** Go through every open record (committed and active items, captures, open questions and decisions) and every plan and phase doc. For each one that cites a matter, compare it with the matter: could an agent act on it, or the human settle it, with only mull? If what it needs is still only in `.mull/`, carry it over (see Repairing below). Don't finish until nothing open depends on `.mull/`.
- Run `mull view`, `mull inbox` and `mull list --state captured`, and walk the human through them.
- Fix anything they say is wrong.
- Add the agent protocol (`mull guide agents`) to the project's AGENTS.md, if the human agrees. Above it, a short part about the project: its plans, that small work is unplanned (`--plan none`), the hard checks and the layers.
- Run `mull sync`.

## Repairing an earlier intake
An earlier intake may have left records that only point at old matters: short notes that cite a matter, docs that summarise a hub matter, captures with a one-line `detail`. Fix them in place, with the human's go-ahead. This is carrying over, not re-planning: don't change plans, phases, needs, titles, states or decisions, and don't add or drop work.
1. **List what's open:** `mull list --state committed`, `--state active` and `--state captured`, `mull inbox`, and each plan's and phase's doc (`mull plan doc <slug>`, `mull phase doc <id>`).
2. **Compare each with its source.** For every record that cites a matter, read the matter (for a hub matter, the sections about this record) and note what the record lacks, by the rules in 3, 4 and 5.
3. **Carry it over.** Items and captures: replace the notes with the full text, keeping the citation line (`mull set <id> notes - < notes.md`, or many at once as `set` lines in `mull batch`). Docs: `mull plan doc <slug> - < doc.md` and `mull phase doc <id> - < doc.md`. Inbox entries can't be edited: for an open question or decision that only points at a matter, file a new one that carries the content, and tell the human which old entry it replaces so they can settle it.
4. **Don't guess.** Where a matter's content fits no record, or it's unclear which record it belongs to, ask the human (`mull ask`).
5. **Leave history alone.** Records in review, done or dropped stay as they are.
6. **Finish** with the check in 7. Show the human what changed (how many records, and anything you asked), then run `mull sync`.
