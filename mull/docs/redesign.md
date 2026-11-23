# mull: the redesign (agreed 2026-11-23)

Status: direction agreed with Jeff on 2026-11-23. Not built yet. Where this and `design.md` differ, this wins for anything new; `design.md` describes what's running today.

## Why

mull (Model v2) tracked work but cost more than it gave. Evidence from oaks, 2026-11-21 to 23:
- 341 decisions logged in three days, about 60% of them implementation detail (px, CSS classes). Each reached Jeff twice, in the inbox and in a report. Jeff approved everything, because no one can read that volume.
- About 78,000 words of reports, notes and decisions in 11 days. The few things that mattered (the component-library failure, the analysis of where 30 minutes per change went) were buried in it.
- Small changes took about 30 minutes, about 5 of them the change: cold-start worktrees, checks run twice, reports, tours, design notes, a review per item.
- About 11,000 words of protocol before an agent writes code; 43% of each agent brief was mull mechanics.
- Nothing held the architecture. Plan docs were intent, the decision log a flat list. Five page agents each passed their own review and built five versions of the same components.
- The backlog went stale: about 10 bugs were already fixed before triage. Moves were drop-and-re-add.
- mull's config pointed every plan at main, and one conversational choice ("roll out piecemeal on main") became a permanent rule in the log. So agents avoided integration branches, Jeff merged PRs one by one, and nobody removed worktrees.

What others found (research, 2026-11-23; sources at the end):
- Per-item approval collapses into rubber-stamping. People approve over 90% of agent prompts, and catch rates fall as volume rises.
- Prose rules barely change agent behaviour; executable checks do (about 50% violations with prompted rules, 15% with checks after generation, 0% with enforcement).
- Rule files rot and grow fast. Keep them short, a map pointing at checks.
- Drift is caught by machines on a schedule (recurring audit agents), and ratchets (a known-violations list that may only shrink) give the trend.
- Declare the top of the architecture; generate the rest from code.
- Heavy spec ceremony backfired (Spec Kit, Kiro). Short contracts with checkable criteria and an appetite survive.
- More than 2 or 3 parallel agents costs more in conflicts than it returns.
- Org-chart role designs are in retreat. The split that survives is plan, build, judge.

## What Jeff needs, and what agents need

Jeff is one person wearing every hat above the builders. He doesn't think one role at a time, so there are no role views. He needs:
- An architect's view: what the system is, its rules, and where it's trending bad.
- One move to regain footing: state a rule and have it enforced from then on.
- To see where the plans stand, and what needs him, in one place.
- Never to read micro-decisions, merge 20 PRs, or clean up worktrees.

Agents need a short brief (the task, its done-criteria, the rules in scope, where things are), a fast loop sized to the work, freedom to decide locally, and one duty: flag any contact with a rule.

## The shape

### 1. Rulebook (in the repo)

About 100 lines. Each rule has an ID, the files it covers (globs), and the check that enforces it, or is marked advisory when it has none, so it's visible which rules are only hopes. Each checked rule has a ratchet: a committed list of known violations that may only shrink, each entry with a reason. The count per rule over time is the trend line.

Branch policy and how-we-work rules (size tiers, parallelism cap) live here too.

Jeff owns it. Agents propose changes; they never change it on their own. A rule change lands with its check in the same commit.

The core loop: Jeff notices something, states the rule in a sentence, and an agent turns it into a check with a ratchet. Existing violations become work automatically.

### 2. Map (in the repo)

Jeff declares the top level: the parts and their allowed connections (for Elixir, ideally the Boundary config itself; for Go, go-arch-lint or a structural test). Everything below is generated from the code on every run, so it can't go stale. Drift is the gap between declared and generated.

### 3. Plans (in the repo)

A short shaped doc per plan: the goal, the appetite (fixed time, scope cut rather than slipped), the rules it adds, and phases with checkable done-criteria. Planning is a conversation with Jeff; its output is that doc and any new rules. Shared foundations are named and ordered before any phase fans out.

