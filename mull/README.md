# mull

The record of a program run by you and your agents: its plans, where they stand, and everything that needs you.

> **This branch is a from-scratch rewrite (`mull-next`).** It replaces the old matter tracker at cutover. The design is in [docs/design.md](docs/design.md), and the research behind it is in [docs/research/](docs/research/).

## What it does

- **Plans → epics → items.** An item is a unit of work one agent completes in one session. Items and epics can *need* each other, and mull works out what's ready and what's blocked.
- **Claims.** `mull start <id>` claims an item for the current worktree. It's atomic, so two agents can't take the same item.
- **One inbox for what needs you:** decisions agents made, questions, loose ends and reviews. You settle each with one action: accept, decide, do, drop or park. Settled decisions form a log agents read before deciding anything.
- **Stored in git, out of the way.** Everything lives on a `mull` branch that is never checked out. Writes never touch your working tree, and every worktree sees the same data.
- **Any LLM, any harness.** Agents use the CLI. Nothing depends on a particular tool.

## Quick start (development build)

```bash
make build
./mull init --integration main
./mull plan new launch "Public launch"
./mull epic add "Billing" --plan launch
./mull add "Stripe webhook handler" --plan launch --epic <epic-id>
./mull ready
./mull start <item-id>
./mull ask "Monthly or annual plans first?" --item <item-id>
./mull inbox
./mull decide <entry-id> "Monthly first"
./mull decisions
```

`./mull sync` exchanges the `mull` branch with your remote. It pushes only that branch, never forces, and refuses to push anything that isn't a mull record. `--dry-run` shows the exact git commands first.

`./mull view` is the board: each plan's epics and progress, problems first, work in flight, and what needs you. It reads git as well as mull's records:
- a branch named `<id>-…` marks its item's work as in flight;
- a `Mull: <id>` commit trailer, or a merge of an `<id>-…` branch, on the integration branch marks the work as landed;
- direct commits with neither are counted as unlinked.

**Finishing work is deterministic.** The human configures the hard checks once:

```bash
./mull config checks "make done-checks"
./mull config layer ui "lib/web/**" "assets/**"
```

An agent then runs `./mull done <id> --summary "..." --tour -`. mull runs the checks against the clean HEAD, reusing a passing result for the same tree. On a pass it writes a done report (summary, UI tour, check results, files changed by layer), marks the item done, and files a review for the human. On a failure the item stays active. `./mull report <id>` prints the report.

**Agents learn the protocol from mull itself:**
- `./mull guide agents` prints the block to paste into a project's AGENTS.md;
- `./mull guide supervisor` covers coordinating several agents;
- `./mull guide intake` is the procedure for bringing a project, and its old `.mull/` matters, into mull, with the human.

`./mull context --item <id>` is the compact briefing an agent reads at the start of a session.
