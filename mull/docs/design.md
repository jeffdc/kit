# mull: design

Status: revision 3, 2026-11-10 (Model v2 and `mull serve`). mull is a from-scratch rewrite of old mull, and none of the old code is reused. It replaced old mull at the 2026-11-09 cutover (see "Transition").

Background:
- `serve/`: the design of `mull serve`, and in `serve/README.md` the Model v2 summary and the decisions of 2026-11-10.
- `research/prior-art.md`: what exists and what to borrow.
- `research/taxogall-recast.md`: the target view, filled in with real gallformers data.
- `research/superseded/`: earlier drafts, kept for history.

## Why

On large work run by many agents, you lose track of the program.

- **Today you can't see:** what the plan is, where it stands, what's in flight vs done, or what needs your input.
- **Agents report "done" in prose,** and nothing checks it.
- **Decisions agents made pile up unreviewed.** In gallformers, 21 of them existed "only in a session summary".
- **Loose ends fall through the cracks.**

Old mull made this worse. Its matters grew into long agent-written prose, statuses drifted from reality, and the per-worktree copies of `.mull/` diverged.

New mull is the record of a program: its plans, where they stand, and everything that needs you.

## Principles

1. **Any LLM, any harness.** The only interfaces are a CLI and git. Agents learn the protocol from a plain-text block in `AGENTS.md`. Nothing depends on Claude Code, Codex or any other harness. The test for every feature: it works with you typing the commands. `mull serve`, the human's web view, reads the same records and changes none of this.
2. **Agents never edit mull's data files.** They read and write through the CLI. Long design documents are not mull data: they stay as ordinary files in the repo, and records link to them.
3. **What agents write is a claim; git and checks are evidence.** An item reaches review only when mull itself has run the project's hard checks and they passed, and it's done only once reviewed. Whether its work has landed is shown separately, as *seen in git*.
4. **You stay involved, not sidelined.** Plans are yours. Decisions agents make come to you for ratification. Finished items come to you for review. Everything that needs you is in one queue, and you settle each entry with one action.
5. **It holds up when the model has a bad week.** An item is sized for one agent in one session. Checks are deterministic and run by mull. Problems surface fast.
6. **Limits are soft and loud,** with one exception: an item can't reach review until its checks pass. Everything else is shown, never blocked.

## Data model: three levels

