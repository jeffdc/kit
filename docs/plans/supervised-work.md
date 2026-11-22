# Supervised work: where it lands, how it's previewed, what workers are told

## The picture

Jeff used mull to supervise a multi-agent rebuild of the oaks Phoenix site: one supervisor session handing items to worker agents in git worktrees, up to five in parallel, with Jeff reviewing. Claiming, capturing and reviewing held up. Where mull fell short, the supervisor improvised, and Jeff had to chase the result:

- items were done while their code sat unmerged in a worktree, and nothing said when work reached the branch Jeff was looking at (the integration branch `6b12-site-shell`, not main);
- tours pointed at per-worker dev servers that had died;
- every worker bootstrapped its worktree by hand, and 22 worktrees (5.5 GB) were left behind after acceptance;
- the rules workers followed lived in ad-hoc supervisor prompts, which contradicted each other ("use existing components" and "don't edit shared files"), so five page agents each built their own widths, helpers, buttons and dialogs, every per-item review passed, and the site came out inconsistent;
- parallel items collided in one big CSS file, and nobody reviewed the combined result until everything had merged;
- Jeff's own decisions, made in conversation, were recorded as agent decisions with "Jeff chose" in the detail;
- small changes Jeff asked for after an item was accepted were committed under the old item's trailer;
- the same bug was captured twice, and `mull batch` takes `text` for `bug` but `title` for `add`.

This plan fixes those while keeping mull's model small. Git stays the evidence: where work landed and what files it touched are derived, never stored.

## What gets built

1. **Small fixes.**
   - `bug` and `idea` output a `similar` list (open items whose titles overlap a lot in their words): a hint, never a block. The protocol says to check it and ask before keeping a near-duplicate.
   - In batch, `bug` and `idea` take `title`, with `text` kept as a silent alias. `decided` and `ask` keep `text`.
2. **Jeff's own decisions.** `mull decided "…" --human --on-request` records a decision Jeff made directly: settled at once, by Jeff (via an agent, on request), straight into the decision log, never in the inbox.
3. **Follow-ups after acceptance.**
   - `mull add "…" --follow-up <done-id>` creates a committed item in the same plan and phase, with `from` set to the done item. No reopening: done means reviewed, and the old report stays as it was.
   - New problem: commits carrying a done item's trailer after it was accepted.
4. **A target branch per plan.**
   - A plan's `target` (`mull plan set <slug> target <branch>`) falls back to config `integration`. There are no phase-level targets.
   - "Seen in git" is measured against the plan's target. The merge time is derived from the commit that brought the work in, never stored.
   - Accepted but not in the target is a problem at once, not after three days. In review and already merged is normal.
   - The view shows each plan's target and how far it is ahead of release.
5. **A preview per plan.**
   - A plan's `preview` URL serves its target branch, and `mull context --item` hands both to the worker. mull records the preview and never runs or monitors the server.
   - Tours give paths (`/trees/quercus-alba`), which `mull serve` and `report --text` resolve against the preview.
   - `done` warns (soft) when a tour carries a full `localhost:` URL that isn't the preview.
   - The intended order, which the supervisor guide spells out: done, then the supervisor reviews the diff and merges into the target, then Jeff walks the tour on the preview, then accept.
6. **Standing worker rules.**
   - A `rules` doc on a plan, and optionally on a phase: plain markdown, Jeff's, printed in full by `mull context --item` (the plan's, then the phase's).
   - A plan may add its own hard checks, which `done` runs after config's.
   - The supervisor guide keeps the hand-off one line: anything else a worker needs goes in the rules or the item's notes, never in a prompt.
   - Required skills are harness-specific, so they're only text in the rules.
7. **Shared areas and the integration review gate.**
   - Overlap is derived from git: for each pair of items in flight, the files both branches changed (claim base to branch tip) show as a problem in the view.
   - A plan's rules may list `shared` globs, and `done` flags changes to them in the report.
   - A phase may require an integration review (`mull phase set <id> integration-review on`, Jeff's call). Once all its items are done, it shows "awaiting integration review" instead of complete, and phases that need it stay blocked.
   - The supervisor, in a worktree on the target, runs `mull done <phase-id> --summary --tour --design -`. That runs the checks on the target tip, writes a report with the phase's combined diff, and files a review entry on the phase.
   - Accepting that review completes the phase. `do --reason` creates a committed fix-up item in the phase, which reopens it.
8. **Worktree lifecycle, through the guides.** mull doesn't create or remove worktrees: a worktree isn't always needed, and creating one in `mull start` would force it (decided on question c6c0).
   - `mull guide agents` and `mull guide supervisor` say how to handle it: when an item gets its own worktree, branch `<id>-<name>` from the plan's target, set it up as the plan's rules say (copying deps, the dev and test databases), and once the item is accepted and its work is in the target, the supervisor removes the worktree and its merged branch.
   - Project-specific setup lives in the plan's rules (6) or the project's AGENTS.md, not in mull config.
   - `mull worktrees` lists claim worktrees with their state and disk size, read-only. A worktree is prunable when its item is done, its work is in the target, and it's clean, and the list gives the `git worktree remove` and `git branch -d` commands for it. The view shows the prunable count and size as a problem.

## Not in mull

- Running, restarting or health-checking preview servers.
- Enforcing skills, or anything else that depends on a harness.
- Declared per-item areas. Overlap comes from what branches actually changed. Add declarations only if that turns out to come too late.
- Phase-level target branches.
- A stored merge time.
- Creating, bootstrapping or removing worktrees. The guides tell agents how, and mull only lists the prunable ones.
- The sequencing that would have prevented the inconsistent site (a shared-components phase that the page phases need). That's planning, and phase needs already cover it.

## Why the order

The small fixes (1 to 3) are independent and cheap, so they come first. The plan target (4) underpins the preview (5), the integration review gate (7) and pruning (8), because each needs to know where work goes. Rules (6) come before the overlap and gate work (7), since the `shared` globs live in the rules. Worktree lifecycle (8) is last: it needs the target, and its guide text needs the rules doc.

## Done when

The next oaks-style run needs no supervisor improvisation for any of the failures above:
- the view shows done-but-unmerged work against the plan's target;
- tours resolve against a live preview;
- workers get their rules from `mull context`;
- overlapping branches and an unreviewed whole phase show up before Jeff has to find them;
- workers know from the guides how to set up a worktree, and leftover worktrees show up as prunable;
- Jeff's decisions and follow-ups have their own commands.