Items are cut from a phase just in time, when the phase starts, not up front.

### 4. Work queue (state, on the mull branch)

The agents' own. One line per item, tagged by map area and by what it was discovered from. No human gate: agents file it and work it.
- Small, in-area discovered work is fixed in the current change, with no record.
- Items are pulled into briefs by area ("also fix these").
- Clusters in one area become one item, and show to Jeff as a drift signal.
- Items fixed by later work are closed automatically with the commit. Duplicates merge when filed. Moving is a move; one item can cover others.
- Ideas (new capability, not discovered work) go to plan shaping, not the queue.

### 5. Runner

Owns branches and worktrees, so neither Jeff nor agents manage them.
- Each plan gets an integration branch when work starts; unplanned work goes to one standing branch.
- Items branch from their integration branch and merge back locally when checks pass. No PR per item.
- Jeff opens one PR to main when he decides a milestone ships. Commits are there to bisect.
- Jeff's preview serves the integration branch.
- At most 2 or 3 agents in parallel, on disjoint areas. Small work runs in the integration worktree directly.
- The runner creates worktrees and removes them once their branch is merged.
- No agent ever pushes to main, enforced by a pre-push hook and branch protection, never by a prompt.
- Risk is scored from mechanical signals (paths touched, diff size and structure, rules touched), never from the agent's account. Low-risk work lands with no review. A fixed random sample of landed work comes to Jeff; when a sample is bad, its whole family of similar changes is pulled.

### 6. Sensors

Deterministic checks gate every change (the hard checks, run by mull as today, once per change: not again on a fast-forward merge). A scheduled auditor agent compares code with the rulebook and map, proposes rules for recurring findings, keeps a grade per area, and opens small cleanup changes. LLM audits are for discovery; recurring findings get promoted to checks.

## How agents use it

The CLI, with a guide of about 300 words.
- **Start:** one command returns the brief: task, done-criteria, rules in scope, map area, related queue items. The branch and worktree are already made.
- **Capture:** one line.
- **Ask:** only for a rule conflict or something irreversible. An ask names the decision, the agent's default, and what happens if nobody answers by when.
- **Done:** one command runs the checks and records a one-line summary, whether any rule was touched, and what slowed the agent down. No design notes, tours or reports for small work.

## How Jeff uses it

- **The surface (mull serve, across every repo):** what changed since he last looked. In order: calls that need him, each with a default; rules trending worse, new connections between parts, hotspots, rework; each plan's progress (a hill chart: still figuring it out vs executing); everything else folded. Silence and failure look different.
- **Conversation:** a supervisor agent answers "where are we on compendium?" or "why is this slow?" from the same data, and shapes plans with him.
- **The repo:** rules, map and plans are plain files.
- **His moves:** shape a plan, assert a rule, answer asks, look at the preview, glance at samples, ship a milestone.
- **Ask budget:** if Jeff's approval rate on any gate nears 100%, that gate is deleted or automated.

## What changes from mull today

| Part | What happens |
|---|---|
| The mull branch, compare-and-swap writes, `mull sync` | Kept, for state only: progress, queue, claims, asks, check results, last looked |
| Hard checks run by mull | Kept, run once per change |
| mull serve | Kept; becomes the surface |
| Per-item reviews, the decisions inbox, required design notes and tours | Dropped |
| The capture-then-commit gate on the backlog, triage stamps | Dropped |
| Guides of about 15,000 words | Replaced by a guide of about 300 |
| Phases needing phases, item ranks | Revisit; likely replaced by the plan doc's order |
| Rulebook with ratchets, map generator, auditor, runner, risk routing and sampling | New |

## Moving existing projects

A one-time intake per repo, run by an agent, approved by Jeff in one sitting:
- Plans become short shaped docs. Committed items in phases not yet started are dropped back to phase outlines.
- Rules are mined from plan docs, the decision log, CLAUDE.md, AGENTS.md, CODING_STANDARDS.md and DESIGN.md. The agent drafts the rulebook and notes which rules already have a check; Jeff cuts and approves.
- Open bugs go to the queue; ideas go to plan shaping. Only real open questions survive as asks.
- Done items, reports and the decision log stay as read-only history on the mull branch.