| Record | What it is | Fields | Who writes it |
|---|---|---|---|
| **Plan** | A large effort, such as `taxogall`. High-level only. | id (plain-English slug, e.g. `taxogall`, `general`), title, summary, doc, rules, its own hard checks, state `shaping`, `active` or `done`, target (the branch its work lands on; config's integration branch when unset) | You |
| **Phase** | A big piece of a plan; a set of items. Called an epic before Model v2. | id (4-hex), plan (required), title, summary, doc, rules, needs (phases only), from | You, or agents on your instruction |
| **Item** | A unit of work **one agent completes in one session.** | id (4-hex), plan (optional), phase (optional), rank, title, notes, docs (paths), stage, claim, from (the inbox entry it was made from, or the done item it follows up; no needs: see "Ordering") | Agents and you |
| **Captured item** | The backlog: a bug or an idea, not work until you commit it (see "The backlog") | an item at stage `captured`, plus kind (`bug` or `idea`), category, source, triaged; a short note as its notes; optionally tagged to a plan or phase | Anyone captures; only you commit |
| **Inbox entry** | Something that needs you | id, kind (`decision`, `question` or `review`), one-line text, detail, item, plan, source, by, created; a review also lists the decisions it carries; when settled: action, note, what it made, by, time | Agents capture; you settle |
| **Done report** | The record of a finished item | item, commit SHA, tree hash, summary, UI tour, check results, design notes, decisions made in the session, system context | Agent plus mull |
| **Config** | Per repo | integration branch, release branch, hard checks, layers, remote, storage format | You |

**Structure rules:**
- Every phase belongs to a plan. An item belongs to a plan or to none: committed work with no plan is **unplanned** (`--plan none`), and a captured item with no plan is backlog not tagged to a plan.
- **The Archive** is finished work, derived, never stored: a done (closed) plan and everything in it, and unplanned work that's done or dropped with no entry on it open or parked. Unplanned work has no plan to close, so finishing it archives it. `mull serve` lists the Archive at the foot of each project; the CLI shows it as it always has. Done work in a plan that isn't done isn't archived. Find (in `mull serve`) doesn't tell the two apart: its status filter has Open (live work) and Finished, which is done, dropped and settled records and the whole Archive alike.
- An item in a plan may sit directly under it with no phase. The view shows those as "(no phase)" in that plan.
- Before Model v2, `mull init` created one standing plan, `general`, for work outside any big effort. Model v2 has unplanned work instead, and `mull migrate` retires `general`.
- No docket, no sessions, no free-floating phases.
- **A phase no longer needed is deleted** (`mull phase delete <id>`, the human's call), not dropped: only one with no committed work in any stage, so nothing done is lost, and the mull branch's history keeps it. Captured items tagged to it move up to its plan, and other phases' needs on it are removed. A phase whose items are all done or dropped is complete, not unneeded, and stays.

**Plan states** (Model v2): `shaping → active → done`.
- `mull plan new` creates a plan `shaping`, and only on your explicit ask (`--on-request`): its doc and questions are being worked out with you. It takes captures and inbox entries, but refuses phases and committed items (and starting a capture tagged to it) until you agree it.
- `mull plan agree <slug>` makes it `active`. Agreeing is yours: an agent must pass `--on-request`, and mull refuses otherwise.
- `mull plan close <slug>` makes it `done`, from either state. A done plan takes no new work and leaves the view (`view --plan` still shows it).
- Data from before Model v2 stores a done plan as `closed` (see "Formats and `mull migrate`").

**Item stages** (Model v2, see `serve/README.md`): `captured → committed → active → in-review → done`, plus `dropped` from any stage before done.

| Stage | What triggers it |
|---|---|
| `captured` | The item is in the backlog, not committed to (`mull bug`, `mull idea`) |
| `committed` | The item is committed work. `mull add` creates items here, and `mull commit` moves a captured one here. `release` and a review sent back return an item here |
| `active` | The item is claimed by an agent (see "Claims"). `start` works from captured or committed |
| `in-review` | Only through `mull done`, after the hard checks pass |
| `done` | The work is reviewed and accepted (see "Review") |

Because `start` works from captured, a small bug can go from the backlog to done in one session with no planning step. Review is never skipped.

**Unreviewed work is incomplete work.** An item counts as finished only once it's `done`, which means reviewed.
- A phase's progress counts done items out of its committed items, with items in review shown separately. Captured items tagged to a phase are counted on their own and never hold it open.
- A phase is complete when it has committed items and all of them are done or dropped.
- Data from before Model v2 stores stages under their old names (`open`, `done`, `accepted` for committed, in-review and done), under a different key, so the two are never confused (see "Formats and `mull migrate`").

**Triage** (Model v2) is a process the human runs with an agent (`mull guide triage`), not a screen. Each captured item is committed, tagged to a plan or phase (`mull set <id> plan|phase`, captured items only, so tagging never commits), started, dropped or left. `mull triage <id…> --on-request` (or `--all [--plan <slug>|none]`) stamps an item's `triaged` time; a captured item with no stamp is untriaged. The view shows only the untriaged count: for the project, on each board, and in each progress line.

### Ordering: phases need phases, items are ranked

Two kinds of order, and only one of them holds work back (Model v2, and plan `item-order` for rank):
- A phase may **need** other phases. That is the only dependency: items have no needs. A phase's need is met when the needed phase is complete.
- "Can start now" is derived: the phases whose needs are done.
- **Ready** means committed, in a phase whose needs are done (or in no phase), in a plan that isn't shaping.
- **Blocked** means committed, in a phase with at least one need not done.
- Cycles are refused when written.
- Needs from before Model v2 are kept until `mull migrate` converts them: an item's needs are shown but ignored (migrate drops them), and a phase's needs on items still count (migrate makes each a need on that item's phase).
- **Rank** orders open work (committed, active, in review) within its **group**: a phase, a plan's items with no phase, or Unplanned. Ranks never compare across groups. It's an item's `rank`, a sparse integer (10, 20, 30…), lower first; equal ranks mean no order between them. It sorts and warns, and never changes readiness.
- **Agents give an order, not numbers.** `mull rank <phase|plan|none> <ids…>` sets a group's whole order in one commit: it must list every open item in the group, first to last, and refuses with the missing IDs otherwise. mull writes ranks 10 apart. Committed work joins its group last (`add`, `commit`, starting a captured item, `set <id> phase` to another group), or `add --before`/`--after` an open item, slotted into the gap or renumbering the group when there is none. Agents reorder only when the human says what comes first, without `--on-request`; a reorder files no decision, and mull serve's change cloud shows it.
- **Captured items** have no rank; **done and dropped** items keep theirs and list after open work. **Unranked** items exist only from before ranks: they sort after ranked ones, oldest first, and their group's next placement ranks them all in that order. No migration.
- **Work order** is how every CLI list goes: plan by plan (by slug, Unplanned last); within a plan, its phases in schedule order (by depth of needs not yet done, then age; done phases last), then its items with no phase; within a group, open work by rank, then the rest. `mull ready [--plan p]`, `mull context` (whose ready list is cut at 15, so the top is what's kept), `mull list`, `mull show <phase|plan>`, `mull plan show` and the view's work in flight all use it. A supervisor agent hands out work from the top of `mull ready`.
- **Rank is loud.** `mull start` on committed work while open work ranked before it in its group isn't done (committed, active or in review) still succeeds, with a warning naming each of those items and its stage. Equal ranks and unranked items don't warn, and nor does a captured item started now, since nobody ranked it.
- The view sorts phases by readiness (can start, blocked, complete), then by age.

### Claims (several agents at once)

- `mull start <id>` claims an item and sets it `active`.
- The claim is atomic: the write is a compare-and-swap (see "Storage"). If two agents start the same item, one wins and the other gets an error.
- A claim records the worktree, the branch and the time. The worktree is the agent's identity.
- `mull release <id>` gives a claim back, and the item returns to committed.
- A claim with no linked commits for a day shows up as a problem in the view.

### Review

Most items are reviewed in the session that finished them, by you or by a supervising agent. The inbox is the safety net for any that slip through.

- **Review entry.** `mull done` always files a `review` entry. Accepting it marks the item `done`, and `mull accept <item-id>` works directly on the item too.
- **Review is what and how** (Model v2). The report holds what was built (summary, tour, checks) and how (design notes, the decisions made in the session, files by layer), and they're settled together:
  - the review carries every open decision on its item when `mull done` files it (unsettled and not parked), listed on the entry as `decisions`;
  - accepting the review ratifies those still unsettled. Deciding one differently (`mull decide <decision-id> "…"`) while the work waits for review sends the work back: the item returns to committed with the decision in its notes, and it can't be accepted until an agent takes the decision in (saying how it makes the change with `mull decided`) and files it for review again. `mull accept <decision-id>` alone still ratifies just that one, without accepting the work;
  - sending the review back leaves them open, and the next `mull done` carries them again. Decisions captured while the item is in review aren't added to that review.
- **In session:** you or the supervisor accept right after reviewing the done report and the tour, so the entry never lingers.
- **Who may accept:**
  - a supervising agent, working from another worktree;
  - you;
  - the worker itself, only with `--on-request`, meaning you told it to in conversation.

  An accept run from the item's own claim worktree without `--on-request` is refused. That's a cheap, deterministic guard against an agent approving its own work.
- **Anything not reviewed stays in NEEDS YOU** as a review entry until someone settles it.

### The backlog

The backlog is the captured items: bugs and ideas. A captured item is a title, a short note, a source, a free-text category, and optionally the plan or phase it's tagged to. It isn't work yet, so it:
- needs no sizing and is never ready;
- counts toward no progress: plans and phases count their captured items on their own, and they never hold a phase open;
- stays out of `inbox` and NEEDS YOU, so it never nags;
- shows on the board only as counts: captured and untriaged per plan and phase, plus the project's untriaged total. `mull list --state captured` lists them all, and `--untriaged` or `--kind bug|idea` narrows the list.

**Anyone captures; only the human commits.**
- `mull bug "…"` captures something out of scope that's wrong: a bug, an inconsistency, half-finished work, an assumption. Feedback the human hands over for later goes in the same way, in their words, with the human as its source.
- `mull idea "…"` captures a possibility worth keeping that isn't work yet, whether the human raised it or an agent noticed it. In intake, most of the old backlog fits here.
- Both print `similar`: up to five open items, captured or committed, whose titles overlap the new one's a lot in their words (rarer words count for more, so a prefix most titles share counts for little). It's a hint, never a block: the capture is filed either way, and the agent protocol says to check it and ask the human before keeping a near-duplicate.
- `mull commit <id>` makes it committed work, optionally moving it to a plan or phase (`--plan none` for unplanned work). An agent must pass `--on-request`, meaning the human asked, and mull refuses otherwise.
- `mull start <id>` on a captured item commits it too, for a small fix the human wants done now. A capture tagged to a shaping plan can't be started.
- `mull drop <id>` drops it.
- The human goes through the backlog with an agent in triage (see "Triage" above).

A committed item keeps the kind, category and source it was captured with.

**Before Model v2** (history): agents filed out-of-scope findings as inbox entries of kind `loose-end`, and ideas the human raised as inbox entries of kind `idea`, promoted with `mull do <idea> --plan <p> [--as epic]`. Model v2 keeps NEEDS YOU for attention only, so captured items replaced both, and `mull loose` and `mull ideas` are retired (code `retired`). Loose ends and inbox ideas already filed are kept until `mull migrate` turns them into captured items. Until then they're listed only by `mull inbox --kind loose-end|idea`, counted in the view as legacy, never in NEEDS YOU, and settled as before: a loose end like a question, and an idea with do, drop or park, where promoting it still needs `--on-request`.

### The inbox

The inbox holds only what needs you: questions, decisions and reviews.

| Action | Applies to | Effect |
|---|---|---|
| **Accept** | decisions, reviews | Decision: ratified as recorded. Review: the item becomes `done`, and the decisions the review carries are ratified (see "Review"). `--follow-up "text"` also creates a new item in the same plan and phase |
| **Decide** `"text"` | decisions, questions | You write the decision (overruling an agent's decision if need be), and it joins the decision log |
| **Do** | questions, reviews | Question: becomes a new committed item (`--as phase` for a phase). Review: **sends the work back**: the item returns to committed for another session, with `--reason` added to its notes |
| **Drop** | decisions, questions | Gone for good, with the drop recorded so it doesn't resurface |
| **Park** `"reason"` | any | Out of the queue, still listed with `--parked`, and can be settled later |

A review can't be decided or dropped, because both would leave unreviewed work looking finished. To abandon the work, drop the item itself.

- Accepted and decided entries make up the **decision log**. Agents read it before starting work, so a settled question stays settled.
- **Your own decisions** made in conversation go straight into the log: an agent records one with `mull decided "…" --human --on-request` (`--item` or `--plan` as for any decision, `--detail` for why). It's a decision entry settled at once as decided, by the agent's worktree on request, so it never reaches the inbox and no review carries it. Without `--on-request` it's refused (`not_allowed`); `--on-request` without `--human` is refused too (`invalid_argument`), since an agent's own decision needs neither.
- An agent may settle entries **on your explicit instruction in conversation**. The record notes `by: agent, on request`.

## Storage: the `mull` branch

All data lives on a dedicated git branch, `mull`, which is never checked out. Its history is separate from the code: it starts with its own first commit and contains only mull data.

```
config.yml
plans/<slug>.md
phases/<id>.md                 # epics/<id>.md before Model v2
items/<id>.md
inbox/<id>.md
reports/<item>-<n>.md
checks/<tree-hash>.json        # cached hard-check results
docs/plans/<slug>.md           # a plan's doc
docs/phases/<id>.md            # a phase's doc (docs/epics/ before Model v2)
rules/plans/<slug>.md          # a plan's rules
rules/phases/<id>.md           # a phase's rules
```

There's one file per record, so concurrent writes and merges rarely conflict.

**Docs** (Model v2). A plan's doc is the picture, done-when and why the order; a phase's doc is its design and done-when. Docs hold intent only, never status, which comes from the records and shows beside the doc.
- A doc is plain markdown with no frontmatter, in its own file at `docs/` plus its record's path. Writing it never touches the record, and changing the record never touches the doc, so the doc's history is exactly its own versions.
- `mull plan doc <slug> [-]` and `mull phase doc <id> [-]` print a doc or replace it from stdin; empty stdin removes it. `--log` lists its versions, newest first, read from the commits that changed the file.
- A plan's or phase's `summary` is separate and unchanged. `mull migrate` turned summaries into doc stubs.

**Rules.** A plan, and optionally a phase, may have rules: the human's standing rules for the agents working in it (how to set up a worktree, what to use and reuse, what not to touch, skills to load). They're what a supervisor would otherwise repeat in every worker's prompt, so they live where every worker reads them, and the supervisor's hand-off stays one line (`mull guide supervisor`).
- Rules are kept as a doc is: plain markdown in their own file, at `rules/` plus the record's path, so writing them touches neither the record nor the doc, and their history is their own. Only format 2 has them.
- `mull plan rules <slug> [-]` and `mull phase rules <id> [-]` print or replace them, as `plan doc` does a doc, with `--log` and `--text`. Like docs, they're the human's: agents change them only when asked.
- `mull context --item` gives the item's plan's rules, then its phase's (`plan_rules`, `phase_rules`), in full, and its protocol then ends with a line saying to follow them. mull never checks that they're followed: anything harness-specific, such as a required skill, is only text in the rules.
- `mull serve` shows and edits them under the doc: a plan's on its Plan doc tab, a phase's on its sheet.

**Writes:**
- Git plumbing: a temporary index, `write-tree`, `commit-tree`, then `update-ref refs/heads/mull <new> <old>`.
- That last step is a compare-and-swap. On a race it re-reads, re-applies and retries. Claims rely on this.
- Writes work from any worktree on any branch and never touch the working tree.

**Reading:** `ls-tree` plus `cat-file --batch`. Every worktree sees the same branch, because branches live in the repo's shared git directory.

### Formats and `mull migrate`

`config.yml` records the data's storage **format**. Format 1 is the data from before Model v2 (a config with no `format`); format 2 is Model v2's, and `mull init` writes it.

| | Format 1 | Format 2 |
|---|---|---|
| Phases | `epics/<id>.md`, docs at `docs/epics/<id>.md` | `phases/<id>.md`, docs at `docs/phases/<id>.md` |
| An item's phase | `epic:` | `phase:` |
| An item's stage | `state:` with `open`, `done`, `accepted` for committed, in review, done | `stage:` with the Model v2 names |
| A done plan | `state: closed` | `state: done` |
| A settlement's phase | `epic:` | `phase:` |

**Each record shows its own format.** A stored `done` means in review in format 1 and done in format 2, so items use a different key (`state:` or `stage:`) in each, and a record is never read in the wrong one.

**This mull reads both and writes only format 2.**
- On format 1 data, reads work and every write fails with `migration_required`, pointing at `mull migrate`. There's no format 1 writer to keep, and nothing writes old names under the new format or new names under the old.
- Format 1 records can still reach migrated data: `sync` merges work from a machine on an older mull. While any are present, writes fail the same way, and `mull migrate` converts them.
- A newer format than mull knows fails every command with `unsupported_format`.
- The view shows `format`, and `migration_required` while writes are refused. `context` tells the agent to stop and tell the human.
- The older binary doesn't know any of this, and writes format 1 into migrated data. Install the new mull on every machine before migrating.

**`mull migrate`** converts a repository in one commit. It isn't a batch op, because it reaches across every record.
- `--dry-run` only reads, and prints exactly what would change: counts, every record with its changes, the loose ends and ideas it captures, the needs it converts or drops, and warnings. `--text` renders it for the human.
- The real run is the human's call: an agent passes `--on-request`. It settles inbox entries and retires a plan, as `plan new` and settling need it.
- It's idempotent: on migrated data it writes nothing.
- **Format:** it rewrites every record in format 1, moves `epics/` and `docs/epics/` to `phases/` and `docs/phases/` (`phase doc --log` reads both paths, so a doc's history survives the move), and records `format: 2`.
- **Loose ends and inbox ideas,** open or parked, become captured items of kind `idea`: title, detail as notes (with the item it was filed on, and a parked entry's reason), source, its plan (none if that plan is done), `from` the entry, and the entry's created time. They're untriaged, so triage reaches them, and the human re-kinds the bugs with `mull set <id> kind bug`. Each entry is settled `do`, pointing at its item, so the history stays. Settled entries are left as they are.
- **Needs:** a phase's need on an item becomes a need on that item's phase. Waiting for a whole phase is stricter than waiting for one item, but it keeps the phase blocked rather than freeing it early. It's dropped instead when the item is done or dropped, has no phase, is in the same phase, or the need would make a cycle. Items' needs are dropped, and an unfinished item keeps them as a line in its notes.
- **First migration only** (format 1 to 2), so a rerun never redoes them:
  - the standing `general` plan is retired, because Model v2 has no standing plan. Its items become unplanned (or untagged backlog), its entries project-wide, and the plan `done`. A `general` plan with phases is real work, so it stays as it is, with a warning;
  - every plan and phase with a `summary` and no doc gets a doc stub holding the summary, except a retired `general`. The summary stays, as decided for docs.
- **Times:** a record rewritten only for the format keeps its `updated` time. One whose meaning changes (a plan retired, an item unplanned, needs converted or dropped) gets the migration's time.

### `mull sync`, and why it can only push the `mull` branch

`mull sync` calls `git push`, and it's the only command that does. Because it can only ever push the `mull` branch, **agents may run it too**. The agent protocol has them sync after `done` and at the end of a session. Safeguards:

1. **A fixed refspec.** It pushes exactly `refs/heads/mull:refs/heads/mull`, built from a constant in the code. The branch name isn't configurable, and there's no code path that pushes any other ref. It never runs a bare `git push`, `--all`, `--mirror` or `--tags`.
2. **Never forced.** There's no `--force` or `+` refspec. If the remote has moved, sync fetches `refs/heads/mull`, merges (`git merge-tree --write-tree`; Git 2.38+, this machine has 2.39) and pushes the merge. If the same record was changed on both sides, sync stops and lists the conflicts.
3. **Only mull data leaves the machine.** The `mull` branch's history contains only mull's records, so the push can't carry code objects. Before pushing, sync checks that the commits it's about to send touch only paths in the layout above, and refuses otherwise.
4. **Hooks run.** It never passes `--no-verify`, so your pre-push hook still applies.
5. **A preview is available.** `mull sync --dry-run` prints the exact git commands and the commits that would be pushed.
6. **Tests assert the exact argument list** passed to `git push`, against a local bare remote.

The remote defaults to `origin` and can be changed in config. Pushed commits carry your git identity, as any commit does.

**As built:**
- Each push is `git push --quiet <remote> <sha>:refs/heads/mull`, where `<sha>` is the commit just verified, so a branch moving mid-sync can't slip unchecked commits in.
- The path check is a single `git log --name-only` over every outgoing commit, matched against an explicit list of record paths.
- Fetching also updates the local remote-tracking ref `refs/remotes/<remote>/mull`, as any `git fetch` does. That ref is local and never pushed.

**A second machine** needs no `init`: `mull sync` adopts the remote's branch.

## Linking work to git

An item ID appears in **both** places:
- **Branch name:** `f923-tree-facts`. After a merge, the merge commit's message still carries the branch name.
- **Commit trailer:** `Mull: f923`.

mull derives from git:

| Mark | Meaning |
|---|---|
| **Linked commits** | Commits with the trailer, or reached through a branch or merge carrying the ID |
| **Seen in git** | The item's linked commits are reachable from its plan's target branch, so its work has landed. Shown next to `done`, never a gate |
| **Merged at** | When the work came into the target: the date of the target's first-parent commit that brought in the item's latest linked commit (the merge, or the commit itself when it was made on the target). Derived each time, never stored |
| **Problems** | An active claim with no linked commits for a day; an item in review with nothing seen in git on its target after three days, or accepted (done) with nothing there at once (work in review may not be merged yet, but accepted work belongs on the target); a failed done attempt; recent commits on an item nobody has claimed; commits carrying a done item's trailer, on any branch, written after it was accepted (see "Follow-ups"); commits on the integration branch with no link, shown as a count |

Bare 4-hex mentions in commit prose are not counted, because they collide with hashes.

**A target branch per plan** (plan `supervised-work`). A plan's `target` is the branch its work lands on, set with `mull plan set <slug> target <branch>` (or `mull set <slug> target <branch>`; `""` clears it). Without one, a plan's work, and unplanned work, lands on config's integration branch. There are no phase-level targets.
- Each item is measured against its own plan's target: its work is seen in git once linked commits are on that branch, and a merge of an id-branch counts only on that branch. Work that reached another plan's target, or integration, isn't seen for it.
- Unmerged work is linked commits on no target branch, so a branch merged into its target and left behind doesn't count. Target branches themselves (and the release branch) are never read as unmerged branches.
- The target is any name git accepts for a branch, other than `mull`. It needn't exist yet, and the view says when it doesn't.
- Unlinked commits are still counted on the integration branch only.
- Each target's history is read once (up to 5000 commits, cached by its tip), so a plan that shares integration costs nothing extra.

## Done: deterministic, with mull as the only path

The agent runs `mull done <id> --summary "…" --tour "…" --design -` when it says it's finished. mull then works through these steps:

1. **Check that the tree is clean.** A dirty working tree is refused, because the results must belong to a commit. mull records the commit SHA and the tree hash.
2. **Run the hard checks.** These are the commands in config, then the item's plan's own (see "A plan's own checks" below). Typically there's one project target, such as `make done-checks` in gallformers, which can call `mix precommit`. mull runs them and records each command, its exit code and its output tail.
3. **Don't re-run what already passed.** If a passing result is cached for the same tree hash and the same commands (`checks/<tree-hash>.json`, written by an earlier `mull check` or `done`), it's reused. Same tree, same result.
4. **If a check fails:** the item stays `active`. The failed attempt is saved and shown as a problem, and the command exits non-zero, so the agent knows it isn't done.
5. **If all checks pass,** mull:
   - marks the item `in-review`;
   - writes the done report: the agent's summary, UI tour and design notes (required: how it was built and why), the decisions the review carries, the check results, and system context from the diff (files added, changed and deleted, grouped by the layers in config, with test files and code files counted separately);
   - files a **review** entry in your inbox.

You then read the report and explore the tour. Settling the review is how an item becomes `done`:
- **Accept** marks it done. `--follow-up "text"` adds a new item for small leftovers.
- **Do** sends it back: the item returns to committed, with the reason in its notes.

**Follow-ups.** Done means reviewed, so a done item is never reopened, and its report stays as it was. Work after acceptance (a change you ask for once you've seen it) is a new item: `mull add "title" --follow-up <done-id>` creates committed work in the done item's plan and phase (a `--plan` or `--phase` given must agree), ranked last in its group, with `from` set to the done item. `accept --follow-up "text"` makes the same item at the moment of accepting.
- **The problem it prevents.** Commits carrying a done item's `Mull:` trailer and written after it was accepted (by author date, so a rebase or cherry-pick of earlier work doesn't count), on the integration branch or any other, show in the view as `work-after-accept`. A branch name or a merge commit alone doesn't count, so merging the item's branch after acceptance is fine.
- **When it clears.** It shows while the latest such commit is under a week old, and clears as soon as a follow-up of the item is added after that commit. "Accepted" is when its review was last accepted; an item done without one (from before Model v2) counts from when it was last updated.

There's no override for agents. For you, `mull done --override "reason"` exists, and it stays visible in the report and the view.

Your instinct was right, and not naive. The project owns *what* the hard checks are (a make target), and mull owns *running them and recording the result*. The agent can't skip the checks or misreport them, because the only way into review, and so to done, is through `mull done`. What checks can't prove (that it works and that you understand it) is covered by your review.

**As built:**
- **The human's name.** `mull name <name>` sets, per machine (in `~/.config/mull/name`, beside the registry, never on the mull branch), the name the person using mull there goes by: a repository is shared, and everyone who works on it has their own. mull's own words then use it for "the human": the protocol in `mull context`, which also opens by telling agents to write the name in what they write to mull, and mull's messages and errors. Guides stay as written, since they're copied into shared files (AGENTS.md). Stored records are never rewritten, and with no name the words are as before.
- **Setting config.** Config is set with `mull config set integration|release|remote <v>`, `mull config checks "<cmd>" ... --on-request` and `mull config layer <name> <glob...>`. A layer named `ui` is what triggers the tour requirement.
- **A plan's own checks.** A plan may add hard checks of its own for the work in it, such as an end-to-end suite only that plan's pages need: `mull plan checks <slug> "<cmd>" ... --on-request` replaces them, and no commands removes them. They're the human's, like config's, and kept in the plan's record (`checks`, shown by `plan show`). `done` runs config's checks, then the item's plan's, leaving out any command config already runs, as one list. `mull check --item <id>` runs exactly what `done` would for that item, and `mull check` alone runs config's. An item with no plan runs config's only, and when neither has any, `done` refuses as below.
- **Where changes are measured from.** Changes are measured from the commit that was checked out when the item was claimed (`claim.base`, recorded by `mull start`). That captures exactly the session's work. If there's no recorded base, they're measured from the merge base with the item's plan's target branch (the integration branch for unplanned work).
- **How checks run.** Checks run with `sh -c` in the worktree, one after another, and stop at the first failure. Only the last 40 lines of a failing command's output are kept. Results are cached on the `mull` branch at `checks/<tree>.json`, keyed by tree and command list, so a pass of config's checks alone doesn't stand for a plan's longer list.
- **No checks configured.** When neither config nor the item's plan has any, `done` refuses unless the human gives `--override`.
- **Reading a report.** `mull report <id> --text` prints the report as markdown.
- **Trust.** Check commands come from mull's config and plans, which travel with `sync`, and `done` runs them. Treat them like a Makefile in the repo: as trusted as whoever can push to the remote.
- **Only the human changes the checks.** They judge agents' work, so an agent may not change them: `mull config checks` and `mull plan checks` (and their batch ops) are refused (`not_allowed`) unless the agent passes `--on-request`, meaning you told it to. mull serve's writes are yours and would need no flag, though its pages don't offer these ops. This reverses the first cut (decision d359), where `plan checks` took no gate, as `plan doc` and `plan rules` don't: a doc or rules can steer an agent, but the checks are what let its work through.

Config example:

```yaml
integration: taxogall
release: main
remote: origin
done_checks:
  - make done-checks
layers:
  ui:     ["lib/gallformers_web/**", "assets/**"]
  domain: ["lib/gallformers/**"]
  data:   ["priv/repo/migrations/**"]
  tests:  ["test/**"]
```

## The view

`mull view [--plan p] [--text]`, with one block per open plan (active, then shaping), then unplanned work, the backlog's untriaged count and unlinked commits. Each plan's block opens with its target branch, marked `(integration)` when the plan falls back to it, and how far that branch is ahead of release. With `--text`:

```
integration taxogall · 166 ahead of main · 2026-11-09

TAXOGALL  Taxogall
  target taxogall (integration) · 166 ahead of main
  ▰▰▰▰▱▱▱▱▱▱ 7/17 done · 1 in review · 2 active · 3 ready · 12 captured (4 untriaged)
  PHASES
    3f1a  Admin                        ▰▰▰▱       3/4 done · 1 ready
    8b2e  Phase 2                      ▰▰▰▰▱▱▱▱▱  4/9 done · 1 in review · 2 active · 2 ready
    d470  Notes                        ▱▱▱        0/3 done · blocked by 8b2e
    e915  Cutover                      ▱          0/1 done · blocked by 8b2e, d470
  PROBLEMS (2)
    d3b0  claimed 3d ago by agent-1, no linked commits for a day
    d3b0  done attempt 1 failed its checks (reports/d3b0-1.md)
  IN FLIGHT
    cb28  Add a taxon from iNat                agent-2 · cb28-inat · 6 commits · last 3h
    d3b0  Unified edit model                   agent-1 · d3b0-edit-model · 2 commits · last 3d
  NEEDS YOU  1 review · 21 decisions · 8 questions (4 older than a week)

BACKLOG  4 captured items untriaged (triage with the human: mull guide triage) · last triage 2026-11-02

UNLINKED  31 commits on taxogall this week carry no item link
```

`mull inbox [--kind] [--plan]` lists the queue, oldest first, with each entry's source.

## `mull serve`: the human's view

The CLI is for agents, and `view --text` is a snapshot in a terminal. `mull serve` is the human's own view: a local web UI of every repository on mull, live as agents write. Opening a project answers "where are we?" and "what's next, and what do I need to do or decide?", and every review can be read there in full.

- **A server inside mull.** It's a Go server in the mull binary, using mull's own packages. It listens on `127.0.0.1` only, on a fixed port (6855, or `--port`), and serves plain JS modules, CSS and fonts embedded in the binary, so there's no Node build step. Plain `http://localhost` is enough, since nothing leaves the machine.
- **It writes only as you.** It reads each repository's `mull` branch as the CLI does. A write from a page goes through one endpoint that runs the same ops `mull batch` does, one commit per write, and only the ops the UI offers. What it settles is recorded as yours, by your name (the one this machine goes by, `mull name`, or else the repository's git `user.name`), not a worktree's and not as an agent's on request, so the steps only you take need no `--on-request`. A write carries the revision the page showed, and if a record it touches changed since, it's refused and the page redraws. Besides that it writes only when you last looked at each repository.
- **The registry.** Repositories are found with no configuration: every mull command run in a repository with a `mull` branch adds it to `~/.config/mull/repos` (under `$XDG_CONFIG_HOME` when that's set), one path per line. It's per machine and never on the `mull` branch, and a line can be removed by hand.
- **Last looked, and revisions.** What changed since you last looked is clouded in red, with no delta counts. A revision is a commit on the `mull` branch, numbered by how many commits lead to it. Your last look is the revision you marked seen, kept per machine in `~/.config/mull/seen.json` beside the registry, never on the `mull` branch: it's yours, not the program's. The changes are the records that differ between it and the tip. The first read of a repository takes the tip as the baseline, so only later changes are clouded. What you do from a page isn't news to you: the records a write changed are kept with your last look, as acknowledged, and stay unclouded until something changes them again. Marking seen clears that list.
- **Live updates.** mull serve watches each repository's `refs/heads/mull` and the registry, and tells open pages which project changed over Server-Sent Events; the page refetches that project. A write from any worktree, a sync, or a newly registered repository shows without a reload.
- **Read-first, writing step by step.** Most actions still copy the mull command (or a prompt for an agent) to run instead. Editing from the UI (the `mull-ui-edit` plan) moves your own steps onto the page one at a time; handing work to an agent stays a copy. Marking a revision seen writes only the last-looked state.
- **Nothing that needs you is lost below the fold.** Every row in the sheet index that holds a question, review or decision is marked (yellow when it blocks work), each count adds up to the rows below it, and a mark opens that sheet at its Needs you.
- **Needs you is ranked** by what an answer unblocks, in six tiers: questions that block work, reviews, questions on phases that can start now, questions on later phases, questions about a whole plan, then decisions to approve, grouped by source.
- **At login on macOS.** `mull serve --install` runs it as a LaunchAgent (`~/Library/LaunchAgents/mull.serve.plist`, logging to `~/Library/Logs/mull-serve.log`), restarted if it stops. It runs this mull binary with the PATH and XDG_CONFIG_HOME it was installed with. When the agent is installed, `make install` restarts it on the new binary with `launchctl kickstart` (falling back to `--install` if it isn't loaded), leaving the plist alone. It also signs the binary with the keychain's Apple Development identity and the identifier `mull`, so macOS sees each build as the same background item instead of announcing a new one. With no such identity it skips signing. An open page that reconnects to a different build (`GET /api/build`: the version and the embedded UI, hashed) reloads itself. `--uninstall` removes it.

Its design is in `serve/`: who it's for and its principles in `serve/PRODUCT.md`, the visual direction in `serve/direction.md`, and the design system as built in `serve/DESIGN.md`. Its JSON API (endpoints, the Needs you tiers, revisions and changes) is in `mull guide json`.

## The CLI

The CLI is used by agents. The human sees its data through an agent, or in `mull serve` (see above).
- **Output.** Every command prints JSON to stdout. `view`, `inbox`, `report`, `guide`, `migrate`, `plan doc`, `phase doc`, `plan rules` and `phase rules` also have `--text`, so an agent can show its human a rendering.
- **Errors.** An error prints `{"code", "error"}` to stderr and exits 1. The codes are stable.
- **The contract.** The shapes and codes are in `mull guide json`, and tests check every command's output against it.
- **Long text** is read from stdin with `-`.

**Every write is one op,** with one implementation used both by its command and by `mull batch`. Batch reads JSON Lines (`{"op": "...", "ref": "...", ...fields}`, with field names exactly the flag names) and runs every line in one transaction:
- **All or nothing.** If any line fails, nothing is written and every failed line is reported with its code. A batch that passes is a single commit.
- **References.** `"@ref"` refers to a record an earlier line created.
- **`--dry-run`** runs every line and writes nothing.

The ops are every write except `init`, `sync`, `migrate`, `check` and `done`.

**Scope is never silent:**
- `add` takes its plan from `--phase`, and refuses a `--plan` that disagrees. With neither, it refuses: unplanned work is asked for with `--plan none`. `--follow-up <done-id>` takes both from the done item, and refuses a `--plan` or `--phase` that disagrees.
- `decided` and `ask` require `--item` or `--plan` once the repo has more than one open plan (shaping or active). Captures (`bug`, `idea`) need neither.

| Group | Commands |
|---|---|
| Setup | `mull init [--integration b] [--release b]` · `mull config [set\|checks … --on-request\|layer …]` · `mull sync [--dry-run]` · `mull migrate [--dry-run] [--text] [--on-request]` |
| Plans (you) | `mull plan new <slug> "title" [--summary -] --on-request` · `plan agree <slug> --on-request` · `plan list` · `plan show <slug>` · `plan set <slug> title\|summary\|target <value>` · `plan doc <slug> [-] [--log]` · `plan rules <slug> [-] [--log]` · `plan checks <slug> ["<cmd>" …] --on-request` · `plan close <slug>` |
| Phases | `mull phase add "title" --plan <slug> [--needs …] [--summary -]` · `phase list [--plan]` · `phase doc <id> [-] [--log]` · `phase rules <id> [-] [--log]` · `phase delete <id> --on-request` · `show <id>` |
| Items | `mull add "title" (--plan <slug> \| --phase <id> \| --plan none \| --follow-up <done-id>) [--notes -] [--doc path]` · `show` · `list [--plan] [--phase] [--state] [--kind] [--untriaged]` · `set` · `drop` |
| Dependencies and order | `mull need <phase> <phase…>` · `mull unneed <phase> <phase…>` · `mull rank <phase\|plan\|none> <ids…>` · `mull ready [--plan]` |
| Work (agents) | `mull start <id>` · `release <id>` · `check [--item <id>]` (run the hard checks, cache the result; with `--item`, its plan's too) · `done <id> --summary "…" --tour "…" --design -` |
| Capture (agents) | `mull decided "text"`, `ask "question"`, each with `--item <id>` or `--plan <slug>`, plus `[--detail -] [--source …]` · `decided "text" --human --on-request` (your own decision, settled at once) |
| Backlog | `mull bug "title"` · `idea "title"`, each with `[--detail -] [--source …] [--plan \| --phase] [--category …]` · `commit <id> [--plan \| --phase \| --plan none] --on-request` · `triage <id…> --on-request` (or `--all [--plan <slug>\|none]`) |
| Batch | `mull batch [--dry-run] < ops.jsonl` |
| Settle (you, or an agent on your request) | `mull inbox [--kind] [--plan] [--parked]` · `accept <id…> [--follow-up "…"]` · `decide <id> "text"` · `do <id> [--plan] [--phase] [--as phase] [--reason "…"]` · `drop <id…> [--reason "…"]` · `park <id> "reason"` |
| Read (agents) | `mull view [--plan] [--text]` · `mull context [--item <id>]` (compact JSON for the start of a session: the protocol reminder, the shaping and active plans with their targets and phases, ready and active items, your claims, recent decisions, and for the item its phase's doc, its plan's and phase's rules, open questions and last report) · `decisions [--plan]` · `report <id> [--n] [--text]` |
| Guides | `mull guide agents` · `mull guide intake` · `mull guide supervisor` · `mull guide triage` · `mull guide json` (the output shapes and error codes) |
| The human's view | `mull serve [--port n]` · `mull serve --install` · `mull serve --uninstall` |

## The agent protocol (`mull guide agents`)

This is a short block pasted into the project's `AGENTS.md`. In outline:
1. At session start, run `mull context --item <id>`. Read the decision log before deciding anything it may already settle, and read the item's docs and its phase's doc. Follow its plan's rules, then its phase's.
2. **One item per session.** It's committed work from `mull ready`, or a captured item the human wants done now. Claim it with `mull start`, which commits a captured item. Name the branch `<id>-…`. End every commit message with `Mull: <id>`.
3. Capture as you go, one line each:
   - `mull decided` for any choice you made that the human didn't, and with `--human --on-request` for one the human made in conversation;
   - `mull ask` for anything you need from the human;
   - `mull bug` for anything out of scope that's wrong, and for feedback the human hands over for later;
   - `mull idea` for a possibility worth keeping that isn't work yet.

   Bugs and ideas go to the backlog, not to the inbox. When a capture's `similar` list holds what looks like the same thing, ask the human before keeping the near-duplicate.
4. Don't change plans or phases (or their docs), add items, or commit captured ones unless the human asks. Propose changes with `mull ask`.
5. Finish with `mull done`. If it fails, fix the problem or `mull release` the item and say why. After `done` the item is in review, and it's done only once its review is accepted. Never claim "done" in prose.
6. Show the human, or the supervisor, the done report and the UI tour. Run `mull accept <id> --on-request` only when the human tells you to. Then `mull sync`.
7. Never read or edit mull's data directly. Use the CLI.

`mull guide supervisor` covers coordinating several agents:
- pick from `mull ready`;
- give each worker a one-line hand-off, its item ID and nothing else: anything a worker needs goes in its plan's or phase's rules or the item's notes, never in a prompt, so it reaches every worker through `mull context --item`;
- propose missing or contradictory rules to the human with `mull ask`, since rules are theirs, and propose a phase for shared foundations (layout, components, helpers) that the phases using them need, rather than letting parallel workers each build their own;
- review each finished item's done report and diff, then `accept` (with `--follow-up` for small leftovers), or send it back with `do <review> --reason`;
- watch `mull view`, acting on problems first;
- route questions and decisions to the human instead of answering them;
- keep the backlog from workers until the human commits it, and go through it with the human when they ask (`mull guide triage`).

## Intake (`mull guide intake`)

There's no intake tooling. `mull guide intake` prints a procedure an agent follows in a project, with you, using the normal CLI and recording with `mull batch`. A repo already on mull from before Model v2 is migrated first (`mull migrate`). The agent reads:
- the old `.mull/` matters;
- any hub document (for gallformers, 50ff);
- branches, worktrees and recent commits.

The agent then:
1. **Proposes plans and phases** for you to approve, with a doc for each. Taxogall becomes a plan, and its release steps become phases with `needs` between them. Small, unrelated work is unplanned.
2. **Creates items** for work in flight or ready, each sized for one session.
3. **Files inbox entries** for decisions still awaiting your review and for open questions.
4. **Captures loose ends as bugs** in the backlog, including stale branches and worktrees.
5. **Cites sources.** Each record notes where it came from.
6. **Leaves the old backlog to you.** Matters with no place in a plan are gone through with you in conversation: most become ideas, some unplanned items or bugs, and some are left behind.

## Transition

History: new mull was developed beside old mull, then replaced it at the cutover on 2026-11-09.
- **The new binary is installed as `mull`** (`make install`), and the old one as `mull1`, for reading old `.mull/` data.
- **Each repo moves over through its own intake** (`mull guide intake`). A repo on new mull from before Model v2 moves to it with `mull migrate`.
- **Old `.mull/` directories stay in git as history.** Removing them is a later, explicit decision for each repo.
- **Claude-specific setup is dropped.** Old mull's `onboard hooks` is replaced by `mull context` plus the AGENTS.md block.

## Build order

History: mull was built in this order, each step usable before the next one started.

1. **Store and CLI:**
   - the `mull` branch, with plumbing writes and compare-and-swap;
   - `init`;
   - plans, phases (epics then) and items, with needs and readiness;
   - claims;
   - capture and settle;
   - decisions;
   - JSON output.

   Tests use a temporary repo for each test.
2. **`sync`,** with its safeguards and tests against a local bare remote.
3. **View:** `view` and `inbox`, with git linking and the derived marks.
4. **Done:** `check`, `done`, the report and the review entry.
5. **Guides and pilot:** `guide agents`, `guide intake` and `guide supervisor`. Intake piloted on gallformers with an agent, including `make done-checks` there.
6. **Later:** a GUI (native vs web decided then), a view across repos, and automatic sync. The first two became `mull serve`, a web UI across every repo on the machine, together with Model v2.

## Settled in review (2026-11-09)

- `mull sync` pushes only the `mull` branch, so agents may run it.
- Plan IDs are plain-English slugs. Phases and items use 4-hex IDs.
- A claim's identity is its worktree.
- Every done item gets reviewed, usually in session by you or a supervising agent, with the inbox as the safety net. Unreviewed work is incomplete work.
- Agents may settle inbox entries on your explicit instruction.
- `mull done` runs the project's hard checks deterministically, reusing cached passes for the same tree.

Model v2 and `mull serve` were settled with the human on 2026-11-10. Those decisions are in `serve/README.md`.
