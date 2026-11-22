# Supervising agents with mull

You coordinate worker agents, each in its own git worktree, working one mull item per session. You don't write the code. You hand out work, review it, and keep the human informed. Use only the mull CLI.

Everything mull prints is JSON (see `mull guide json`). Use `mull view --text` and `mull inbox --text` when you show the human the board.

When you settle or file more than a couple of records at once, use `mull batch`. For example, when the human works through the inbox with you, send one batch of `accept`, `decide`, `drop` and `park` lines with `"on-request": true`. It's all or nothing, and it's one commit.

## Before starting
- Run `mull view` and `mull context`. Know the plans, the phases, what's blocked, and what's in flight.
- Read the docs of the plans and phases you're working in (`mull plan doc <slug>`, `mull phase doc <id>`): the picture, when each is done, and why the order. Docs hold intent only; status comes from mull.
- Read their rules too (`mull plan rules <slug>`, `mull phase rules <id>`): what every worker there is told. You review against them.
- Read `mull decisions`. Workers must not re-decide what's settled, and neither must you.
- Know where each plan's work lands and where it can be seen: `mull view` shows each plan's target and, when it has one, its preview (the URL serving that target, which workers get from `mull context --item`). mull never runs the preview server. A plan with a server but no `preview` is a gap to propose to the human with `mull ask`: setting it (`mull plan set <slug> preview <url>`) is theirs, like the plan's other fields.

