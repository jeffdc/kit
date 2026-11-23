# Intake: setting a project up on mull

Done once per repository, with the human, who approves the result in one sitting. Draft everything first; write nothing to mull or the repo until they've seen it.

1. **Look around (read only).** `mull status`, `mull context`, `mull decisions`, each plan's doc, and the project's CLAUDE.md, AGENTS.md, coding standards, design docs and CI config. Note where rules live today.
2. **Draft RULES.md.** Gather the real rules from all of those places: boundaries between parts, what's shared, what's forbidden, how work runs. Most decision-log entries are implementation detail. Keep only what a future change could break. Give each rule an ID, a one-line title, a scope, and a check when one is possible (an existing script, a grep, the compiler's boundary checks). Say which have no check yet. Aim for 10 to 30 rules.
3. **Draft the settings.**
   - Branches: the release branch, and the integration branch for unplanned work, which isn't the release branch (`next` by default).
   - Each active plan's target, usually `plan/<slug>`.
   - `mull config setup`: what a new worktree needs, such as copying dependencies or a test database.
   - `mull config risk`: the default high-risk paths plus the project's own (auth, data migrations).
   - The hard checks.
4. **Draft the plans.** For each active plan, a short doc: the goal, the appetite, the rules it adds, and phases with checkable done-criteria. Committed items in phases that haven't started go back to phase outlines; they're cut again when the phase starts.
5. **Draft the queue and asks.** Open bugs stay queued, with an area. Ideas go to plan shaping. Only questions that still need the human survive as asks, each with a default.
6. **Show the human** the drafts in one place, short: the rules (what's checked, what's advisory), the settings, and what changes in each plan.
7. **Apply what they approve**, all with `--on-request`.
   - RULES.md (and any ratchet baselines: `mull rules ratchet <ID> --grow --on-request`) is a change to the code: commit it on the integration branch.
   - The records and settings go in one `mull batch`.
   - Run `mull hooks`.
   - Point the project's AGENTS.md at `mull guide agents`, replacing any long copy of the protocol.
   - Done items, reports and the decision log stay as they are: read-only history.

## An example batch

Check it with `mull batch --dry-run < intake.jsonl`, then run it without `--dry-run`.

```
{"op": "config set", "key": "integration", "value": "next", "on-request": true}
{"op": "config checks", "commands": ["make done-checks"], "on-request": true}
{"op": "config risk", "add": ["lib/app/accounts/**"], "on-request": true}
{"op": "config setup", "commands": ["test -d \"$MULL_MAIN/deps\" && cp -R \"$MULL_MAIN/deps\" . || true"], "on-request": true}
{"op": "plan new", "slug": "launch", "title": "Launch", "target": "own", "on-request": true}
{"op": "plan set", "slug": "launch", "field": "doc", "value": "Goal: charge for the app.\n\nAppetite: two weeks.\n\nRules it adds: R12.\n\nDone when: a customer can pay by card."}
{"op": "plan agree", "slug": "launch", "on-request": true}
{"op": "phase add", "ref": "billing", "plan": "launch", "title": "Billing", "summary": "Done when: a card is charged and a receipt sent."}
{"op": "bug", "title": "The footer's pricing link 404s", "area": "lib/app_web/components", "source": "intake"}
{"op": "idea", "plan": "launch", "title": "Annual plans", "source": "intake"}
{"op": "ask", "plan": "launch", "text": "Monthly or annual plans first?", "default": "Monthly first", "by": "3d"}
```
