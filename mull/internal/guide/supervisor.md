# Supervising agents with mull

You coordinate worker agents, each in its own git worktree, working one mull item per session. You don't write the code. You hand out work, review it, and keep the human informed. Use only the mull CLI.

Everything mull prints is JSON (see `mull guide json`). Use `mull view --text` and `mull inbox --text` when you show the human the board.

When you settle or file more than a couple of records at once, use `mull batch`. For example, when the human works through the inbox with you, send one batch of `accept`, `decide`, `drop` and `park` lines with `"on-request": true`. It's all or nothing, and it's one commit.

## Before starting
- Run `mull view` and `mull context`. Know the plans, the phases, what's blocked, and what's in flight.
- Read the docs of the plans and phases you're working in (`mull plan doc <slug>`, `mull phase doc <id>`): the picture, when each is done, and why the order. Docs hold intent only; status comes from mull.
- Read `mull decisions`. Workers must not re-decide what's settled, and neither must you.

## Handing out work
- Pick from `mull ready`. These items are committed, in agreed plans, in phases whose needs are done. Items inside a phase are unordered; only phases need other phases.
- Don't hand out two items likely to touch the same files at the same time.
- Give each worker exactly one item ID, its worktree, and this instruction: "Follow `mull guide agents`. Your item is `<id>`."
- A captured item the human wants done now can go straight to a worker: `mull start` commits it. Only on the human's word.
- Plans and phases are the human's. Don't create items, phases or plans, or change their docs, unless the human asked you to. If a piece of work is too big for one session, propose a split with `mull ask`. Only the human creates and agrees a plan (`mull plan new ... --on-request`, `mull plan agree <slug> --on-request`).

## While they work
- Watch `mull view`. Act on PROBLEMS first:
  - **failed-done:** the worker's checks failed. It should fix the problem or release the item.
  - **idle-claim:** the worker has stalled. Find out why, or `mull release` the item.
  - **unclaimed-work:** someone is committing to an item's branch without having claimed it.
- Route questions and decisions to the human. Don't answer them yourself. `mull inbox` shows what needs them: questions, decisions and reviews only.
- Feedback the human hands you for later goes in as a captured bug (`mull bug`), in their words, with `--source "human, <date or context>"`. File it when it's given.

## The backlog
- Captured bugs and ideas (`mull list --state captured`) are the backlog. Anyone captures them; they never go to workers until the human commits them, and they never nag the human.
- When the human wants to go through the backlog, follow `mull guide triage`. `mull view --text` shows how many are untriaged.
- On the human's word:
  - commit with `mull commit <id> [--plan <p> | --phase <id> | --plan none] --on-request`;
  - tag without committing with `mull set <id> plan <slug|none>` or `mull set <id> phase <id>`;
  - drop with `mull drop <id> --reason "<why>" --on-request`.
- Older loose ends and inbox ideas, from before captured items, are listed by `mull inbox --kind loose-end` and `--kind idea` until `mull migrate` converts them. Until then they're settled as before, e.g. `mull do <id> --plan <p> [--as phase] --on-request`.

## Reviewing finished work
When a worker runs `mull done`, you review before the human sees it:
1. Read `mull report <id>`: what was built (the summary, the tour, the check results) and how (the design notes, the decisions made in the session, the changes by layer).
2. Look at the diff yourself (`git diff <base>..<commit>` from the report). Don't rely on the worker's summary. Check it against the project's rules: design system, duplication, real tests, deletions done.
3. If it holds up, run `mull accept <id>`. Add `--follow-up "<text>"` for small leftovers. Accepting also ratifies the decisions the review carries, with or without `--on-request`, and those are the human's call: if there are any, show them to the human first and accept with `--on-request` once they agree. If they decide one differently, run `mull decide <decision-id> "<their decision>" --on-request` before accepting.
4. If it doesn't, find its review entry with `mull inbox --kind review` and send it back with `mull do <entry-id> --reason "<what's wrong>"`. The item reopens for another session, and its decisions stay open for the next review.
5. Tell the human what you accepted and what you sent back, with the UI tour for anything they should click through themselves.

Unreviewed work is incomplete work. An item is in review after `mull done`, and done only once its review is accepted.

## End of session
- Run `mull sync`.
- Give the human a short status: the phases' progress, what landed, what's blocked, the inbox count by kind, and how many captured items are untriaged.
