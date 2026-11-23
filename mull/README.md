# mull

The record of a program run by you and your agents, and the runner that moves its work through git. The design is in [docs/redesign.md](docs/redesign.md).

`make install` installs it as `mull`.

## What it does

- **Plans, phases, items.** A plan has a goal, an appetite and phases with done-criteria (its doc). Items are cut from a phase when it starts; an item is one agent's session. Phases can need other phases.
- **The rulebook.** RULES.md, in the repository, holds your rules, each with a check where one is possible. A ratcheted check's known violations may only shrink. Every agent gets the rulebook in its brief.
- **The runner.** `mull start` makes the item's branch and worktree from its plan's branch. `mull done` runs the checks once, scores risk from the diff alone, and lands low and medium risk work on the plan's branch. High risk (risky paths, a touched rule, a new connection in the code's map) waits for you. Nothing ever lands on main; you ship each plan with one PR.
- **The queue.** Bugs and ideas agents find are theirs to work, tagged by where in the code they are. Work in an area pulls in the queue there, and clusters show where the design is going wrong.
- **What needs you.** Questions come with the agent's default and a deadline; reviews are for high-risk work, and looks for a sample of what landed. Agents' other decisions go in their reports, never your inbox.
- **One view.** `mull status --text`, or `mull serve`: what needs you, what's trending worse, where each plan stands, and how work moved, since you last looked.
- **Stored in git, out of the way.** The record lives on a `mull` branch that is never checked out; `mull sync` exchanges it with the remote. Any agent, any harness: everything is the CLI.

## Getting started

```bash
mull init                      # the mull branch; unplanned work lands on next
mull config checks "make test" --on-request
mull hooks                     # refuse pushes to main
mull guide agents              # paste into AGENTS.md
mull guide intake              # setting up an existing project
mull serve                     # http://localhost:6855
```

## mull serve

A local web UI of every repository on mull, on 127.0.0.1 only. `mull serve --install` runs it as a macOS LaunchAgent. It writes only as you: accepting, deciding, sending back, queuing a bug or idea, agreeing or closing a plan.