## Build order (proposed)

1. **Cut the ceremony.** The 300-word guide; `done` records the three lines and runs checks once; low-risk work lands without a review; no decisions inbox. Fastest relief.
2. **Runner.** Integration branches, worktree create and remove, the parallelism cap, the pre-push hook.
3. **Rulebook and ratchets.** The format, the assert loop, rules in the brief, rule contact at done.
4. **Intake for oaks,** then kit.
5. **The surface** in mull serve.
6. **Queue changes.** Area tags, auto-close, clustering.
7. **Map generator and auditor.**

## Settled with Jeff (2026-11-23)

1. **Risk bands, from mechanical signals only.**
   - **High** waits for Jeff before it lands: it touches the rulebook, the map, checks, CI or hooks; data migrations; auth and security code; new dependencies; a new connection between parts; any path the project marks high-risk.
   - **Medium** lands and is always shown to Jeff afterwards: checks pass, but the diff is over about 400 lines or 15 files.
   - **Low** lands, sampled: everything else that passes checks.
   - The high-risk list is a starting point. It lives in each project's rulebook and grows or shrinks as we learn, by the normal rule change.
   - Sampling starts at 20% of low-risk work, chosen at random and fixed per change, and drops to 10% once samples come back clean for a few weeks. A bad sample pulls every similar change.
   - UI changes are never gated. The surface lists the pages that changed, to look at on the preview.
   - mull's layers and shared globs become the high-risk path list.
2. **The map is declared where its enforcing tool wants it, and the rulebook points there.**
   - oaks uses the Boundary library, as gallformers already does. Each context declares what it may call, and the compiler enforces it.
   - Go tools use a plain test that walks imports against a small declared table, with no new tool.
   - mull stays language-neutral: each project gives one command that prints its real connections. mull draws the map from that output and diffs it against the declared one.
3. **The runner makes worktrees, and agents use those.** Starting an item creates the branch and worktree and prints the path. The supervisor guide says never to use the harness's own worktree option. As a backstop, cleanup removes any worktree whose branch is merged and whose tree is clean, including harness-made and locked ones.
4. **The supervisor is any agent with the CLI.** mull ships a supervisor guide of about 300 words, down from 2,600. Its job: cut a phase into items, dispatch up to three agents, answer Jeff, and turn the rules he asserts into checks.
5. **mull serve's pages.**

   | Page today | What happens |
   |---|---|
   | Needs you, problems, revision schedule | Merge into the one surface: what changed since you last looked |
   | Plans, plan | Rewritten: the plan doc, a hill chart, and phases with done-criteria |
   | Rules | The rulebook: each rule, whether it has a check, and its ratchet trend |
   | Search, the project list across repos | Kept |
   | Review packet, item review | Dropped. Sampled and high-risk work shows a diff link and the agent's one-line summary |
   | Capture, backlog triage | Dropped. The queue is the agents' |
   | Phase, item | Plain read-only pages |
   | Map | New: declared against generated, with new connections highlighted |

## Sources

- OpenAI, Harness engineering: https://openai.com/index/harness-engineering/
- Böckeler, Harness engineering for coding agent users: https://martinfowler.com/articles/harness-engineering.html
- Anthropic, Measuring agent autonomy: https://www.anthropic.com/research/measuring-agent-autonomy
- Anthropic, Effective harnesses for long-running agents: https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents
- Shopify, A Packwerk retrospective: https://shopify.engineering/a-packwerk-retrospective
- Böckeler, Understanding spec-driven development: https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html
- Shape Up: https://basecamp.com/shapeup
- Elixir Boundary: https://github.com/sasa1977/boundary
- Cursor, Scaling agents: https://cursor.com/blog/scaling-agents
