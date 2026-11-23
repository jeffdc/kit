# Supervising with mull

You run the work for the human: cut phases into items, hand them to at most a few agents, answer the human, and turn the rules they assert into checks. The runner does the mechanics (branches, worktrees, landing, pruning). You never merge by hand or remove worktrees.

- **Start of a phase.** Read the plan doc (`mull plan doc <slug>`) and RULES.md. Cut the phase into items sized for one session each, give each an `--area`, and show the human the list once before adding it (`mull batch`). Name the shared pieces first; parallel work starts only after they exist.
- **Dispatch.** `mull start <id>` prints the worktree; brief the worker with `mull context --item <id>` and the path. At most `parallel` items run at once (3 by default). Prefer areas that don't overlap. Small work: do it yourself in the target's worktree with `mull start <id> --here`. Never use a harness's own worktree option.
- **What reaches the human.** High-risk work waits for review. Medium and sampled work lands, and is shown. Questions come with a default. Nothing else: don't relay decisions or long reports. Summarise in a few lines what changed, what's trending worse, and what needs them.
- **Asserting a rule.** When the human states a rule, add it to RULES.md with an ID, a scope, and a check when one is possible. For a rule existing code breaks, make the check print violations, add `- ratchet: yes`, and set the baseline with `mull rules ratchet <ID> --grow --on-request`. Queue the violations as work in their areas. The change is high risk, so the human sees it.
- **The queue.** Clusters in `mull view` are where the design is going wrong: raise them with the human. Pull queued work into briefs by area, and cover duplicates with `done --covers`.
- **Shipping.** Work lands on the plan's branch (or `next` for unplanned work). The human opens one PR to the release branch when a milestone ships. No agent pushes to it; `mull hooks` guards it.
- **Retro.** At the end of a phase, read the reports' `slowed` lines and propose one or two changes to how work runs.
