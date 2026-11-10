# mull (next): design

Status: draft for review, 2026-11-09 (revision 2). Built on branch `mull-next` from scratch; none of the old code is reused.

Background:
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

1. **Any LLM, any harness.** The only interfaces are a CLI and git. Agents learn the protocol from a plain-text block in `AGENTS.md`. Nothing depends on Claude Code, Codex or any other harness. The test for every feature: it works with you typing the commands.
2. **Agents never edit mull's data files.** They read and write through the CLI. Long design documents are not mull data: they stay as ordinary files in the repo, and records link to them.
3. **What agents write is a claim; git and checks are evidence.** An item is done only when mull itself has run the project's hard checks and they passed. Whether its work has landed is shown separately, as *seen in git*.
4. **You stay involved, not sidelined.** Plans are yours. Decisions agents make come to you for ratification. Finished items come to you for review. Everything that needs you is in one queue, and you settle each entry with one action.
5. **It holds up when the model has a bad week.** An item is sized for one agent in one session. Checks are deterministic and run by mull. Problems surface fast.
6. **Limits are soft and loud,** with one exception: an item isn't done until its checks pass. Everything else is shown, never blocked.

## Data model: three levels

| Record | What it is | Fields | Who writes it |
|---|---|---|---|
| **Plan** | A large effort, such as `taxogall`. High-level only. | id (plain-English slug, e.g. `taxogall`, `general`), title, summary, state `shaping`, `active` or `done` | You |
| **Epic** | A big piece of a plan; a set of items. | id (4-hex), plan (required), title, summary, needs | You, or agents on your instruction |
| **Item** | A unit of work **one agent completes in one session.** | id (4-hex), plan (required), epic (optional), title, notes, docs (paths), state, claim (no needs: see "Ordering") | Agents and you |
| **Inbox entry** | Something that needs you | id, kind (`decision`, `question`, `loose-end` or `review`), one-line text, detail, item/epic/plan, source, created; when settled: action, note, by, time | Agents capture; you settle |
| **Idea** | Not work: something to keep until its time comes | an inbox entry of kind `idea`: title, short note (`detail`), source, optional plan | Agents file only ideas the human raises; only the human promotes |
| **Done report** | The record of a finished item | item, commit SHA, tree hash, UI tour, summary, check results, system context | Agent plus mull |
| **Config** | Per repo | integration branch, release branch, hard checks, layers, remote | You |

**Structure rules:**
- Every epic and every item belongs to a plan. An item may sit directly under its plan with no epic. The view shows those as "unfiled" in that plan.
- `mull init` creates one standing plan, `general`, for work outside any big effort, so the rule never forces a plan to be invented.
- No docket, no sessions, no free-floating epics.

**Plan states** (Model v2): `shaping → active → done`.
- `mull plan new` creates a plan `shaping`: its doc and questions are being worked out with you. It may already hold phases, items, captures and inbox entries, but none of its items is ready, and starting one is warned about.
- `mull plan agree <slug>` makes it `active`. Agreeing is yours: an agent must pass `--on-request`, and mull refuses otherwise.
- `mull plan close <slug>` makes it `done`, from either state. A done plan takes no new work and leaves the view (`view --plan` still shows it).
- Until the Model v2 migration, a done plan is stored as `closed`.

**Item stages** (Model v2, see `serve/README.md`): `captured → committed → active → in-review → done`, plus `dropped` from any stage before done.

| Stage | What triggers it |
|---|---|
| `captured` | The item is in the backlog, not committed to |
| `committed` | The item is committed work (`mull add` creates items here) |
| `active` | The item is claimed by an agent (see "Claims"). `start` works from captured or committed |
| `in-review` | Only through `mull done`, after the hard checks pass |
| `done` | The work is reviewed and accepted (see "Review") |

**Unreviewed work is incomplete work.** An item counts as finished only once it's `done`, which means reviewed.
- A phase's progress counts done items out of its committed items, with items in review shown separately. Captured items tagged to a phase are counted on their own and never hold it open.
- A phase is complete when it has committed items and all of them are done or dropped.
- Until the Model v2 migration, stages are stored under their old names (`open`, `done`, `accepted` for committed, in-review and done).

### Ordering: loose, with dependencies between phases

There is no priority list and no ordering field. Order only where it matters (Model v2):
- A phase may **need** other phases. That is the only ordering: items have no needs, and items inside a phase are unordered. A phase's need is met when the needed phase is complete.
- "Can start now" is derived: the phases whose needs are done.
- **Ready** means committed, in a phase whose needs are done (or in no phase).
- **Blocked** means committed, in a phase with at least one need not done.
- Cycles are refused when written.
- Needs from before Model v2 are kept until `mull migrate` converts them: an item's needs are shown but ignored, and a phase's needs on items still count.
- `mull ready [--plan p]` lists ready items. A supervisor agent uses it to hand out work.
- The view sorts epics and items by readiness, then by age.

