# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

`mull serve`: a local Go HTTP server inside the mull module (kit/mull), importing mull's own packages, with an embedded web UI and live updates pushed over Server-Sent Events. Served on `http://localhost:<fixed port>`, pinned in a browser tab. The current surface is a single-file HTML prototype; the production front-end stack is not decided yet.

## Users

One person: a solo developer running several programs of work with coding agents, across several git repos on one Mac. The agents do most of the writing; the human reads, reviews, decides and steers. The human holds themself to always understanding where each project stands and how the system is being built. They don't yet trust agents to build without review of both what was built (does it work) and how (design and architecture choices).

## Product Purpose

mull is the record of a program run with agents: plans, phases, items, a backlog, and an inbox of what needs the human. It lives on a `mull` git branch and agents use it through a CLI. The UI is the human's own view of that record. On opening a project it must answer two questions fast:
1. Where are we?
2. What's next, and what do I need to do or decide to take the next step?

Success is that the human never has to ask an agent "where are we", and that nothing needing them gets lost.

## Positioning

Unlike session dashboards (agent views, worktree managers), mull models the program above the session: a plan doc shared by the human and agents, phases ordered only by what they need, deterministic done checks, and a review of every finished item. Status always comes from records and git, never from agent prose. Works with any LLM and any harness.

## Operating Context

- Opened in four ways, all regularly: quick glances between agent sessions (is anything stuck on me?), sit-down reviews (finished work, the plan, the backlog), open beside a terminal while working through questions or triage with an agent, and a start- or end-of-day check-in across every project.
- Used on both a large external monitor and a laptop screen, often split with a terminal.
- Data changes underneath the page while agents work; the page must keep up live.
- Triage is a process the human runs with an agent, not a screen.

## Capabilities and Constraints

The information model (settled 2026-11-10):
- **Project** (a repo) → **Plan** (shaping → active → done; carries a shared doc of intent: the picture, done-when, why the order) → **Phase** (an epic; a sub-plan with its own doc and done-when; needs other phases only) → **Item** (one agent session; no ordering).
- **Item stages:** captured → committed → active → in review → done, or dropped. Any item can be started from captured (the fast path); every item is reviewed before it's done.
- **Backlog:** captured items of kind bug or idea, with a free-text category, optionally tagged to a plan or phase. Anyone captures; the human commits.
- **Unplanned work:** committed items with no plan.
- **Needs you:** questions, decisions and reviews only, ranked: blocking work, reviews, current-phase questions, later-phase questions, whole-plan questions, decisions to approve (grouped by source).
- **Review:** what was built (summary, tour, checks) and how (design notes, the session's decisions, files by layer), settled together.
- **No status in prose:** docs hold intent; status renders as live references to records.
- For now the priority is getting the flow and the data model right. The UI is read-first; actions will move into it later, so the design must leave room for them.
- Undecided: "epic" or "phase" as the name; how answers get folded back into the plan doc; how mull serve discovers repos.

## Evidence on Hand

Real records from the gallformers repo (plan taxogall: 11 phases, 28 items, 21 questions, 37 decisions, about 30 backlog captures) and from kit (plans mull-next and forage-rework). Some review content and item states in the prototype are invented to show screens and are marked; nothing else may be fabricated.

## Product Principles

1. Answer "where are we" and "what's next" before anything else.
2. Attention is ranked, never a flat pile. Only what needs the human is in Needs you.
3. Status comes from records and git, never prose.
4. Light process: no step is mandatory except review.
5. Everything is reachable through the hierarchy, and nothing gets lost below the fold.
