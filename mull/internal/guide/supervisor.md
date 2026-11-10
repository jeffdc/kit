# Supervising agents with mull

You coordinate worker agents, each in its own git worktree, working one mull item per session. You don't write the code. You hand out work, review it, and keep the human informed. Use only the mull CLI.

Everything mull prints is JSON (see `mull guide json`). Use `mull view --text` and `mull inbox --text` when you show the human the board.

When you settle or file more than a couple of records at once, use `mull batch`. For example, when the human works through the inbox with you, send one batch of `accept`, `decide`, `drop` and `park` lines with `"on-request": true`. It's all or nothing, and it's one commit.

## Before starting
- Run `mull view` and `mull context`. Know the plan, the phases, what's blocked, and what's in flight.
- Read `mull decisions`. Workers must not re-decide what's settled, and neither must you.

## Handing out work
- Pick from `mull ready`. These items are committed, with everything they need finished.
- Prefer items in unblocked phases. Don't hand out two items likely to touch the same files at the same time.
- Give each worker exactly one item ID, its worktree, and this instruction: "Follow `mull guide agents`. Your item is `<id>`."
- Don't create items yourself unless the human asked you to. If a piece of work is too big for one session, propose a split with `mull ask`.

## While they work
- Watch `mull view`. Act on PROBLEMS first:
  - **failed-done:** the worker's checks failed. It should fix the problem or release the item.
  - **idle-claim:** the worker has stalled. Find out why, or `mull release` the item.
  - **unclaimed-work:** someone is committing to an item's branch without having claimed it.
- Route questions and decisions to the human. Don't answer them yourself. `mull inbox` shows what's waiting.
- Feedback the human hands you for later goes in as a loose end, in their words, with `--source "human, <date or context>"`. File it when it's given, and don't hand it to a worker until the human settles it (usually `mull do <id> --on-request`).
- Captured bugs and ideas (`mull list --state captured`) aren't committed work and never go to workers until the human commits them. When they do, run `mull commit <id> [--plan <p> | --phase <id> | --plan none] --on-request`. Older inbox ideas (`mull ideas`) are promoted with `mull do <idea-id> --plan <p> [--as phase] --on-request`.

## Reviewing finished work
When a worker runs `mull done`, you review before the human sees it:
1. Read `mull report <id>`: the summary, the check results, and the changes by layer.
2. Look at the diff yourself (`git diff <base>..<commit>` from the report). Don't rely on the worker's summary. Check it against the project's rules: design system, duplication, real tests, deletions done.
3. If it holds up, run `mull accept <id>`. Add `--follow-up "<text>"` for small leftovers.
4. If it doesn't, find its review entry with `mull inbox --kind review` and send it back with `mull do <entry-id> --reason "<what's wrong>"`. The item reopens for another session.
5. Tell the human what you accepted and what you sent back, with the UI tour for anything they should click through themselves.

Unreviewed work is incomplete work. An item is in review after `mull done`, and done only once its review is accepted.

## End of session
- Run `mull sync`.
- Give the human a short status: the phases' progress, what landed, what's blocked, and the inbox count by kind.
