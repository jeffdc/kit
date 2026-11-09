# Attention layer prior art: decision logs, approval queues, agent inboxes, and running agent fleets

Research date: 2026-11-09. All dates are as reported by the sources. Verdict key: **use directly** (adopt the tool), **copy ideas** (borrow the design into a new tool), **learn from** (useful framing, nothing concrete to take).

## Tools for capturing agent decisions and questions and putting them in front of the human

### Takeaway
Approval and question queues are mature, but they are almost all **synchronous and tool-call-scoped**: they block one agent on one action (permission, a question, a plan). Nothing found gives an **asynchronous, cross-session queue of decisions agents already made and executed that still need the human to ratify them**. The nearest pieces are (a) agent-written decision records (AgDR, ADR-as-memory), which have no ratification step, and (b) Claude Code's agent view, which shows "Needs input" across sessions but only for live blocking questions. The gap the user describes, decisions buried in session summaries, looks genuinely unfilled.

### Cited Findings

**Synchronous approval and HITL infrastructure**
- HumanLayer (YC F24) started as an API/SDK that let tool-calling agents contact humans for approvals over Slack, SMS, and email, using a `require_approval()` decorator. A denial was fed back into the agent's context. — [Pushary vs HumanLayer](https://pushary.com/vs/humanlayer); [DEV teardown](https://dev.to/alexlee7359/i-tore-down-humanlayers-source-code-it-leashes-your-coding-agent-but-sessions-move-to-its-cloud-58j0)
- During 2025 HumanLayer pivoted to CodeLayer, an IDE that orchestrates Claude Code agents. A competitor reports the classic approvals SDK is now marked deprecated in the README, the old approval docs return 404, and the Python package has not shipped since mid-2025. (This comes from a competitor, so treat it with some caution.) — [Pushary vs HumanLayer (2026)](https://pushary.com/vs/humanlayer)
- HumanLayer's current approach moves the human checkpoint earlier, to the design document before any code is written (the "QRSPI" workflow, which is not open source). — [Webfuse: Human Approval Checkpoints (2026)](https://www.webfuse.com/blog/human-approval-checkpoints-for-ai-agents)
- HumanLayer's "Harness Engineering" post describes using hooks to auto-approve or auto-deny tool calls, with rules more expressive than the agent's default permission model. — [HumanLayer blog: Skill Issue](https://www.humanlayer.dev/blog/skill-issue-harness-engineering-for-coding-agents)
- Verdict on HumanLayer: **learn from**. The SDK is deprecated, sessions move to its cloud, and its insight (review the plan, not every action) is a pattern rather than a component.
- LangChain Agent Inbox (released Jan 16, 2025) is an open-source Next.js inbox UI for any LangGraph app that calls `interrupt()`. Its generic interrupt schema covers three action types: **Question, Review, Notify**. — [LangChain on X](https://x.com/LangChain/status/1879966219299877171); [GitHub langchain-ai/agent-inbox](https://github.com/langchain-ai/agent-inbox)
- Agent Inbox schema: `HumanInterrupt{action_request{action,args}, config{allow_ignore, allow_respond, allow_edit, allow_accept}, description}`. The response `HumanResponse.type` is one of `accept | ignore | response | edit`. — [GitHub langchain-ai/agent-inbox](https://github.com/langchain-ai/agent-inbox)
- LangGraph interrupts pause at any node. A checkpointer persists the state, so a human can answer hours or days later and the run resumes via `Command`. LangChain 1.0 also adds `HumanInTheLoopMiddleware` with per-tool `allowed_decisions: approve/edit/reject`. — [LangGraph Interrupts docs](https://docs.langchain.com/oss/python/langgraph/interrupts); [FlowHunt](https://www.flowhunt.io/blog/human-in-the-loop-middleware-python-safe-ai-agents/)
- Verdict on Agent Inbox: **copy ideas**. The Question/Review/Notify taxonomy and the accept/edit/respond/ignore verbs map directly onto a "decisions awaiting ratification" plus "blocking questions" queue. The runtime is LangGraph-only, so it doesn't fit Claude Code sessions.
- Other approval-layer products named in 2026 comparisons: Pushary (hooks for Claude Code, Codex, Cursor, Gemini CLI, and MCP tools), GotoHuman, Velt, Webfuse, Redouble AI. — [Pushary vs HumanLayer](https://pushary.com/vs/humanlayer). Verdict: **learn from**. All are per-action approval, not decision ratification.

**Claude Code native surfaces**
- **Agent view** (`claude agents`, research preview; the docs reference versions around v2.1.248 to v2.1.299; a third party says it shipped in May 2026):
  - One screen lists all background sessions, grouped as Pinned / Ready for review / Needs input / Working / Completed.
  - Each row has a one-line summary written by a Haiku-class model. For a waiting session the summary is the question itself (e.g. "double jump or wall climb?").
  - A peek panel shows numbered choices and lets you reply inline without attaching.
  - The terminal tab title shows a count like "2 awaiting input".
  - The `Notification` hook fires with `agent_needs_input` or `agent_completed`.
  - `claude agents --json` is the supported way to read state from scripts.
  - Each dispatched session gets its own worktree under `.claude/worktrees/`.

  — [Claude Code Docs: agent view](https://code.claude.com/docs/en/agent-view); [claudefa.st guide](https://claudefa.st/blog/guide/agents/agent-view)
- Verdict on agent view: **use directly** as the live "who's blocked right now" signal. `claude agents --json` and the `agent_needs_input` hook are clean inputs for a new tool. It does not record decisions that were already made, and its state is ephemeral, so it does not cover ratification or coverage.
- Hook gaps:
  - The Notification hook's `permission_prompt` matcher doesn't cover AskUserQuestion.
  - `idle_prompt` waits about 60 seconds.
  - ExitPlanMode awaiting approval fires no hook.
  - Workaround: a `PreToolUse` hook with matcher `AskUserQuestion` fires immediately. `PermissionRequest` also fires immediately.

  — [anthropics/claude-code #13024](https://github.com/anthropics/claude-code/issues/13024); [#13830](https://github.com/anthropics/claude-code/issues/13830)
- A hook can answer AskUserQuestion programmatically by returning `updatedInput` with an `answers` map plus "allow". — [Hooks reference](https://code.claude.com/docs/en/hooks)
- Community phone approvers built on hooks and ntfy:
  - **claude-remote-approver**: Approve / Always / Deny buttons; forwards AskUserQuestion.
  - **claude-code-phone-notifications**: warns that the AskUserQuestion hook must be `"async": true` or it blocks the dialog.
  - **claude-ntfy-hook**: ntfy plus Tailscale callbacks; shows the question text and options, and can accept plans.
  - **cc-remote-approval**: Telegram-based; covers permissions, AskUserQuestion, and MCP Elicitation.

  — [claude-remote-approver](https://github.com/yuuichieguchi/claude-remote-approver); [claude-code-phone-notifications](https://github.com/ssarnecki/claude-code-phone-notifications); [claude-ntfy-hook](https://github.com/nickknissen/claude-ntfy-hook); [DEV: approve remotely](https://dev.to/dan_288d398451d57/approve-claude-code-permission-requests-remotely-3bgm)
- Verdict on the phone approvers: **copy ideas**. A PreToolUse(AskUserQuestion) hook is the capture point for "questions blocking work": write the question to a durable queue as well as notifying.
- Community multi-session dashboards:
  - **Agent Deck** (tmux, polls every 2s, yellow "Waiting").
  - **claude-code-dashboard** (reads the JSONL logs under the Claude projects directory; thinking/waiting/idle/stale).
  - **Claude-Code-Agent-Monitor** (kanban, notifications).
  - **claude-dash** (about 1,300 lines of bash; a "needs you" state).
  - **CodeAgentSwarm**.
  - **Scope** (spawn/wait/poll primitives plus a display-only dashboard).

  — [DEV: Agent Deck](https://dev.to/asheshgoplani/how-to-manage-multiple-claude-code-gemini-and-codex-sessions-in-one-terminal-1dci); [HN Show: dashboard](https://news.ycombinator.com/item?id=47314375); [Claude-Code-Agent-Monitor](https://github.com/hoangsonww/Claude-Code-Agent-Monitor); [Joseph Lapin](https://josephlapin.com/blog/manage-multiple-claude-code-sessions/); [Scope](https://github.com/adagradschool/scope)
- Verdict on the dashboards: **learn from**. They all answer "which session is waiting", which agent view now covers natively. None extracts decisions.
- One practitioner has an orchestrator Claude agent spawn the others in cmux workspaces, with an Obsidian Bases dashboard grouping work as blocked / done / in-progress / review. — [artemxtech Substack](https://artemxtech.substack.com/p/how-i-manage-10-claude-code-agents)

**Decision records adapted for agents**
- **Agent Decision Records (AgDR)**, by me2resh, 2026; 53 stars when fetched:
  - Format: YAML frontmatter with `id` (AgDR-0001), `timestamp`, `agent`, `model`, `trigger` (user-prompt / hook / automation), and `status` (`proposed | executed | superseded`), plus optional `session` and `supersedes`.
  - Body: a Y-statement ("In the context of… I decided… accepting…"), an options table, Decision, and Consequences.
  - Files live in `docs/agdr/` with a README index.
  - Ships a Claude Code `/decide` command, Codex/Cursor/Copilot rules, a JSON Schema plus validator, and a pre-commit hook to enforce creation.
  - There is **no human ratification state**. Review happens implicitly at PR time.

  — [GitHub me2resh/agent-decision-record](https://github.com/me2resh/agent-decision-record); [me2resh blog](https://www.me2resh.com/blog/agent-decision-records)
- Verdict on AgDR: **copy ideas**. It is the closest prior art to the record format. Add a `ratified | rejected | needs-discussion` status and a human `ratified_by/at`, which AgDR lacks. The `session`, `model`, and `trigger` provenance fields are worth keeping.
- **ADRs as agent memory** (Andrew Connell, about late Oct 2026):
  - 141 ADRs accumulated in about four months.
  - A rule file makes Claude Code *propose* a new ADR whenever a decision is worth remembering.
  - Claude reads an index (`docs/adrs/README.md`, grouped by area, with title, status, and keywords) and opens only the relevant records.
  - When a decision changes, record why the old reasoning failed.

  — [andrewconnell.com](https://www.andrewconnell.com/articles/architecture-decision-records-ai-agents/)
- Other guidance: one decision per file, a front-matter block (tags, domain, scope, date), and an index agents read first. A rule of thumb: if reversing a decision would need a discussion, it deserves a record. Product and design decision records can sit alongside ADRs. — [WebsiteInit](https://websiteinit.com/blog/architecture-decision-records-for-ai-agent-codebases/); [Rost Glukhov](https://www.glukhov.org/app-architecture/documentation/decision-records-ai-driven-development/)
- Lightweight alternative: a single running `Decisions.md` log for calls too small for an ADR, kept separate from CLAUDE.md/AGENTS.md. — [Seekvana: Decision Journaling](https://seekvana.com/library/agentic-ai/decision-journaling)
- One argument for the split: an ADR answers "what did we decide and why", while AGENTS.md answers "given that, what must or must never happen". — [aiadvances.org](https://aiadvances.org/agents-md-is-the-ew-architecture-decision-record-adr-3cfb6bdd6f2c?gi=d498487a3c92)
- The canonical ADR repo now ships two Claude Code skills. It also lists agent-oriented tools:
  - Decision Guardian
  - Mneme HQ (ADR enforcement for agents)
  - ADR Guard (a GitHub Action that fails PRs touching watched code without an ADR)
  - kgai (an append-only decision log for coding agents)

  — [architecture-decision-record repo](https://github.com/architecture-decision-record/architecture-decision-record)
- A Claude skill claims to auto-detect "decision moments" in Claude Code sessions and record context, alternatives, and rationale as ADRs. — [aiagentskills.net](https://aiagentskills.net/skill/affaan-m-ecc-architecture-decision-records)
- Verdicts on the classic ADR tools (adr-tools, log4brains, MADR): **learn from**. I did not fetch these individually. Their value here is the MADR-style template and the status lifecycle, which AgDR already adapts.

### Inferences
- The two capture points are clear. **Questions** can be caught at the moment they are asked (PreToolUse on AskUserQuestion, or agent view's `agent_needs_input`). **Decisions** must be emitted by the agent itself, through a `/decide`-style command or tool call, an instruction in CLAUDE.md, or a Stop-hook sweep of the session summary. Nothing passively extracts them today.
- AgDR's `proposed → executed` lifecycle captures "the agent decided and acted". The user's missing state is "executed, awaiting ratification". That is the core new primitive.
- Agent Inbox's verbs (accept / edit / respond / ignore) are a good minimal action set for a ratification queue item.

### Gaps
- I did not verify adr-tools, log4brains, or MADR activity or features for 2025–2026.
- I found no product that offers an asynchronous "ratify decisions already taken" queue for coding agents. This is absence of evidence from about 15 searches, not proof.
- I did not check whether kgai, Mneme HQ, and Decision Guardian are active or what they do in detail.
- I could not confirm Slack-native approval flows for Claude Code beyond ntfy and Telegram.

## Practitioner thinking on supervising many agents (2025–2026)

### Takeaway
The consensus is that **human attention and verification, not generation, is the bottleneck**. Practitioners cap parallelism at review capacity, batch their check-ins, and push state into durable files or trackers (progress files, plans, Beads) instead of session memory. Yegge's Beads/Gas Town is the most developed system that makes work items persistent across "cattle" sessions. Anthropic's harness posts formalize progress files and structured feature lists as the handoff surface.

### Cited Findings
- **Simon Willison**, "Embracing the parallel coding agent lifestyle" (Oct 5, 2025):
  - He runs multiple Claude Code and Codex instances across terminals, directories, and worktrees.
  - Riskier tasks go to async cloud agents.
  - He reviews "one significant change at a time" but fires off research, maintenance, and directed tasks in parallel.

  — [simonwillison.net](https://simonwillison.net/2025/Oct/5/parallel-coding-agents/); [Pragmatic Engineer](https://blog.pragmaticengineer.com/new-trend-programming-by-kicking-off-parallel-ai-agents/)
- Willison reportedly told Lenny's Podcast (April 2026) that running four agents in parallel left him mentally exhausted before noon. This is secondhand. — [Xeve](https://xeve.io/blog/parallel-coding-agents-cognitive-cost)
- Willison keeps a running "parallel-agents" tag, including a "Running multiple coding agents in parallel" post on Mar 17, 2026. — [simonwillison.net/tags/parallel-agents](https://simonwillison.net/tags/parallel-agents/)
- **Addy Osmani**, "The Orchestration Tax" (May 24, 2026):
  - Your attention is the single serial resource, so architect it like a concurrent system.
  - Use a WIP limit: the agent count you can properly review, "a low single digit" for most people.
  - Split work into two queues: isolated delegable work vs. judgment work. Don't parallelize the judgment pile.
  - Batch reviews: "Let the work pile up a bit and process the batch."
  - Make agents self-verify (tests, screenshots) so you review only what needs judgment.
  - Give the bottleneck your best hours.

  — [addyosmani.com/blog/orchestration-tax](https://addyosmani.com/blog/orchestration-tax/)
- **Osmani**, "The Code Agent Orchestra" (Mar 26, 2026; O'Reilly CodeCon talk):
  - The human moves from conductor (one agent, synchronous) to orchestrator (many agents, asynchronous).
  - Slide: "Your attention = the bottleneck."
  - The human bottleneck used to slow how fast errors compounded; with agent armies, verification is the bottleneck.
  - Tools named: Conductor, Vibe Kanban, Gastown, Claude Squad, Antigravity, Cursor Background Agents.

  — [addyosmani.com/blog/code-agent-orchestra](https://addyosmani.com/blog/code-agent-orchestra/); [talk slides](https://talks.addy.ie/oreilly-codecon-march-2026/)
- **Osmani**, "Death of the IDE?": with many agents running, the real problem is knowing which one needs you now. That is why Conductor shows live progress across sessions and cmux added notification rings and unread badges to terminal panes. — [addyo.substack.com](https://addyo.substack.com/p/death-of-the-ide)
- **Steve Yegge, Beads** (Oct 2025):
  - A git-backed issue tracker and memory store for agents, with hierarchical TODOs for long-horizon multi-session work.
  - Agents find unblocked work (`bd ready`), claim it atomically (`bd update --claim`), and attach findings.
  - About 28.6k GitHub stars at fetch time.
  - Backend is now Dolt (versioned SQL) with hash IDs to avoid merge collisions. Supports deferred and pinned filters, a Message issue type with threading, mail delegation, and "semantic memory decay" that summarizes old closed tasks.
  - Yegge says he implemented the commands agents repeatedly expected to exist rather than prompting them out of it.

  — [GitHub steveyegge/beads](https://github.com/steveyegge/beads); [ai.engineer speaker page](https://ai.engineer/speakers/steve-yegge)
- Verdict on Beads: **use directly or copy ideas**. It is a strong candidate substrate for the coverage layer (ready/blocked/deferred queries, findings attached to issues). It has no decision-ratification concept, and the Dolt backend is heavy.
- **Yegge, Gas Town** (launched Jan 2026):
  - A Go/tmux orchestrator for 20–30 parallel agents, built on Beads ("the data plane and control plane").
  - Roles: Mayor, Polecats, Refinery, Witness, Deacon, Dogs, and the human **Overseer**.
  - "An agent is not a session": sessions are cattle thrown at persistent work tracked in Beads.

  — [ascii.co.uk](https://ascii.co.uk/news/article/news-20260102-190a5f9f/steve-yegge-releases-gas-town-multi-agent-orchestrator-for-c); [yegge.ai/gastown](https://yegge.ai/gastown); [rywalker.com research](https://rywalker.com/research/gastown); [Gas Town: from Clown Show to v1.0](https://steve-yegge.medium.com/gas-town-from-clown-show-to-v1-0-c239d9a407ec)
- Search summaries report that in an August 2026 essay Yegge abandoned his own Gas Town setup because agents kept tinkering with the orchestration instead of the application. His private successor, "Wheelhouse", kept Beads, planners, workers, separate reviewers, messaging, and merge processing. I could not fetch the primary essay. — via search results pointing to [blog.gascity.com](https://blog.gascity.com/authors/steve/) (unverified)
- Verdict on Gas Town: **learn from**. Its key lesson is that work identity should be persistent and separate from sessions. Its own author reports orchestration overhead.
- **Anthropic**, "Effective harnesses for long-running agents" (about Nov 26, 2025):
  - An initializer agent creates `init.sh`, `claude-progress.txt`, and an initial commit.
  - Each later session reads the progress file and git log, smoke-tests the app, implements one failing feature, then commits and updates progress.
  - `feature_list.json` (structured end-to-end features with pass/fail) prevents declaring victory early.

  — [anthropic.com/engineering](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)
- A follow-up (Mar–Apr 2026) adds a three-agent harness with context resets, structured handoff artifacts, and a separate evaluator agent calibrated with few-shot scoring. The `anthropics/cwc-long-running-agents` repo collects the patterns, using `PROGRESS.md`. — [InfoQ (Apr 2026)](https://infoq.com/news/2026/04/anthropic-three-agent-harness-ai/); [GitHub anthropics/cwc-long-running-agents](https://github.com/anthropics/cwc-long-running-agents)
- Verdict on the Anthropic harness posts: **copy ideas**. The structured feature list is the coverage-layer seed, and the progress file is the briefing source.
- **Geoffrey Huntley, "Ralph"** (July 2025):
  - A bash loop spawns a fresh agent each iteration. Memory is only `fix_plan.md` (later `IMPLEMENTATION_PLAN.md`), `AGENT.md`, and git.
  - Prompt rules include: bugs noticed, even unrelated ones, must be fixed or documented in the plan. Periodically clean completed items out with a subagent. Delete and regenerate the plan when it accumulates cruft.
  - A "planning mode" diffs specs against the implementation to regenerate the plan.

  — [ghuntley.com/ralph](https://ghuntley.com/ralph/); [how-to-ralph-wiggum](https://github.com/ghuntley/how-to-ralph-wiggum); [paddo.dev playbook](https://paddo.dev/blog/ralph-wiggum-playbook/)
- Verdict on Ralph: **copy ideas**. "Every finding goes in the plan" is the anti-drop rule, and spec-vs-implementation gap analysis is a coverage sweep.
- **Armin Ronacher** (2025–2026):
  - "Agentic Coding Things That Didn't Work" (Jul 30, 2025): context management remains a major challenge.
  - "The Coming Loop" (Jun 23, 2026): work goes into a queue, a machine attempts it, and an outer "harness-level loop" decides whether it is actually done.
  - Other 2026 posts include "The Final Bottleneck" (Feb 13) and "Agent Psychosis" (Jan 18). I did not read these.

  — [lucumr 2025/7/30](https://lucumr.pocoo.org/2025/7/30/things-that-didnt-work/); [The Coming Loop](https://lucumr.pocoo.org/2026/6/23/the-coming-loop/); [2026 archive](https://lucumr.pocoo.org/2026/)
- Verdict on Ronacher: **learn from**.
- **Mitchell Hashimoto**, "My AI Adoption Journey" (early 2026):
  - Agents try ideas in parallel and he reads the results the next morning.
  - Agents triage issues via the gh CLI.
  - **He turns agent notifications off so they don't interrupt him.**
  - He grows the harness (AGENTS.md rules, self-check tools) each time a failure recurs.
  - Background agents run only about 10–20% of the time.

  — [GIGAZINE summary](https://gigazine.net/gsc_news/en/20260211-my-ai-adoption-journey-6-steps/); [bloss0m](https://www.bloss0m.com/en/blog/16-mitchell-hashimoto-harness-origin/)
- Hashimoto, June 2025 Zed session: he briefs agents like a junior developer with guard rails, and AI-written code was under 25% of his recent work. — [Zed blog](https://zed.dev/blog/agentic-engineering-with-mitchell-hashimoto); [ldirer notes](https://ldirer.com/blog/posts/mitchell-hashimoto-zed-agents-2025)
- Verdict on Hashimoto: **learn from**. His "pull, not push" stance (no interrupts, review at chosen times) supports a briefing/queue design over notifications.
- Jesse Vincent (Sept 2025): an architect agent iterates on a plan, which fresh Claude Code instances review and then implement. — cited in [Willison](https://simonwillison.net/2025/Oct/5/parallel-coding-agents/)

### Inferences
- Two camps on notification: push (phone approvers, agent view notifications, cmux badges) vs. pull (Hashimoto disables notifications; Osmani batches reviews). For a solo developer supervising dozens of sessions, a durable queue the human drains in batches fits the expert advice better than more pings. Push should be reserved for true blockers.
- Every serious long-running system externalizes state into files or trackers (progress file, feature_list.json, fix_plan.md, Beads). The user's pain ("only in a session summary") is exactly the failure these systems are built to prevent, but none of them gives *decisions* a dedicated home with a ratify step.
- Yegge's reported abandonment of Gas Town is a warning against building heavy orchestration. A thin attention layer over existing sessions is probably the safer scope.

### Gaps
- I did not research Kent Beck's or Harper Reed's 2025–2026 workflow posts.
- I did not research Yegge's "Revenge of the junior developer" post directly.
- Yegge's Medium posts returned 403. Overseer role details and the August 2026 "abandoned Gas Town" claim come only from search summaries.
- I did not read Ronacher's "The Final Bottleneck" (Feb 2026), which is likely relevant.

## Coverage patterns: detecting dropped scope, orphaned TODOs, and untracked findings

### Takeaway
Mechanical coverage tools exist for code TODOs (TODO-to-issue Actions). The agent-era pattern is **"every finding goes in the tracker"** plus **periodic spec-vs-implementation sweeps** (Ralph planning mode, Anthropic's feature_list.json). I found no tool that detects in-scope items nobody is working on, or findings mentioned in session prose that were never filed.

### Cited Findings
- **alstr/todo-to-issue-action** (v5.1.15) creates, updates, and closes GitHub issues from committed TODO/FIXME comments.
  - It can write the new issue URL back into the comment.
  - Caveats: it ignores TODOs pushed before it was installed, and rebases can create duplicates.

  — [README](https://github.com/alstr/todo-to-issue-action/blob/master/README.md); [Marketplace](https://github.com/marketplace/actions/todo-to-issue)
- Other TODO trackers:
  - **ribtoks/tdg-github-action** (TODO/BUG/FIXME/HACK)
  - **dtinth/todo-actions** (closes the issue when the comment disappears; needs MongoDB)
  - **TODO-Sync** (also writes a markdown summary file)

  — [Track TODO Action](https://github.com/marketplace/actions/track-todo-action); [todo-actions](https://github.com/marketplace/actions/todo-actions); [TODO-Sync](https://github.com/Solo-Web-Works/TODO-Sync)
- Verdict on TODO trackers: **copy ideas**. Use the "close when the comment disappears" lifecycle and the summary file. GitHub Actions are the wrong runtime for a local tool.
- Ralph's anti-drop rule: any bug the agent notices must be fixed or written into the plan, even if unrelated. Planning mode regenerates the plan from spec-vs-code discrepancies. — [ghuntley.com/ralph](https://ghuntley.com/ralph/); [paddo.dev](https://paddo.dev/blog/ralph-wiggum-playbook/)
- Anthropic's `feature_list.json` is an explicit scope inventory with pass/fail per feature. It stops agents declaring the project done while features are unbuilt. — [anthropic.com/engineering](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)
- Beads lets agents attach findings to issues, filter deferred work, and query ready vs. blocked work. — [GitHub steveyegge/beads](https://github.com/steveyegge/beads)
- ADR Guard fails a PR that changes watched code without a matching decision record. This is a coverage check for decisions. — [architecture-decision-record repo](https://github.com/architecture-decision-record/architecture-decision-record)

### Inferences
- Coverage needs a reference set to diff against: a scope list (feature_list / plan / matters) vs. claims (in-progress work by session) vs. evidence (commits, PRs, TODOs, session-summary findings). "In scope with no active claim" and "mentioned in a summary but no tracked item" are both set differences. Nothing found does the second.
- Untracked findings could be caught by a Stop or SessionEnd hook (or batch sweep) that runs an LLM over the session transcript or summary to pull out findings, decisions, and questions, then diffs them against the tracker. That is an inference; I found no existing tool doing it.

### Gaps
- I found no practitioner essay specifically on "punchlist" practice or periodic coverage sweeps for agent programs.
- I did not check whether Beads has a `discovered-from` dependency type. The README I fetched lists blocks / related / parent-child / relates-to / duplicates / supersedes / replies-to, and does not mention it.

## Briefing and digest patterns ("what changed since you last looked")

### Takeaway
A few 2026 tools generate "what your agents did" digests from git and activity logs with a since-last marker. The best ideas are a `--since-last` cursor, diffing two digests, and building from structured activity rather than chat summaries. None covers decisions or open questions.

### Cited Findings
- **herdr-standup**: "a digest of what your agents actually did" across workspaces over a time window.
  - Covers commits, change volume, branch, and whether the work landed anywhere.
  - `--since` accepts any git date, and `--since-last` keeps a marker.
  - `--diff` compares two digests.
  - Outputs Markdown and a JSON snapshot.

  — [GitHub herdr-standup](https://github.com/moneycaringcoder/herdr-standup)
- Commonly PR #2267: a daily digest of "what your agents did while you were away", built from the structured activity recap rather than chat summaries. Format: "<Agent> in <pod>: <latest update> (+N more)". — [Team-Commonly/commonly #2267](https://github.com/Team-Commonly/commonly/pull/2267)
- The localvoxtral issue #720 proposal notes that ScreenPipe has day-recap and standup pipes, but nothing summarizes work with coding agents. — [localvoxtral #720](https://github.com/T0mSIlver/localvoxtral/issues/720)
- Agent view's per-session one-line summaries (Haiku-generated) and `claude agents --json` are a ready-made live-status source for a briefing. — [Claude Code Docs: agent view](https://code.claude.com/docs/en/agent-view)
- Git Digest (older SaaS) emails or Slacks AI summaries of code changes. — [devhunt Git Digest](https://devhunt.org/tool/git-digest)
- Verdicts: herdr-standup **copy ideas** (since-last cursor, digest diff, "did it land"). Commonly **learn from** (build from structured events, not prose). Git Digest **learn from**.

### Inferences
- A useful briefing for this user would be ordered by attention cost:
  1. Blocking questions
  2. Decisions awaiting ratification
  3. Coverage alarms
  4. Landed / unlanded work since the last look

  Only item 4 has existing prior art (herdr-standup). Items 1–3 depend on the attention and coverage layers.
- "Since you last looked" needs a human-side read cursor, as in herdr-standup's marker, not just timestamps.

### Gaps
- I found no rigorous practitioner essay on digests or briefings for solo multi-agent programs specifically. A Lloyd Moore piece, "The Standup Your Agents Can't Attend", was surfaced but not fetched.