### Claims (several agents at once)

- `mull start <id>` claims an item and sets it `active`.
- The claim is atomic: the write is a compare-and-swap (see "Storage"). If two agents start the same item, one wins and the other gets an error.
- A claim records the worktree, the branch and the time. The worktree is the agent's identity.
- `mull release <id>` gives a claim back.
- A claim with no linked commits for a day shows up as a problem in the view.

### Review

Most items are reviewed in the session that finished them, by you or by a supervising agent. The inbox is the safety net for any that slip through.

- **Review entry.** `mull done` always files a `review` entry. Accepting it marks the item `accepted`, and `mull accept <item-id>` works directly on the item too.
- **In session:** you or the supervisor accept right after reviewing the done report and the tour, so the entry never lingers.
- **Who may accept:**
  - a supervising agent, working from another worktree;
  - you;
  - the worker itself, only with `--on-request`, meaning you told it to in conversation.

  An accept run from the item's own claim worktree without `--on-request` is refused. That's a cheap, deterministic guard against an agent approving its own work.
- **Anything not reviewed stays in NEEDS YOU** as a review entry until someone settles it.

### Ideas

An idea is a title, a short note, a source and an optional plan. It isn't work, so it:
- needs no sizing and is never ready;
- stays out of `inbox` and NEEDS YOU;
- shows on the board only as a count (per plan, plus ideas with no plan), with `mull ideas` listing them all.

**Settling an idea:**
- **promote** with `mull do <idea> --plan <p>` (`--as epic` for an epic; the default is an item). The new record's `from` links back to the idea, and the idea's note and source go into its notes;
- **drop**;
- **park.**

Only the human promotes ideas. An agent must pass `--on-request`, meaning the human asked, and mull refuses otherwise. Agents file ideas only when the human raises them; their own findings are loose ends. In intake, most of the old backlog fits here.

### The inbox