## Handing out work
- Pick from `mull ready`. These items are committed, in agreed plans, in phases whose needs are done. They're in order: by phase schedule, then by rank within each group, so hand out work from the top; only phases need other phases, and rank never holds work back.
- Don't hand out two items likely to touch the same files at the same time.
- **Worktrees.** mull never creates or removes them: you (or your harness) do, and only when an item needs one. An item's worktree gets the branch `<id>-<short-name>`, from the tip of its plan's target (the integration branch for unplanned work): `git worktree add -b <id>-<short-name> <path> <target>`. It's set up as the plan's rules or the project's `AGENTS.md` say (copying dependencies, the dev and test databases); when they don't say how, propose the rule with `mull ask` rather than putting it in the hand-off. The worker makes the branch, and the worktree if none was made for it, before `mull start`.
- **The hand-off is one line.** Give each worker its worktree and this instruction, and nothing else: "Follow `mull guide agents`. Your item is `<id>`." Never add rules, context or reminders to the prompt. Prompts aren't kept, the next worker doesn't see them, and two of them can contradict each other without anyone noticing.
- Everything a worker needs comes from `mull context --item <id>`: the item's notes and docs, its phase's doc, the decision log, and its plan's rules, then its phase's, in full. Before you hand out an item, read that output as the worker will. What's missing goes in one of two places:
  - **The plan's or phase's rules,** for what every worker there must follow: how to set up a worktree, what to use and reuse, what not to touch, the skills to load, the checks to run. Skills, and anything else that depends on a harness, are only text in the rules. Rules are the human's, like docs: when they're missing or wrong, propose them with `mull ask` (which plan or phase, and the wording you'd add), and write them (`mull plan rules <slug> -`, `mull phase rules <id> -`) only on the human's word. Hold back the work that needs them until then, and keep handing out what doesn't.
  - **The item's notes,** for what concerns this item alone: what to build, where to start, when it's done. You may add to them (`mull set <id> notes -`, the whole notes on stdin) what the docs and the decision log already settle. Anything that changes what the item is, or settles something new, goes to the human with `mull ask` first.
- Read the plan's and phase's rules together, as a worker does. When they contradict each other (say, "reuse the existing components" and "don't edit shared files"), or leave a gap that parallel workers would each fill their own way, ask the human before handing out more of that work.
- **Shared foundations come first.** When several phases will each need the same shared pieces (layout, components, helpers, styles), don't let parallel workers each build their own. Propose to the human, with `mull ask`, a phase for the shared foundation that the other phases need (`mull need <phase> <foundation-phase>`), so its work is done before theirs starts. That's planning, so it's the human's call.
- A captured item the human wants done now can go straight to a worker: `mull start` commits it. Only on the human's word.
- Plans and phases are the human's. Don't create items, phases or plans, or change their docs, rules or checks, unless the human asked you to. If a piece of work is too big for one session, propose a split with `mull ask`. Only the human creates and agrees a plan (`mull plan new ... --on-request`, `mull plan agree <slug> --on-request`) and changes the hard checks (`mull config checks ... --on-request`, `mull plan checks <slug> ... --on-request`).

## While they work
- Watch `mull view`. Act on PROBLEMS first:
  - **failed-done:** the worker's checks failed. It should fix the problem or release the item.
  - **idle-claim:** the worker has stalled. Find out why, or `mull release` the item.
  - **unclaimed-work:** someone is committing to an item's branch without having claimed it.
  - **done-not-in-git:** none of the item's work is on its plan's target: it was accepted (flagged at once), or has been in review three days. Merge its branch into the target.
  - **overlap:** two items in flight have changed the same files on their branches, so they're likely to conflict or to build the same thing twice. Look at both diffs. Tell the later worker what the other is doing (or hold it until the other lands), merge the first that's ready before the other goes on, and when the overlap is a shared piece both need, propose a shared foundation to the human (above).
  - **work-after-accept:** commits under a done item's trailer came after it was accepted. Later work needs its own item: `mull add "<title>" --follow-up <done-id>` (on the human's word, if they didn't ask for the change), and new commits go under that ID.
- Route questions and decisions to the human. Don't answer them yourself. When a worker's question is one its rules should have answered, propose the rule with `mull ask` as well, so the next worker doesn't ask it again. `mull inbox` shows what needs them: questions, decisions and reviews only.
- A decision the human makes in conversation, not in answer to an inbox entry, goes in with `mull decided "<their decision>" --human --on-request` (with `--item` or `--plan`): settled at once as theirs, into the decision log. To answer an entry, use `mull decide <entry-id>` instead.
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
When a worker runs `mull done`, you review before the human sees it. The order is: done, then your review of the diff, then you merge it into the target, then the human walks the tour on the preview, then accept.
1. Read `mull report <id>`: what was built (the summary, the tour, the check results) and how (the design notes, the decisions made in the session, the changes by layer). Check `warnings` too: a tour step that gives a local URL other than the plan's preview points at a server that won't be there. The tour should give paths, which `mull report --text` and `mull serve` resolve against the preview.
2. Look at the diff yourself (`git diff <base>..<commit>` from the report). Don't rely on the worker's summary. Check it against the plan's and phase's rules and the project's own: design system, duplication, real tests, deletions done.
3. If it holds up, merge its branch into its plan's target, so the plan's preview serves it. When the report has a UI tour and the plan has a preview, give the human the tour (`mull report <id> --text` has the steps as full URLs on the preview) and wait until they've walked it there. A plan with a server but no preview is the gap under "Before starting": raise it with `mull ask`.
4. Then run `mull accept <id>`. Add `--follow-up "<text>"` for small leftovers. A change the human asks for after acceptance is a follow-up too: `mull add "<title>" --follow-up <id>`. Done items are never reopened. Accepting also ratifies the decisions the review carries, with or without `--on-request`, and those are the human's call: if there are any, show them to the human first and accept with `--on-request` once they agree. If they decide one differently, run `mull decide <decision-id> "<their decision>" --on-request` before accepting.
5. If it doesn't, or the human finds a problem on the preview, find its review entry with `mull inbox --kind review` and send it back with `mull do <entry-id> --reason "<what's wrong>"`. The item reopens for another session, and its decisions stay open for the next review.
6. Tell the human what you accepted and what you sent back.

Unreviewed work is incomplete work. An item is in review after `mull done`, and done only once its review is accepted.

## Removing worktrees
Worktrees left behind pile up (dependencies and databases make each one big), and mull never removes them. You do, once both hold:
- the item is done (its review accepted), not just in review;
- its work is on its plan's target: the view shows no done-not-in-git problem for it.

Then, with the worktree and branch from its claim (`mull show <id>`), from a checkout of the target:
```
git worktree remove <path>
git branch -d <id>-<short-name>
```
Both refuse when something would be lost: `git worktree remove` a worktree with uncommitted or untracked changes, and `git branch -d` a branch not merged into its upstream (or into the branch you're on, when it has none). A refusal means there's work to look at, so find out what it is. Never force either (`--force`, `-D`) without the human's word. Leave alone a worktree whose item is still active or in review, or was sent back.

## End of session
- Run `mull sync`.
- Give the human a short status: the phases' progress, what landed on each plan's target branch (`mull view` shows each plan's target), what's blocked, the inbox count by kind, and how many captured items are untriaged.
