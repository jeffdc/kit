# Agent-Oriented Task Trackers, Issue Trackers, and Memory/Handoff Systems (2025–2026)

Research date: 2026-11-09. Assessment axes: TRUTH (status derived from evidence such as git or tests, plus drift detection), ATTENTION (a queue of agent decisions awaiting ratification, and questions blocking work), COVERAGE (in-scope items nobody is touching, deferred items, findings never tracked).

## Which tools exist, and how does each work (data model, read/write, concurrency/worktrees, declared vs derived status, human-input surfacing, ready-work computation, UI)?

### Takeaway
The field splits into four groups. (a) Local agent-first trackers: Beads and its many clones, Backlog.md, Task Master, and Claude Code's native Tasks. (b) SaaS trackers with agent APIs: Linear, GitHub Copilot/Agent HQ, Plane. (c) Coordination sidecars: MCP Agent Mail, beads_viewer, Vibe Kanban. (d) Memory/handoff tools: Cline Memory Bank, Basic Memory, claude-mem, agentmemory, ai-memory. None of the agent-first local tools derives status from git evidence. In every one, an agent sets status by declaring it. The only clear evidence-derived status in the catalog is Linear's GitHub automation (PR opened/merged moves the issue's status). The only structured "agent asks human" primitive is Linear's `elicitation` / `awaitingInput`. COVERAGE-style analysis (stale, orphaned, or untracked work) exists only partially, in beads_viewer's graph triage (staleness, label "attention").

### Cited Findings

**Beads (`bd`, Steve Yegge; now the gastownhall org)**
- Self-description: "Distributed graph issue tracker for AI agents, powered by Dolt." The default embedded mode runs Dolt in-process at `.beads/embeddeddolt/` with a single writer. `bd init --server` uses an external `dolt sql-server` for multiple concurrent writers. — [GitHub README](https://github.com/steveyegge/beads)
- Cross-machine sync uses `bd dolt push/pull` against `refs/dolt/data` on a git remote. `.beads/issues.jsonl` is now "an export for viewers and interchange, not the source of truth or a backup." — [GitHub README](https://github.com/steveyegge/beads)
- Earlier versions used SQLite plus JSONL committed to git. Third-party ports describe the original design as "SQLite with JSONL export for git collaboration." — [beads_rust](https://github.com/Dicklesworthstone/beads_rust)
- Hash IDs (`bd-a1b2`) "prevent merge collisions in multi-agent/multi-branch workflows." Hierarchical IDs serve epics and subtasks (`bd-a3f8.1.1`). — [GitHub README](https://github.com/steveyegge/beads)
- `bd ready` lists tasks with no open blockers. `bd update <id> --claim` will "Atomically claim a task (sets assignee + in_progress)." Dependency types: blocks, related, parent-child, plus graph links relates-to/duplicates/supersedes/replies-to and cross-repo `external:` refs. — [GitHub README](https://github.com/steveyegge/beads)
- Statuses referenced include open, in_progress, hooked, closed, and deferred (via `--defer-*` flags). A Message issue type supports threading. There is no explicit question/approval feature. — [GitHub README](https://github.com/steveyegge/beads)
- "Compaction": semantic "memory decay" summarizes old closed tasks to save context. `bd prime` and `bd remember` exist for context and memory. — [GitHub README](https://github.com/steveyegge/beads)
- `bd init` writes an AGENTS.md section. `bd setup` installs hooks for Claude Code, Codex, Factory Droid, Cursor, and others. — [GitHub README](https://github.com/steveyegge/beads)
- Worktrees: `bd worktree create` "automatically shares the same beads database as the main repository via git common directory discovery." Redirect files were removed in v1.0.1, and the old `sync.branch` hidden-worktree workflow was removed. — [Beads worktree docs](https://beads.gascity.com/reference/worktrees); [v1.0.1 release](https://github.com/gastownhall/beads/releases/tag/v1.0.1)
- Open worktree bugs as of late 2026: an empty phantom store in a worktree "permanently shadows the main repo's database" (#6552), and shared-server mode writes runtime files into worktrees, blocking `bd worktree remove` (#7995). — [Issue #6552](https://github.com/gastownhall/beads/issues/6552); [Issue #7995](https://github.com/gastownhall/beads/issues/7995)
- UI: no core UI. The project points to community TUIs, web UIs, and native apps, and nearly 300 Beads viewer repos had been identified by Sept 2026. — [README](https://github.com/steveyegge/beads); [Beads first-birthday post, 2026-10-15](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)
- Roadmap (Oct 2026): "Memory Beads," a generic typed graph, the Beads Protocol (BDP) over HTTP, and a commercial "Beads Team Server" with governance, agent-run visibility, provenance, and cost tracking. — [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)

**beads_viewer (`bv`, Jeffrey Emanuel) and its Rust port `bvr`**
- A "graph-aware triage engine" over `.beads` data. It computes nine deterministic metrics (PageRank, betweenness, HITS, critical path, eigenvector, degree, density, cycles, topo sort) as a "sidecar" because "agents struggle with graph algorithms." — [beads_viewer AGENTS.md](https://github.com/Dicklesworthstone/beads_viewer/blob/main/AGENTS.md)
- Robot commands: `--robot-triage`, `--robot-next`, `--robot-plan` (parallel execution tracks), `--robot-insights`, `--robot-label-health`, `--robot-label-attention`, and `--robot-graph` (json/dot/mermaid). — [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer/blob/main/AGENTS.md)
- bvr triage ranks work by "composite impact (graph position × blockers × staleness × priority)" and can export a static-pages dashboard. — [beads_viewer_rust](https://github.com/Dicklesworthstone/beads_viewer_rust)

**MCP Agent Mail (Jeffrey Emanuel)**
- An "asynchronous coordination layer for AI coding agents: identities, inboxes, searchable threads, and advisory file leases over FastMCP + Git + SQLite." — [GitHub](https://github.com/Dicklesworthstone/mcp_agent_mail)
- `file_reservation_paths(... paths, ttl_seconds, exclusive, reason)` returns `{granted, conflicts}`. An optional pre-commit guard blocks commits that conflict with another agent's exclusive reservation. Recommended split: Beads holds task state, and Mail holds conversations, decisions, and attachments. — [GitHub](https://github.com/Dicklesworthstone/mcp_agent_mail)
- Rust port: in a 60-agent swarm test, the reservation calls missed the 5 s p99 budget. — [abhi12299/mcp_agent_mail_rust](https://github.com/abhi12299/mcp_agent_mail_rust)

**Beads alternatives and clones (2026)**
- grite: issues live as an append-only event log in `refs/grite/wal` with CRDT merging, which means no server, no DB, and no merge conflicts. About 23 stars. beads_rust (`br`): SQLite plus JSONL export, with "no automatic commits, pushes or pulls." hmans/beans: flat-file, with TUI and GraphQL, called "Much, MUCH less invasive than beads." henriquebastos/beans: SQLite plus git sync, written in response to Beads installing 5 git hooks and injecting prompts. ticket (wedow): a single bash script over markdown. Trekker: SQLite plus a local board plus a Claude Code plugin. linear-beads: uses Linear as storage. Older git-native trackers: git-bug and git-issue. — [HN: minimal beads-like tracker](https://news.ycombinator.com/item?id=46709872); [HN: I replaced Beads](https://news.ycombinator.com/item?id=46487580); [hmans/beans](https://github.com/hmans/beans); [henriquebastos/beans](https://github.com/henriquebastos/beans); [trekker](https://github.com/obsfx/trekker); [beads_rust](https://github.com/Dicklesworthstone/beads_rust)

**Backlog.md (Alex Gavrilescu / MrLesk)**
- A "Markdown-native Task Manager & Kanban visualizer for any Git repository." Each task is a `.md` file in `backlog/`. Git is optional (`--no-git`). — [GitHub](https://github.com/MrLesk/Backlog.md)
- Access is through the CLI (`backlog instructions overview`, `task create/edit/list`, versioned `--json`, `task list --json --watch`) and an MCP server (`backlog mcp start`, with workflow resources such as `backlog://workflow/overview`). The MCP server "follows the client's workspace or worktree unless `BACKLOG_CWD` pins it." — [GitHub](https://github.com/MrLesk/Backlog.md)
- Cross-branch awareness: config options `checkActiveBranches`, `remoteOperations`, and `activeBranchDays`. A task ID is released on delete only when "no other branch or worktree still holds that task." — [GitHub](https://github.com/MrLesk/Backlog.md)
- Status is declared ("To Do" through a configurable Done). Also: dependencies (shows waits-on / waited-by), milestones, `decision list`, `draft list`, docs, and fuzzy search across tasks, docs, and decisions. — [GitHub](https://github.com/MrLesk/Backlog.md)
- UI: `backlog browser` (local web board with drag-and-drop, acceptance-criteria checklists, real-time updates), `backlog board` (terminal kanban), `board export` (markdown report), and a community VS Code extension. — [GitHub](https://github.com/MrLesk/Backlog.md)
- Workflow: three human review checkpoints (task spec, implementation plan, code review), with one task per agent session and per PR. Tasks have five sections (frontmatter, description, acceptance criteria, implementation plan, implementation notes) and a reusable Definition of Done. — [hysenlabs review](https://hysenlabs.com/projects/mrlesk-backlog-md); [author blog, 2026-07-07](https://mrlesk.com/blog/building-backlog/)

**Task Master (claude-task-master, Eyal Toledano)**
- PRD-driven. A PRD at `.taskmaster/docs/prd.txt` is parsed into tasks. Access is through the MCP server (`npx -y task-master-ai`) or the `task-master` CLI. Tools include `next`, `analyze_project_complexity`, `complexity_report`, and `move` with `--from-tag/--to-tag` and `--with-dependencies`. — [GitHub](https://github.com/eyaltoledano/claude-task-master)
- tasks.json is organized into "tags" (separate contexts, e.g. `master`, `feature-branch`). Statuses: pending, in-progress, done, review, deferred, cancelled. `next` picks pending/in-progress tasks whose dependencies are satisfied. Marking a parent done also marks its subtasks done. — [Task Structure docs](https://docs.task-master.dev/capabilities/task-structure); [task-status.js](https://glama.ai/mcp/servers/@eyaltoledano/claude-task-master/blob/f961235442aa88e4983b29ab458b6a476fadf370/src/constants/task-status.js)
- Multi-agent concurrency is not described in the README. There is no dedicated UI; it runs inside Cursor, Windsurf, VS Code, and Claude Code. — [GitHub](https://github.com/eyaltoledano/claude-task-master)

**Claude Code native Tasks (Anthropic, ~Jan 2026, v2.1.16+)**
- Tasks replaced Todos and are exposed via TaskCreate/TaskUpdate/TaskList/TaskGet. Dependencies use `addBlockedBy`. Statuses: pending → in_progress → completed. — [paddo.dev: From Beads to Tasks](https://paddo.dev/blog/from-beads-to-tasks/); [claudefa.st guide](https://claudefa.st/blog/guide/development/task-management)
- Storage is one JSON file per task under `~/.claude/tasks/<list-id>/`, outside the repo. Each session gets a fresh list unless `CLAUDE_CODE_TASK_LIST_ID` is set. Updates broadcast to all sessions on the same list. — [claudefa.st](https://claudefa.st/blog/guide/development/task-management)
- Worktree pitfall: since v2.1.63, project config sits at the repo's shared root, so an ID set in settings.json puts every worktree on the same list. The suggested workaround is `export CLAUDE_CODE_TASK_LIST_ID="$(basename "$PWD")"`. Lists are not repo-scoped, so tasks can leak across repos. — [DEV Community](https://dev.to/greatworkco/how-to-share-claude-codes-task-list-across-sessions-and-keep-worktrees-apart-37ho)
- The Beads team says Claude Code Tasks were "reportedly inspired by Beads." — [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)

**Linear (agent platform plus Linear Agent coding sessions)**
- You delegate by assigning an issue to an agent, or @mention it. The human who delegated stays the primary assignee. Agents don't consume billable seats (as of Oct 2026). — [Linear docs: Agents](https://linear.app/docs/agents-in-linear); [workmanagementhub summary](https://workmanagementhub.com/linear-agent-feature-deep-dive-2026/)
- Agent Interaction API session states: `pending`, `active`, `error`, `awaitingInput`, `complete`, and `stale`. Linear derives them from the last activity the agent emitted, so the agent does not set them directly. Activity types: thought, elicitation (a request for clarification or confirmation), action, response, error. The human reply arrives as a `prompt` activity. — [Linear Developers: Agent Interaction](https://linear.app/developers/agent-interaction)
- "Agent Plans" are a session-level checklist (pending/inProgress/completed/canceled), sent whole on every update. Technology preview. — [Linear Developers](https://linear.app/developers/agent-interaction)
- "Agent guidance": workspace-level and team-level markdown instructions that agents receive automatically, with history. — [Linear docs](https://linear.app/docs/agents-in-linear)
- Evidence-derived status: the GitHub integration moves issues automatically (branch created → In Progress, PR opened → In Review, PR merged → Done). Rules can be branch-specific, and a "ready for merge" trigger fires when checks pass. Linking requires the issue ID in the branch, commit, or PR. — [Linear on X](https://x.com/linear/status/1724851292583895482); [Linear GitHub marketplace](https://github.com/marketplace/linear); [feedvote guide](https://feedvote.app/blog/linear-github-integration)
- Coding sessions (June 2026): Linear Agent writes code in a managed sandbox via Claude Code or Codex and drafts a PR. Third-party agents (Codex, Cursor, Copilot, Devin, Factory, Warp, Cyrus, and others) plug in. A hosted MCP server is also available. — [Linear coding sessions](https://linear.app/docs/coding-sessions); [Linear agents directory](https://linear.app/integrations/agents); [aq.dev](https://aq.dev/docs/linear-integration/)

**GitHub Issues / Copilot coding agent / Agent HQ**
- You assign an issue to Copilot (web, mobile, CLI, or VS Code). It runs on Actions and pushes commits to a draft PR, updating the PR description as it goes. — [GitHub Blog: coding agent](https://github.blog/news-insights/product-news/github-copilot-meet-the-new-coding-agent/)
- Mission Control (announced Oct 2025) is the dashboard at github.com/copilot/agents for assigning, steering, and tracking agent tasks. Custom agents can be defined in `.github/agents/`. Agent HQ lets you assign an issue to Copilot, Claude, Codex, or all three. — [agentpatterns.ai](https://agentpatterns.ai/tools/copilot/agent-mission-control/); [GitHub Blog: Pick your agent](https://github.blog/news-insights/company-news/pick-your-agent-use-claude-and-codex-on-agent-hq/)
- The source of truth is the GitHub issue plus its linked PR. Closing an issue through a "closes #N" PR merge is standard GitHub behavior, which counts as evidence-derived. (This is general knowledge; I did not fetch a 2026 source for it.)

**Plane**
- Open-source PM tool with REST API, webhooks, typed SDKs, and an official MCP server (`@makeplane/plane-mcp-server`). "Plane Agents" were added 2026-09-30: user-defined agents with instructions and MCP tools. A third party says the agents do "triage, deduplication, blocker detection, and status reporting." — [Plane changelog 2026-09-30](https://plane.so/changelog/2026-09-30-plane-agents-propel-new-ai-models); [Plane engineering](https://plane.so/solutions/engineering); [fast.io roundup](https://fast.io/resources/best-ai-agents-project-management-2026/)

**Vibe Kanban (bloop)**
- Apache-2.0, local-first web app: a kanban board, a git worktree and branch per task, inline diff review with feedback to the agent, and PR creation. Supports Claude Code, Codex, Gemini, Copilot, Amp, Cursor, OpenCode, and others. — [tekai.dev](https://tekai.dev/references/2026-04-11-vibe-kanban-ai-agent-management); [vibecoding.app review](https://vibecoding.app/blog/vibe-kanban-review)
- bloop shut down on 2026-04-10. The last release was v0.1.44 (2026-04-24), the README says it is sunsetting, and the board pages were retired. — [vibecoding.app](https://vibecoding.app/blog/vibe-kanban-review); [hysenlabs](https://hysenlabs.com/projects/bloopai-vibe-kanban)

**HumanLayer (human-approval infra)**
- The original v1 offered a `require_approval()` decorator and `human_as_tool()`, routed to Slack or email, and fed denials back into agent context. In 2026 it pivoted to an AI coding IDE ("CodeLayer"), and according to a competitor the approvals API is no longer on the site. — [Orako comparison (competitor, possibly biased)](https://orako.io/compare/humanlayer-alternative); [AI Engineer org profile](https://ai.engineer/orgs/humanlayer)
- AgentClick (arXiv 2604.16520) is a browser-based human-in-the-loop review layer for terminal agents. "Approval fatigue" is documented as a pattern. — [AgentClick](https://arxiv.org/pdf/2604.16520); [AI Agents Push Humans Out of the Loop](https://arxiv.org/pdf/2608.23642)

**Memory/handoff systems**
- Cline Memory Bank is a documentation convention: a rules file plus about six markdown files (e.g., `activeContext.md`), maintained by the agent following custom instructions. `/newtask` is the closest handoff primitive. One tip is to stop the agent from updating the bank until a task is fully done. — [fast.io comparison](https://fast.io/resources/cline-claude-code-memory-comparison/); [fast.io guide](https://fast.io/resources/cline-memory-bank-guide/)
- Basic Memory (AGPL-3.0) is an MCP server. Markdown files are the source of truth, SQLite is the index, and WikiLinks become graph relations. Local use is free; cloud is $15/mo. — [agentsroom.dev, July 2026](https://agentsroom.dev/blog/ai-agent-memory-solutions)
- claude-mem has a runtime worker, SQLite metadata, optional vectors, a web UI, and MCP search/timeline tools. agentmemory offers recall, recap, handoff, lesson, and commit-context skills. ai-memory (akitaonrails) does cross-vendor handoff, e.g. quit Claude Code and the next Codex session gets "where you left off, what failed, and what's still open." — [agentmemory](https://github.com/rohitg00/agentmemory); [ai-memory](https://github.com/akitaonrails/ai-memory)
- Claude Code auto-memory lives per project at `~/.claude/projects/<project>/memory/`, outside the repo, and is invisible to CI agents. — [fast.io comparison](https://fast.io/resources/cline-claude-code-memory-comparison/)

### Inferences
- In every local agent-first tracker (Beads, Backlog.md, Task Master, Claude Tasks, the clones), status is declared. An agent runs `close`, `set-status`, or `TaskUpdate`. None reconciles status against commits, merges, branches, or test results. This is exactly the failure mode the user observed (hub says done, field says planned). It is a real gap in prior art, not something an existing tool solves.
- Linear is the closest prior art for TRUTH (PR/branch events drive status, keyed on the issue ID in the branch name) and for ATTENTION (`elicitation` → `awaitingInput` as a first-class session state, which Linear derives rather than letting the agent set). Both patterns transfer well to a local git-backed tool: parse issue IDs from branch names and commit trailers to derive status, and make "asks a question" a typed event rather than prose.
- Concurrency approaches seen: hash IDs (Beads), atomic claim (Beads `--claim`), Dolt cell-level merge or a shared server, CRDT event log in git refs (grite), advisory file leases plus a pre-commit guard (Agent Mail), and a DB in the git common dir shared across worktrees (Beads `bd worktree`). Storing tracker data per worktree in the working tree is the pattern that diverges. The prior-art fix is a single store located via the git common dir, or in a git ref.
- COVERAGE has the weakest prior art. beads_viewer's staleness weighting and `--robot-label-attention` are the nearest thing. Nothing found detects "findings that never became tracked work" (e.g., scanning agent transcripts or notes for untracked TODOs).
- No tool found ratifies decisions, i.e. records an agent's autonomous decision and holds it as pending until a human approves. Backlog.md has `decision list`, which is a document store, not a queue. Agent Mail stores "decisions" in threads. The Beads "Bee" incident (below) shows why it matters.

### Gaps
- I couldn't verify Backlog.md's decision-record schema (fields, statuses) beyond the `decision list` command's existence.
- Task Master's multi-agent/worktree behavior is undocumented in sources fetched. Tags appear to be per-branch contexts, but I didn't confirm whether they auto-map to git branches.
- I didn't research Kiro specs, GitHub spec-kit, Agent OS, Shrimp Task Manager, or the commercial "Beads Team Server" in depth.
- Whether GitHub Copilot or Agent HQ surfaces agent questions to humans as a structured queue, as opposed to PR comments, was not found.

## Maturity (stars, last release, contributors, license)

### Takeaway
Beads (~28.6k stars, MIT, v1.3.4 on 2026-11-04) and Task Master (~28.3k stars, MIT + Commons Clause) are the most-adopted. Backlog.md (~7.3k, MIT) is mid-size and actively maintained. Most clones are tiny. Vibe Kanban is effectively abandoned (company shut down April 2026).

### Cited Findings
- Beads: 28.6k stars, 1.9k forks, ~1.1k open issues, 640 PRs, 11,575 commits, MIT. — [GitHub](https://github.com/steveyegge/beads)
- Beads: v1.0.0 on 2026-04-02, Go package v1.3.4 published 2026-11-04, and "hundreds of contributors." About 1.72M GitHub release downloads as of 2026-10-14 (up from ~864k on 2026-08-04), 26,682 Homebrew installs per year, and 77.5k PyPI `beads-mcp` downloads in 180 days. — [pkg.go.dev](https://pkg.go.dev/github.com/steveyegge/beads); [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)
- Beads origin: first commit 2025-10-11. Yegge had "605 Markdown plan files" that agents kept generating and losing track of. — [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)
- Beads telemetry: reads (list/context/show/query) are more than two-thirds of all commands, and 39–48% of machines ran each core-loop command. — [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)
- Backlog.md: 7.3k stars, 482 forks, 75 open issues, 1,496 commits, MIT. The author's one-year retrospective is dated 2026-07-07. — [GitHub](https://github.com/MrLesk/Backlog.md); [blog](https://mrlesk.com/blog/building-backlog/)
- Task Master: 28.3k stars, 2.6k forks, 1,216 commits. License is MIT with Commons Clause (no selling or hosting it as a service). Latest release date not captured. — [GitHub](https://github.com/eyaltoledano/claude-task-master)
- Vibe Kanban: 27.6k stars before shutdown. Last release v0.1.44 on 2026-04-24, last push 2026-09-17. — [vibecoding.app](https://vibecoding.app/blog/vibe-kanban-review)
- grite: about 23 stars. — [HN thread summary](https://news.ycombinator.com/item?id=46709872)
- Claude Code Tasks: announced around 2026-01-23 by Thariq (Anthropic). Requires Claude Code 2.1.16+. — [Thariq on X](https://x.com/trq212/status/2014480496013803643)
- Linear coding sessions launched June 2026. Plane Agents launched 2026-09-30. — [Linear](https://linear.app/now/coding-sessions-for-linear-agent); [Plane changelog](https://plane.so/changelog/2026-09-30-plane-agents-propel-new-ai-models)

### Inferences
- Beads' fast pace (11.5k commits in about a year, an agent-written codebase) brings breakage risk. Its open worktree bugs and its storage migration (SQLite/JSONL → Dolt) suggest it is not a stable foundation to build on.

### Gaps
- Exact contributor counts and latest release dates for Backlog.md and Task Master weren't shown on the fetched pages.
- Star and maturity data for beads_viewer, MCP Agent Mail, Basic Memory, claude-mem, and hmans/beans was not collected.

## What do practitioners report (what works, what breaks)?

### Takeaway
What works: replacing sprawling markdown plans with structured, queryable items that survive compaction, plus acceptance criteria as a check on agents' claims of completion. What breaks: invasiveness (git hooks, auto-commits, prompt injection into config), merge conflicts that silently drop task data, buggy agent-written code, agents forgetting to use the tracker unless prompted, and review gates that a "yolo" agent ignores. A cautionary incident: Beads' own autonomous maintainer agent shipped an unready release because the real process was undocumented.

### Cited Findings
- Beads' origin story mirrors the user's own problem: 605 markdown plan files that "agents kept generating, revising, and losing track of." An issue tracker let agents "find, organize, and resume their own work." — [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)
- Yegge: "the complicated Beads CLI isn't for humans; it's for agents." He kept adding the commands agents tried to use until nearly every guess worked. — [search summary of "Software Survival 3.0"](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/) (attribution via the birthday post; original essay not fetched)
- Incident (2026-08-11): the autonomous maintainer agent "Bee" released Beads v1.2.1 to GitHub, Homebrew, PyPI, and npm without release-specific human approval. The release included a hard-to-roll-back schema upgrade. Bee followed the documented process, but "much of the team's actual process was undocumented and lived in human conversations." — [first-birthday post](https://blog.gascity.com/posts/fun-facts-about-beads-on-its-first-birthday/)
- HN criticism: Beads is "240,000 lines of code" and "the code quality is abysmal." Yegge has admitted he has never read the code. — [HN 46669791](https://news.ycombinator.com/item?id=46669791)
- HN: "really buggy, and the bugs seem to confuse the agent." Merge conflicts where the agent kept one side's changes "wiped out their task work." — [HN 46467414](https://news.ycombinator.com/item?id=46467414)
- HN: the Dolt background server used a constant 3–30 kB/s of I/O, and Beads "seems way over engineered." A counterpoint: building your own gives "appreciation for Beads' architecture" because the redundancies cover edge cases where agents fail. — [HN 47772311](https://news.ycombinator.com/item?id=47772311)
- HN: "why not just use github issues?" Also: Beads "has great primitives, but... become a bit unwieldy." — [HN 46508759](https://news.ycombinator.com/item?id=46508759)
- Backlash over invasiveness (global installs, git hook interference, sync problems), with one thread asking whether it is "uninstallable malware." — [asksurf.ai](https://asksurf.ai/pulse/en/beads-backlash-highlights-tooling-friction)
- Positive review: "Claude doesn't proactively use it." You must say "track this in beads" or "check bd ready." — [ianbull.com](https://ianbull.com/posts/beads/)
- Backlog.md author (2026-07-07): tasks survived compaction. Acceptance criteria made agents' completion reports more accurate and reduced scope drift. "A vague task file is just a vague prompt saved to a file." The kanban fits agents only partly, since in-progress can be very brief. The board is more useful for after-the-fact review than live monitoring. The tool's value shifted toward helping the human "understand and review the work." — [mrlesk.com](https://mrlesk.com/blog/building-backlog/)
- Backlog.md caveat: review gates "depend on agent discipline to seek approval; a yolo-configured agent can steamroll them." Comparison from the same review: Task Master imposes structure; Beads suits multi-agent concurrency; Backlog.md suits human reviewability. — [tomrochette.com](https://tomrochette.com/agents/task-management/backlog-md/)
- Vibe Kanban: the kanban + worktree + inline-diff workflow is "still one of the best ways to run agents in parallel." The overheads are workspace setup, monitoring concurrent diffs, and resolving merge conflicts across worktrees. — [vibecoding.app](https://vibecoding.app/blog/vibe-kanban-review)
- Memory advice: a solo developer running agents daily across projects "want[s] automatic capture, because you won't maintain five files by hand." — [agentsroom.dev](https://agentsroom.dev/blog/ai-agent-memory-solutions)

### Inferences
- The recurring lesson is that agent self-reports can't be trusted. Backlog.md mitigates this with acceptance criteria and human checkpoints, but it still relies on the agent ticking boxes. That supports the user's thesis that status needs an independent evidence source and a skeptical auditor, not more structure in the agent's own writes.
- "Agents forget to use the tracker" argues for hooks or session-start injection (Beads' `bd prime` and AGENTS.md injection) and for deriving state passively from git, which needs no agent cooperation.
- The invasiveness backlash argues for opt-in, explicit integration: no auto-commits, and no hooks installed without consent.

### Gaps
- I found no first-hand practitioner reports about Task Master drift or false "done" claims.
- No reports were found on Claude Code Tasks reliability across many parallel worktrees beyond the config-sharing pitfall.

## Verdicts and TRUTH / ATTENTION / COVERAGE assessment per tool

### Takeaway
No existing tool covers TRUTH, ATTENTION, and COVERAGE together. Building the new tool is justified, but specific mechanisms should be copied: Linear's PR-driven status and `awaitingInput`/elicitation, Beads' `ready`/`--claim`/hash IDs/git-common-dir store, beads_viewer's deterministic robot-mode triage, Agent Mail's leases, and Backlog.md's acceptance criteria and checkpoints.

### Cited Findings
(The evidence for each row is cited in the sections above. Ratings are my assessment.)

| Tool | TRUTH | ATTENTION | COVERAGE | Verdict |
|---|---|---|---|---|
| Beads (bd) | Declared (agent `close`). No git or test reconciliation. | None structured. Message type only. | `ready` / blocked / deferred. Compaction. | **Copy ideas**: hash IDs, atomic claim, `ready`, `prime`, a shared DB via git common dir. Too heavy, invasive, and unstable to adopt. |
| beads_viewer (bv/bvr) | N/A (reads Beads) | `--robot-label-attention` (label-level) | Staleness-weighted triage, critical path, cycles | **Copy ideas**: deterministic robot-mode JSON triage as an auditor's input. |
| MCP Agent Mail | N/A | Threads can hold decisions, but there is no ratification state | File leases show who is touching what | **Learn from**: advisory leases plus a pre-commit guard for parallel worktrees. |
| grite / hmans/beans / beans_rust / ticket / trekker | Declared | None | Minimal | **Learn from**: grite's CRDT event log in git refs answers per-worktree divergence. |
| Backlog.md | Declared. Acceptance criteria / Definition of Done checkboxes. | Human checkpoints (spec/plan/review). `decision list` with no approval state. | Milestones, archive, drafts | **Copy ideas**: AC + implementation notes + checkpoints, cross-branch task awareness, web board. Closest in shape to the user's tool. |
| Task Master | Declared (`set-status`). Has a `review` status. | `review` status only | `next` honors dependencies; `deferred` status | **Learn from**: PRD → tasks decomposition and complexity analysis. Commons Clause; not git-aware. |
| Claude Code Tasks | Declared. Stored outside the repo. | None | `addBlockedBy` | **Learn from**: it is the agent's working memory, not the program's record. Keep the tool separate and repo-scoped. |
| Linear | **Partially derived**: branch/PR/merge events move status | **`elicitation` → `awaitingInput`**, derived session state | Views by delegate; triage automations | **Copy ideas**: ID-in-branch linkage driving status; typed question events; derived session state. SaaS doesn't fit a local, git-native solo setup. |
| GitHub Issues + Copilot / Agent HQ | Partially derived (PR "closes #N"; draft PR progress) | PR comments only (not found to be structured) | Projects views | **Learn from**: Mission Control as a cockpit pattern. Could serve as a backend but adds a SaaS dependency. |
| Plane | Declared, with agent "blocker detection / status reporting" (third-party claim) | Not found | Agent-driven triage | **Learn from**: low priority. |
| Vibe Kanban | Diff/PR-centric | Inline diff feedback | Board per task/worktree | **Learn from**: GUI cockpit UX (worktree-per-task plus diff review). Abandoned, so don't depend on it. |
| HumanLayer v1 / AgentClick | N/A | Approval decorators, human-as-tool, browser review layer | N/A | **Learn from**: approval routing and the approval-fatigue risk. |
| Cline Memory Bank / CLAUDE.md / AGENTS.md | Prose, agent-maintained, which is the drift source | None | None | **Learn from (anti-pattern)**: prose hubs written by agents are what drifts. |
| Basic Memory / claude-mem / agentmemory / ai-memory | N/A | Handoff notes ("what's still open") | Session capture could seed "findings never tracked" | **Learn from**: automatic session capture as raw input for a COVERAGE scanner, not as truth. |

### Inferences
- The new tool's distinctive value would be (1) a reconciler that compares declared status against git evidence (branch exists, commits reference the ID, merged to main, tests pass) and reports drift, (2) a typed "decision pending ratification" / "question blocking" record with a derived awaiting-human state in the style of Linear, and (3) coverage scans: in-scope items with no branch or commit activity, deferred items past their review date, and findings in agent notes or transcripts with no linked item. I found no existing tool that does (1) locally, (2) as ratification, or (3) at all.
- Practical takeaways for storage: put the store at the git common dir or in a dedicated ref so all worktrees share one copy, use hash IDs, and use an atomic claim. These are proven answers to the user's per-worktree divergence.

### Gaps
- None of the per-tool ratings above were tested hands-on. They are based on documentation and third-party reviews.
