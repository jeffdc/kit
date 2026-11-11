# mull

The record of a program run by you and your agents: its plans, where they stand, and everything that needs you.

`make install` installs it as `mull`. It replaced the old matter tracker, whose binary is kept as `mull1` for reading old `.mull/` data until each repository is converted (`mull guide intake`). The design is in [docs/design.md](docs/design.md), the research behind it in [docs/research/](docs/research/), and the human's view in [docs/serve/](docs/serve/).

## What it does

- **Plans → phases → items.** An item is a unit of work one agent completes in one session. A phase can *need* other phases (the only ordering; items inside a phase are unordered), and mull works out what's ready and what's blocked.
- **Plans are yours.** Only you create a plan, and it stays *shaping* (captures and questions, no phases or committed work) until you agree it; then it's active, and done when closed. Plans and phases have docs (`mull plan doc`, `mull phase doc`, with history in `--log`): the picture, when it's done, and why the order. Docs hold intent only; status comes from the records.
- **Stages.** An item goes captured → committed → active → in review → done, or is dropped. `mull start` works from captured or committed, so a small bug can go from the backlog to done in one session. Done means reviewed.
- **Claims.** `mull start <id>` claims an item for the current worktree. It's atomic, so two agents can't take the same item.
- **A backlog of captured items.** `mull bug` and `mull idea` capture an item that isn't committed work: never ready, kept out of a phase's progress, and never in what needs you. Anyone captures; only the human commits one (`mull commit`), optionally into a plan or phase; work with no plan is unplanned. Triage runs with an agent: `mull triage` stamps the items you went through, and the view counts what's untriaged. Older loose ends and inbox ideas stay listed by kind (`mull inbox --kind loose-end|idea`) until `mull migrate` converts them.
- **One inbox for what needs you:** decisions agents made, questions and reviews. You settle each with one action: accept, decide, do, drop or park. Settled decisions form a log agents read before deciding anything.
- **Stored in git, out of the way.** Everything lives on a `mull` branch that is never checked out. Writes never touch your working tree, and every worktree sees the same data.
- **Any LLM, any harness.** Agents use the CLI. Nothing depends on a particular tool.

## The human's view: `mull serve`

```bash
mull serve              # http://localhost:6855/, until interrupted
mull serve --install    # macOS: run it at login as a LaunchAgent (make install restarts it)
mull serve --uninstall
```

A local web page of every repository on mull, live as agents write. Repositories are found with no configuration: any mull command run in a repository with a mull branch registers it (`~/.config/mull/repos`). For each project it answers where things stand and what needs you:
- **Needs you,** ranked by what an answer unblocks: blocking work, reviews, questions by phase, then decisions to approve. Everything that needs you is marked in the sheet index, blocking or not.
- **Plans:** what's built, what can start now, what's next, the go-live phase, and the agents at work; each plan's doc, rendered, with the status of what it names beside it.
- **Phases, items and reviews:** what was built and how, with the session's decisions.
- **The backlog and unplanned work,** and the repositories beside yours that aren't on mull yet.
- **What changed since you last looked** is clouded in red; Mark revision seen clears it. The last look is kept on this machine, never on the mull branch.

It listens on 127.0.0.1 only, and writes mull data only as you, through the ops its pages offer. After `make install`, the LaunchAgent restarts on the new build, and open pages reload themselves when they reconnect to it. Most actions copy the mull command to run; approving decisions saves from the page. `mull guide json` documents its API; [docs/serve/](docs/serve/) its design, and [docs/serve/DESIGN.md](docs/serve/DESIGN.md) its design system.

## Output

Every command prints JSON; errors are `{"code", "error"}` on stderr with exit status 1. `mull guide json` documents every shape and error code. `view`, `inbox`, `report` and `guide` take `--text` for a human rendering.

To write many records at once, use `mull batch`. It reads JSON Lines, is all or nothing, and lands as one commit; `"@ref"` refers to a record an earlier line created. See `mull guide json`.

## Quick start

```bash
make build
./mull init --integration main
./mull plan new launch "Public launch" --on-request   # the human's call; shaping until they agree it
./mull plan agree launch --on-request   # the human's call: its work can now start
./mull phase add "Billing" --plan launch
./mull add "Stripe webhook handler" --phase <phase-id>
./mull ready
./mull start <item-id>
./mull ask "Monthly or annual plans first?" --item <item-id>
./mull inbox --text
./mull decide <entry-id> "Monthly first"
./mull decisions
```

`./mull sync` exchanges the `mull` branch with your remote. It pushes only that branch, never forces, and refuses to push anything that isn't a mull record. `--dry-run` shows the exact git commands first.

`./mull view --text` is the board: each plan's phases and progress, problems first, work in flight, and what needs you. It reads git as well as mull's records:
- a branch named `<id>-…` marks its item's work as in flight;
- a `Mull: <id>` commit trailer, or a merge of an `<id>-…` branch, on the integration branch marks the work as landed;
- direct commits with neither are counted as unlinked.

**Finishing work is deterministic.** The human configures the hard checks once:

```bash
./mull config checks "make done-checks"
./mull config layer ui "lib/web/**" "assets/**"
```

An agent then runs `./mull done <id> --summary "..." --tour "..." --design -` (design notes on stdin: how it was built and why). mull runs the checks against the clean HEAD, reusing a passing result for the same tree. On a pass it writes a done report (what: summary, UI tour, check results; how: design notes, the decisions made in the session, files changed by layer), puts the item in review, and files a review for the human. The review carries the item's open decisions, and accepting it ratifies them. On a failure the item stays active. `./mull report <id>` prints the report.

**Agents learn the protocol from mull itself:**
- `./mull guide agents` prints the block to paste into a project's AGENTS.md;
- `./mull guide supervisor` covers coordinating several agents;
- `./mull guide intake` is the procedure for bringing a project, and its old `.mull/` matters, into mull, with the human;
- `./mull guide triage` is the procedure for going through the backlog with the human: commit, tag to a plan or phase, start or drop.

`./mull context --item <id>` is the compact briefing an agent reads at the start of a session.
