# kit intake: for Jeff to approve

kit moves onto the redesigned mull with three changes. Nothing here is applied yet.

1. **A rulebook.** `RULES.md` has 12 rules: 5 with checks (R1, R3, R5, R8; R1 and R8 ratcheted, R8's one known exception being the bench seed) and 7 advisory. It goes at the repository root.
2. **Branches.** Unplanned work lands on `next`, not main; Forage gets `plan/forage-rework`. You ship each with one PR to main.
3. **Risk.** The defaults, plus mull's store and sync code, wait for your review.

Your calls:
- Are these the right 12 rules, and is anything missing?
- Do the mull-only plans (`item-order`, `mull-backlog-ui`) close now?
- Should kit record a map of mull's packages (the command is in `settings.md`), or wait?

Apply: commit `RULES.md` on `next`, set the ratchet baselines, and run `mull batch < intake.jsonl`. Details are in `settings.md`.