| Action | Applies to | Effect |
|---|---|---|
| **Accept** | decisions, reviews | Decision: ratified as recorded. Review: the item becomes `accepted`, optionally with `--follow-up "text"` creating a new item in the same plan and epic |
| **Decide** `"text"` | decisions, questions, loose ends | You write the decision (overruling an agent's decision if need be), and it joins the decision log |
| **Do** | questions, loose ends, reviews | Question or loose end: becomes a new item. Review: **sends the work back**: the item reopens for another session, with `--reason` added to its notes |
| **Drop** | decisions, questions, loose ends | Gone for good, with the drop recorded so it doesn't resurface |
| **Park** `"reason"` | any | Out of the queue, still listed with `--parked`, and can be settled later |

A review can't be decided or dropped, because both would leave unreviewed work looking finished. To abandon the work, drop the item itself.

- Accepted and decided entries make up the **decision log**. Agents read it before starting work, so a settled question stays settled.
- An agent may settle entries **on your explicit instruction in conversation**. The record notes `by: agent, on request`.

## Storage: the `mull` branch

All data lives on a dedicated git branch, `mull`, which is never checked out. Its history is separate from the code: it starts with its own first commit and contains only mull data.

```
config.yml
plans/<slug>.md
epics/<id>.md
items/<id>.md
inbox/<id>.md
reports/<item>-<n>.md
checks/<tree-hash>.json        # cached hard-check results
```

There's one file per record, so concurrent writes and merges rarely conflict.

**Writes:**
- Git plumbing: a temporary index, `write-tree`, `commit-tree`, then `update-ref refs/heads/mull <new> <old>`.
- That last step is a compare-and-swap. On a race it re-reads, re-applies and retries. Claims rely on this.
- Writes work from any worktree on any branch and never touch the working tree.

**Reading:** `ls-tree` plus `cat-file --batch`. Every worktree sees the same branch, because branches live in the repo's shared git directory.

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
| **Seen in git** | The item's linked commits are reachable from the integration branch, so its work has landed. Shown next to `done`, never a gate |
| **Problems** | An active claim with no linked commits for a day; done with nothing seen in git after N days; a failed done attempt; commits on the integration branch with no link, shown as a count |

Bare 4-hex mentions in commit prose are not counted, because they collide with hashes.

## Done: deterministic, with mull as the only path

The agent runs `mull done <id> --tour - --summary "…"` when it says it's finished. mull then works through these steps:

1. **Check that the tree is clean.** A dirty working tree is refused, because the results must belong to a commit. mull records the commit SHA and the tree hash.
2. **Run the hard checks.** These are the commands in config. Typically there's one project target, such as `make done-checks` in gallformers, which can call `mix precommit`. mull runs them and records each command, its exit code and its output tail.
3. **Don't re-run what already passed.** If a passing result is cached for the same tree hash (`checks/<tree-hash>.json`, written by an earlier `mull check`), it's reused. Same tree, same result.
4. **If a check fails:** the item stays `active`. The failed attempt is saved and shown as a problem, and the command exits non-zero, so the agent knows it isn't done.
5. **If all checks pass,** mull:
   - marks the item `done`;
   - writes the done report: the agent's UI tour and summary, the check results, and system context from the diff (files added, changed and deleted, grouped by the layers in config, with test files and code files counted separately);
   - files a **review** entry in your inbox.

You then read the report and explore the tour. Settling the review is how an item becomes `accepted`:
- **Accept** marks it accepted.
- **Do** opens a follow-up item.

There's no override for agents. For you, `mull done --override "reason"` exists, and it stays visible in the report and the view.

Your instinct was right, and not naive. The project owns *what* the hard checks are (a make target), and mull owns *running them and recording the result*. The agent can't skip the checks or misreport them, because the only way to `done` is through mull. What checks can't prove (that it works and that you understand it) is covered by your review.

**As built:**
- **Setting config.** Config is set with `mull config set integration|release|remote <v>`, `mull config checks "<cmd>" ...` and `mull config layer <name> <glob...>`. A layer named `ui` is what triggers the tour requirement.
- **Where changes are measured from.** Changes are measured from the commit that was checked out when the item was claimed (`claim.base`, recorded by `mull start`). That captures exactly the session's work. If there's no recorded base, they're measured from the merge base with the integration branch.
- **How checks run.** Checks run with `sh -c` in the worktree, one after another, and stop at the first failure. Only the last 40 lines of a failing command's output are kept. Results are cached on the `mull` branch at `checks/<tree>.json`, keyed by tree and command list.
- **No checks configured.** `done` refuses unless the human gives `--override`.
- **Reading a report.** `mull report <id>` prints the report as markdown.
- **Trust.** Check commands come from mull's config, which travels with `sync`, and `done` runs them. Treat them like a Makefile in the repo: as trusted as whoever can push to the remote.

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

`mull view [--plan p] [--json]`, with one block per active plan:

```
TAXOGALL                                       taxogall 166 ahead of main · 2026-11-09
EPICS      Admin ▰▰▰▱ 3/4   Phase 2 ▰▰▱▱ 4/9 +1 unreviewed (2 ready)   Notes ▱▱▱ blocked by Phase 2   Cutover ▱ blocked
PROBLEMS   1 failed done (d3b0) · 1 claim idle 2 days · 31 unlinked commits this week
IN FLIGHT  cb28 Add a taxon from iNat     agent-2 · cb28-inat · 6 commits · 1d
           d3b0 Unified edit model        agent-1 · no commits since 11-07
NEEDS YOU  3 reviews · 21 decisions · 8 questions · 22 loose ends (4 older than a week)
```

`mull inbox [--kind]` lists the queue, oldest first, with each entry's source.

## The CLI

mull is used only by agents; humans see its data through an agent.
- **Output.** Every command prints JSON to stdout. `view`, `inbox`, `report` and `guide` also have `--text`, so an agent can show its human a rendering.
- **Errors.** An error prints `{"code", "error"}` to stderr and exits 1. The codes are stable.
- **The contract.** The shapes and codes are in `mull guide json`, and tests check every command's output against it.
- **Long text** is read from stdin with `-`.

**Every write is one op,** with one implementation used both by its command and by `mull batch`. Batch reads JSON Lines (`{"op": "...", "ref": "...", ...fields}`, with field names exactly the flag names) and runs every line in one transaction:
- **All or nothing.** If any line fails, nothing is written and every failed line is reported with its code. A batch that passes is a single commit.
- **References.** `"@ref"` refers to a record an earlier line created.
- **`--dry-run`** runs every line and writes nothing.

The ops are every write except `init`, `sync`, `check` and `done`.

**Scope is never silent:**
- `add` takes its plan from `--epic`, and refuses a `--plan` that disagrees.
- `decided`, `ask` and `loose` require `--item` or `--plan` once the repo has more than one active plan.

| Group | Commands |
|---|---|
| Setup | `mull init` · `mull sync [--dry-run]` |
| Plans (you) | `mull plan new <slug> "title" [--summary -]` · `plan agree <slug>` · `plan list` · `plan show <slug>` · `plan close <slug>` |
| Epics | `mull epic add "title" --plan <slug> [--needs …]` · `epic show <id>` · `epic list [--plan]` |
| Items | `mull add "title" (--plan <slug> \| --epic <id>) [--notes -] [--doc path]` · `show` · `list` · `set` · `drop` |
| Dependencies | `mull need <phase> <phase…>` · `mull unneed <phase> <phase…>` · `mull ready [--plan]` |
| Work (agents) | `mull start <id>` · `release <id>` · `check` (run the hard checks, cache the result) · `done <id> --tour - --summary "…"` |
| Capture (agents) | `mull decided "text"`, `ask "question"`, `loose "text"`, each with `--item <id>` or `--plan <slug>`, plus `[--detail -] [--source …]` |
| Batch | `mull batch [--dry-run] < ops.jsonl` |
| Settle (you, or an agent on your request) | `mull inbox` · `accept <id…>` · `decide <id> "text"` · `do <id> [--plan] [--epic]` · `drop <id…>` · `park <id> "reason"` |
| Read (agents) | `mull context [--item <id>]` (compact JSON for the start of a session: the plan, its epics, ready items, the item's needs, recent decisions, open questions on the item, the protocol reminder) · `decisions [--plan]` |
| Guides | `mull guide agents` · `mull guide intake` · `mull guide supervisor` |

## The agent protocol (`mull guide agents`)

This is a short block pasted into the project's `AGENTS.md`. In outline:
1. At session start, run `mull context --item <id>`. Read the decision log before deciding anything it may already settle.
2. **One item per session.** Claim it with `mull start`. Name the branch `<id>-…`. End every commit message with `Mull: <id>`.
3. Capture as you go, one line each:
   - `mull decided` for any choice you made that the human didn't;
   - `mull ask` for anything you need from the human;
   - `mull loose` for anything out of scope you noticed.
4. Don't edit plans or epics unless the human asks. Propose changes with `mull ask`.
5. Finish with `mull done`. If it fails, fix the problem or `mull release` the item and say why. Never claim "done" in prose.
6. Show the human, or the supervisor, the done report and the UI tour. Run `mull accept <id> --on-request` only when the human tells you to. Then `mull sync`.
7. Never read or edit mull's data directly. Use the CLI.

`mull guide supervisor` covers coordinating several agents:
- pick from `mull ready`;
- give each worker one item ID;
- review each finished item's done report and diff, then `accept`, or `do` a follow-up;
- watch `mull view`;
- route questions and decisions to the human instead of answering them.

## Intake (`mull guide intake`)

There's no intake tooling. `mull guide intake` prints a procedure an agent follows in a project, with you, using the normal CLI. It reads:
- the old `.mull/` matters;
- any hub document (for gallformers, 50ff);
- branches, worktrees and recent commits.

The agent then:
1. **Proposes plans and epics** for you to approve. Taxogall becomes a plan, and its release steps become epics with `needs` between them.
2. **Creates items** for work in flight or ready, each sized for one session.
3. **Files inbox entries:**
   - decisions still awaiting your review;
   - open questions;
   - loose ends, including stale branches and worktrees.
4. **Cites sources.** Each record notes where it came from.
5. **Leaves the old backlog to you.** Matters with no place in a plan are triaged with you in conversation.

## Transition

- **During development:** the new binary runs from the worktree as `./mull`. The installed `mull` stays the old one.
- **At cutover:** the old binary is installed as `mull1`, and the new one as `mull`. Each repo moves over through its own intake.
- **Old `.mull/` directories stay in git as history.** Removing them is a later, explicit decision for each repo.
- **Claude-specific setup is dropped.** Old mull's `onboard hooks` is replaced by `mull context` plus the AGENTS.md block.

## Build order

Each step is usable before the next one starts.

1. **Store and CLI:**
   - the `mull` branch, with plumbing writes and compare-and-swap;
   - `init`;
   - plans, epics and items, with needs and readiness;
   - claims;
   - capture and settle;
   - decisions;
   - JSON output.

   Tests use a temporary repo for each test.
2. **`sync`,** with its safeguards and tests against a local bare remote.
3. **View:** `view` and `inbox`, with git linking and the derived marks.
4. **Done:** `check`, `done`, the report and the review entry.
5. **Guides and pilot:** `guide agents`, `guide intake` and `guide supervisor`. Intake piloted on gallformers with an agent, including `make done-checks` there.
6. **Later:** a GUI (native vs web decided then), a view across repos, and automatic sync.

## Settled in review (2026-11-09)

- `mull sync` pushes only the `mull` branch, so agents may run it.
- Plan IDs are plain-English slugs. Epics and items use 4-hex IDs.
- A claim's identity is its worktree.
- Every done item gets reviewed, usually in session by you or a supervising agent, with the inbox as the safety net. Unreviewed work is incomplete work.
- Agents may settle inbox entries on your explicit instruction.
- `mull done` runs the project's hard checks deterministically, reusing cached passes for the same tree.
