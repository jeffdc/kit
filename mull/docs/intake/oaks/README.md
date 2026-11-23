# oaks intake: one sitting

Drafted 2026-11-23. Nothing has been written to oaks or its mull branch. The batch passed `mull batch --dry-run` on a scratch copy of the oaks mull branch, all 48 lines.

## What changes

- **Rules:** 27 rules in `RULES.md`. 12 have a check (3 of them ratcheted: web-to-Repo calls, `raw(`, page CSS restyling components); 15 are advisory. Most of the 341 logged decisions were implementation detail and stay as history.
- **Branches:** unplanned work lands on `next`, not main. Compendium lands on `plan/compendium`, and your 4460 preview serves that branch. Main changes only through your PR when a milestone ships.
- **Plans:** compendium gets a short doc and per-phase done-criteria. 34 committed items in the 7 phases that haven't started go back to outlines. Sources and facts core starts now, so it keeps its 5 items and gains a Boundary item. gallformers-parity is left with 2 ports. Search is still shaping. audit-2026-11 closes.
- **Queue:** 13 bugs, each with an area. Ideas are tagged to the phase that will shape them.
- **Inbox:** 9 of the 12 Polish reviews are accepted in the batch. 3 are left for you to look at. Of the old questions, only 3 survive as asks, each with a default.
- **Worktrees:** `mull start` copies deps, _build, the PLTs and the databases into each new worktree, so compiling starts warm.
- **Risk:** auth, migrations, the import endpoint, the router, plugs, deploy files and checks are high risk and wait for you. Everything else lands, and 20% of it is sampled.

## Your decisions

1. **Rules:** cut or reword any of the 27, especially R6. R6 replaces CLAUDE.md's "stop, present findings, wait for approval" with "decide locally; ask only for a rule conflict or something irreversible".
2. **Appetites:** about 6 weeks for compendium, 3 to 5 days a phase (plans.md). Change any of them.
3. **The component library** (`lib/oaks_web/components/**`): should it be high risk, so changes wait for you as AGENTS.md says today? It's in the batch as high risk. Take that line out of `config risk` to leave UI work ungated, as the redesign says.
4. **Outlines:** drop the 34 not-started items back to outlines?
5. **Reviews:** look at c0d4 (auth), 3fa9 (security headers) and 0edd (the new Field Face Rule in DESIGN.md), then `mull accept c0d4 3fa9 0edd --on-request`.

## How to apply (from ~/dev/oaks, on the integration branch)

1. Commit `RULES.md` to `next` with its ratchet baselines (`mull rules ratchet R8 --grow --on-request`, the same for R11 and R17). In the same commit, update two docs. In AGENTS.md, point "Working with mull" at `mull guide agents` and say `.mull/ratchets/` is the exception to ".mull is read-only". In CLAUDE.md, change "Investigation before action" to match R6.
2. `mull batch --dry-run < intake.jsonl`, then run it without `--dry-run`.
3. Skip `mull hooks`. oaks's own pre-push guard already blocks pushes to main and force pushes, and R2 checks that it's installed.
4. When the Boundary item is done, run the two `mull config map` commands in settings.md.

## Fixed in mull since drafting

- A phase whose items are all dropped is no longer complete: it takes one done item. Decision 4 is safe to apply.
- Shrinking a ratchet after a fix no longer makes work high risk; a ratchet that grows does.
- A rule check that's also a hard check isn't run twice.
- `mull guide json` lists `config risk`, `config setup`, `area`, `default` and `by`.

Still open: ratchet entries can't carry reasons, so each rule's text says why its known violations are allowed.
