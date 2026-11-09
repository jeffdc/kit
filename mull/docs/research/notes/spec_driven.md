# Spec-driven development (SDD) tools and plan/milestone frameworks for AI coding agents (2025–2026)

Research date: 2026-11-09. Scope: how SDD and plan-centric tools turn intent into plan, tasks and verification, and whether any of them keep plan and reality aligned over a long program (truth, attention, coverage, milestones, auditor). Star counts are as shown on the GitHub pages on 2026-11-09 unless noted.

## Which tools/frameworks exist, and how does each structure intent → plan → tasks → verification?

### Takeaway
The field has settled on one pipeline: principles/steering → requirements/spec → (clarify) → design/plan → tasks checklist → implement → (verify). Almost every tool stores state as markdown that the agent edits. Task "completion" is nearly always an agent ticking `- [x]` in tasks.md. Verification, where it exists, is an LLM reading code against the spec. None of these tools derive status from git or test evidence. Each one is built around a single feature or change. None is built around a multi-week program.

### Cited Findings

**Overview / taxonomy**
- Böckeler (martinfowler.com, 2025-10-15) defines three levels: *spec-first* (a spec is written for the current task), *spec-anchored* (the spec is kept and used to evolve the feature), and *spec-as-source* (humans edit only the spec and code is generated). She found every tool she reviewed was spec-first, and only some aim higher. — [Böckeler, Understanding SDD: Kiro, spec-kit, Tessl](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)
- Memory-bank equivalents: Kiro "steering" (product.md, tech.md, structure.md), spec-kit "constitution" (immutable principles), Tessl `.tessl/framework`, KNOWLEDGE.md, AGENTS.md. — [Böckeler](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)

**GitHub Spec Kit** (MIT, ~148.9k stars; v1.3.2 released 2026-11-06; 1.0 reached Aug 2026 per a third-party comparison)
- Full command order: `/speckit.constitution → specify → clarify → plan → checklist → tasks → analyze → implement → converge`. — [Spec Kit converge search results / Agentic SDD reference](https://github.github.com/spec-kit/reference/agentic-sdd.html); [repo](https://github.com/github/spec-kit)
- `/speckit.clarify` asks at most 5 questions per session. It scans a 10-category ambiguity taxonomy, including "Completion Signals" and "Constraints & Tradeoffs", and marks each category Clear/Partial/Missing. Answers are written into spec.md under `## Clarifications` → `### Session YYYY-MM-DD` as `- Q: … → A: …`, and the answer is also applied to the relevant spec section, replacing contradictory text. Its completion report has a coverage table with **Resolved / Deferred / Clear / Outstanding** statuses. Deferred covers items beyond the quota or implementation-level questions, and each comes with a rationale. — [clarify.md template](https://github.com/github/spec-kit/blob/main/templates/commands/clarify.md)
- `/speckit.analyze` is a read-only consistency check across spec.md, plan.md and tasks.md. It flags constitution violations and zero-coverage requirements as CRITICAL. — [DeepWiki checklist command](https://deepwiki.com/github/spec-kit/4.9-speckit.checklist-command); [dceoy speckit skills](https://deepwiki.com/dceoy/speckit-agent-skills/3.1-spec-kit-skills-(speckit-*))
- `/speckit.checklist` produces "unit tests for English". These validate how well the requirements are *written*, not whether code matches the spec, and the command must not pre-tick its own items. — [checklist.md template](https://github.com/github/spec-kit/blob/main/templates/commands/checklist.md)
- `/speckit.converge` shipped June 2026 (#3001; one blog says v0.11.2 on June 18). It reads the codebase against spec/plan/tasks and is append-only: it never edits code, and the only thing it writes is new tasks under a "Convergence" section of tasks.md. It reports either "✅ Converged" or the number of gaps found, and you loop implement→converge until it converges. The June 2026 newsletter calls it the core's direct answer to the drift-and-verification concern raised in nearly every review. — [Spec Kit June 2026 newsletter](https://github.com/github/spec-kit/blob/main/newsletters/2026-June.md); [Agentic SDD ref](https://github.github.com/spec-kit/reference/agentic-sdd.html)
- Third-party SpecKit Companion separates two jobs. *Converge* closes the gap between code and one feature's own docs. *Drift/sync* keeps long-lived capability specs honest. It also notes Spec Kit forbids running converge while implement is still running. — [speckit-companion PR #813](https://github.com/alfredoperez/speckit-companion/pull/813)
- The extension ecosystem as of Nov 2026: "Quality Gates (Enforcement Layer)", "DocGuard — CDD Enforcement", "GitHub Project Lifecycle Sync", "Pull Request Workflow", and bug-assess/fix/test extensions whose reports carry a `verified|partial|failed` verdict. v1.3.0 (2026-11-03) removed core `/speckit.taskstoissues` and moved it to a `github` extension. It also added hooks such as `preflight_setup()`. — [Spec Kit releases](https://github.com/github/spec-kit/releases); [repo README](https://github.com/github/spec-kit)
- Spec Kit is multi-agent in the "works with many coding agents" sense: it has integration keys for many agents. — [repo](https://github.com/github/spec-kit)
- A Nov 2025 issue: `/speckit.specify` creates a new branch and artifact every time, which suits first-time authoring better than refining a spec. — [Augment SDD guide](https://www.augmentcode.com/guides/what-is-spec-driven-development)

**AWS Kiro** (IDE, CLI and Web; spec docs updated 2026-10-15)
- Each spec is three files in `.kiro/specs/`: requirements.md (or bugfix.md), design.md and tasks.md. It offers Requirements-First and Design-First variants, plus a "Quick Spec" that writes all three files with no approval gates. — [Kiro specs docs](https://kiro.dev/docs/specs/)
- The task UI shows real-time in-progress/completed status. "Run all tasks" builds a dependency graph and runs independent tasks concurrently in *waves*, with waves run in sequence. Tasks trace to requirement numbers. — [Kiro specs docs](https://kiro.dev/docs/specs/); [Böckeler](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)
- Verification: property-based testing for "Correctness" (IDE only), and "Analyze Requirements" to find inconsistencies, ambiguities and gaps before design. Markdown Review lets you comment on passages and send the comments back to the agent. — [Kiro specs docs](https://kiro.dev/docs/specs/)
- Hooks (docs updated 2026-10-21) cover PostFileSave, PreToolUse/PostToolUse with matchers, and Stop, which is the only one with a `confirm` prompt. Actions are either a shell command (gets session JSON on stdin) or an agent prompt injection. Hooks live in `.kiro/hooks/` or `~/.kiro/hooks/`. — [Kiro hooks docs](https://kiro.dev/docs/hooks/)

**OpenSpec (Fission-AI)** (MIT, ~74.4k stars; Thoughtworks Radar *Trial* in Nov 2026)
- Current truth lives in `openspec/specs/`. Proposed changes live in `openspec/changes/<name>/` with proposal.md, design.md, tasks.md and delta specs (ADDED/MODIFIED/REMOVED/RENAMED requirements with WHEN/THEN scenarios). Archive merges the deltas into the main specs and moves the folder to `changes/archive/YYYY-MM-DD-<name>/`. — [OpenSpec concepts](https://github.com/Fission-AI/OpenSpec/blob/main/docs/concepts.md); [repo](https://github.com/Fission-AI/OpenSpec)
- `/opsx:verify` checks Completeness, Correctness and Coherence ("design decisions reflected in code"). It searches the codebase for evidence and grades findings CRITICAL/WARNING/SUGGESTION with a ready-to-archive verdict, but it **does not block archive**. `/opsx:archive` warns on incomplete tasks and does not block. `/opsx:sync` merges deltas without archiving, for long-running changes. — [OpenSpec commands.md](https://github.com/Fission-AI/OpenSpec/blob/main/docs/commands.md)
- `openspec validate --archived` confirms that every archived change has all its tasks.md boxes ticked. That is a checkbox audit, not an evidence audit. — [OpenSpec cli.md](https://github.com/Fission-AI/OpenSpec/blob/main/docs/cli.md) (via search summary)
- During `/opsx:apply` the agent flips each `- [ ]` to `- [x]` as it goes. Unticked tasks produce a warning, and `-y` continues past it. — [DEV OpenSpec tutorial 2026](https://dev.to/remybuilds/openspec-tutorial-2026-spec-driven-development-in-claude-code-step-by-step-1k4p)
- Recent releases: archive halts on blocking sync conditions, `validate` warns when delta text would be dropped, and `show --json --deltas-only` exists. — [OpenSpec releases](https://github.com/Fission-AI/OpenSpec/releases)
- Supports "30+ AI assistants via slash commands". There is no multi-agent orchestration. — [repo](https://github.com/Fission-AI/OpenSpec)

**BMAD Method** (MIT, ~55.2k stars; stable v6.12; v7 alpha with release ETA mid-Nov 2026)
- The repo page describes a planning → build → verify → "Learn and adjust" loop, "multiple-agent discussions", a separate "BMad Loop" module that runs epic-level retros, and an enterprise "Test Architect" add-on. — [BMAD repo](https://github.com/bmad-code-org/BMAD-METHOD)
- v4 (from a docs fork): the PO agent shards the PRD into epic files, and the SM agent turns each epic into story files (`1.1.user-registration.story.md`). The Dev agent may update **only** the Tasks/Subtasks checkboxes and the "Dev Agent Record" (model, debug log, completion notes, file list). It must not touch Status, Acceptance Criteria or QA Results. The QA agent writes gate YAML at `docs/qa/gates/{epic}.{story}-{slug}.yml` with status PASS|CONCERNS|FAIL|WAIVED, plus commands such as `*trace` and `*nfr`. — [BMAD user-guide fork](https://github.com/starktoony/BMAD-METHOD/blob/main/docs/user-guide.md) (third-party fork; not the official v6 docs)
- v6 reportedly merged the SM and QA roles into the Developer agent. It runs "validation checks and adversarial reviews" before sprint planning. Secondary sources disagree on whether sharding is still used. — [Augment BMAD guide](https://www.augmentcode.com/guides/bmad-method-ai-development); [Medium v6](https://medium.com/@hieutrantrung.it/from-token-hell-to-90-savings-how-bmad-v6-revolutionized-ai-assisted-development-09c175013085)

**Agent OS (Builder Methods)** (MIT; v3 released 2026-01-22)
- v3 removed about 70% of the codebase, including spec writing, task breakdown, orchestration and subagents. It keeps `discover-standards`, `index-standards`, `inject-standards`, `plan-product` and `shape-spec`, and leaves planning to the host's Plan Mode. The stated reason is that Claude Code's plan mode and better models "now handle much of the scaffolding". — [Agent OS v3 discussion #310](https://github.com/buildermethods/agent-os/discussions/310); [CodeMySpec review](https://codemyspec.com/blog/agent-os-review); [Casel on X](https://x.com/CasJam/status/2014375279733461043)
- Each spec folder is timestamped (`agent-os/specs/2026-01-15-1430-…/`) and holds shape.md (shaping *decisions* and context), standards.md, plan.md (Plan Mode output) and visuals/. — [DeepWiki Agent OS](https://deepwiki.com/buildermethods/agent-os/5-configuration-and-standards)

**Tessl**
- Launched a Spec Registry (open beta, 10k+ library specs) and a spec-as-source Framework (closed beta, Sept 2025). Generated files carry `// GENERATED FROM SPEC - DO NOT EDIT`. — [Tessl launch blog](https://tessl.io/blog/tessl-launches-spec-driven-framework-and-registry)
- The Framework apparently never reached GA. On 2026-01-29 the company repositioned around agent skills ("Skills are the new code") as an "Agent Enablement Platform". — [CodeMySpec Tessl review](https://codemyspec.com/blog/tessl-review); [spec-compare tessl.md](https://github.com/cameronsjo/spec-compare/blob/main/docs/tools/tessl.md)
- Böckeler saw Tessl generate different code from the same spec across runs. — [Böckeler](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)

**GSD ("Get Shit Done") → GSD Core** (MIT; original repo had ~64.4k stars by 2026-09-30 and is now archived in favor of `open-gsd/gsd-core`)
- This is the closest match to the user's "program with milestones" need. State lives in `.planning/`: PROJECT.md, REQUIREMENTS.md, ROADMAP.md (phases with status), STATE.md ("decisions, blockers, session memory"), per-phase CONTEXT.md (implementation decisions) and MILESTONES.md (completed milestone archive). The loop is discuss → plan → execute → verify → ship per phase. `/gsd-complete-milestone` archives the milestone and tags a release. `/gsd-resume-work` rebuilds context in a new session. — [GSD USER-GUIDE](https://github.com/gsd-build/get-shit-done/blob/main/docs/USER-GUIDE.md); [farasat.me](https://www.farasat.me/blog/get-shit-done-claude-code)
- **Drift tooling:** `gsd-tools.cjs state validate` detects drift between STATE.md and the filesystem, and `state sync` rebuilds STATE.md from disk. This is the only "derive status from evidence" mechanism found in this survey, though its evidence is the filesystem, not git or tests. — [GSD USER-GUIDE](https://github.com/gsd-build/get-shit-done/blob/main/docs/USER-GUIDE.md) (via search summary)
- `/gsd-verify-work [N]` is *manual acceptance testing*: it walks the human through what was built and spawns debug agents on failures. `/gsd-ship` opens a PR from verified phase work. — [farasat.me](https://www.farasat.me/blog/get-shit-done-claude-code); [codecentric deep dive](https://www.codecentric.de/en/knowledge-hub/blog/the-anatomy-of-claude-code-workflows-turning-slash-commands-into-an-ai-development-system)

**Traycer** (commercial)
- Flow: Phases (milestones) → Plan → Handoff to any agent → Verification. Verification checks that changes match the plan, "pass checks/tests, and introduce no regressions", and returns fix suggestions as UI comments. "YOLO mode" automates phase→verify→next phase, with configurable plan/verification/review agents, severity thresholds and a per-phase "Skip verification". — [Traycer blog](https://traycer.ai/blog/build-with-intent-ship-with-confidence); [Traycer YOLO docs](https://docs.traycer.ai/extension/tasks/yolo-mode)

**Augment Intent** (commercial Mac app; public beta 2026-02-10)
- Three tiers: a coordinator drafts a *living spec*, specialists run tasks in parallel **in separate git worktrees**, and a verifier checks results against the spec's success criteria before anything reaches a PR. It is bring-your-own-agent (Claude Code, Codex, OpenCode) with reduced capability. — [softwareherald](https://softwareherald.com/augment-intent-living-spec-as-infrastructure-for-parallel-agents/); [rywalker](https://rywalker.com/research/augment-intent)
- The living spec "updates to reflect what was built, including incorrect implementations". Augment says the verifier catches plan drift, not merge-boundary defects. — [aitooltier review](https://aitooltier.com/tools/augment-intent); [Augment Intent vs Claude Code](https://www.augmentcode.com/tools/intent-vs-claude-code) (vendor page)

**Claude Code plan mode / plan files**
- Plans are saved as frontmatter-less markdown with random names (e.g. `dreamy-orbiting-quokka.md`) in `~/.claude/plans/` and auto-cleaned after `cleanupPeriodDays` (default 30). The `plansDirectory` setting (reportedly v2.1.9+) can point them into the repo (`./docs/plans`). The plan is re-injected from disk after compaction. — [ThePromptShelf plan mode guide 2026](https://thepromptshelf.dev/blog/claude-code-plan-mode-complete-guide-2026/); [macmdviewer](https://macmdviewer.com/blog/claude-code-plan-mode)
- Practitioner advice: "Plan Mode saves your plan where only one agent can find it. Commit PLAN.md instead". This matters for parallel worktrees. — [DEV: commit PLAN.md](https://dev.to/gabrielanhaia/plan-mode-saves-your-plan-where-only-one-agent-can-find-it-commit-planmd-instead-4n72)

**Cursor plan mode**
- The agent researches, asks clarifying questions, and writes an editable markdown plan. Plans go to the home directory by default, and "Save to workspace" commits one into the repo. — [flaviocopes plan mode](https://flaviocopes.com/ai-glossary/plan-mode/)

### Inferences
- Artifacts are similar everywhere. The real differences are (a) whether a persistent "current truth" spec exists separately from change proposals (OpenSpec yes; Spec Kit only via extensions or "evolving specs" guides; Kiro, Agent OS and plan mode no), and (b) whether verification gates anything (Traycer and Intent route failures back; OpenSpec verify and Spec Kit converge are advisory).
- GSD's STATE.md/ROADMAP.md/MILESTONES.md plus `state validate` is the nearest existing equivalent to the user's "hub epic + truth" idea. Its drift check compares the doc against files on disk, not against git merge history or test results.
- In BMAD v4, the dev agent may tick task boxes but may not change Status or QA Results, and QA writes a separate gate file. That is the clearest case of keeping "claimed" apart from "ratified", and it maps directly onto ATTENTION (a human/auditor ratification queue).

### Gaps
- I did not research Taskmaster AI, obra/Superpowers, Cursor's plan to-do mechanics, or Kiro's steering inclusion modes in detail.
- No official BMAD v6 docs were fetched. The v6 role/gate structure comes from secondary sources that partly disagree.
- Kiro's license and pricing were not verified (presumed proprietary AWS product).
- The latest OpenSpec release version and date were not shown on the repo page.

## How is task completion marked, and is there drift detection between spec and code?

### Takeaway
Completion is almost always self-reported: the implementing agent ticks a markdown checkbox. The verification steps that exist (Spec Kit converge, OpenSpec verify, Kiro PBT, Traycer/Intent verifiers) are LLM-judged, mostly advisory, and scoped to one change. No tool found reconciles claimed status against git commits, merges or CI results across a program. Documented failures include agents ticking boxes for work that doesn't exist, and parsers that count unrecognized markers as done.

### Cited Findings
- Spec-kit checklists act as a "definition of done", but there is "no 100% guarantee that they will be respected" because the AI interprets them. — [Böckeler](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)
- Real-world checkbox theater: in lightspeedwp/.github issue #3523, a spec's tasks.md marks tasks complete whose deliverables do not exist on the branch. The missing items (hook scripts, validation libraries, audit reports) were found by checking paths on 2026-09-23. — [Issue #3523](https://github.com/lightspeedwp/.github/issues/3523)
- When a task is blocked on a human, the agent can only tick a box for undone work ("dishonest") or leave CI red. tasks.md has no "blocked on human approval" state. — [trending-local-llms #409](https://github.com/andyholst/trending-local-llms/issues/409)
- Proposal for tasks only the user may tick (`[user]`) and "evidence tasks". — [specwright #186](https://github.com/sh1ny/specwright/issues/186)
- OpenSpec parser bugs: `- [~]` tasks were counted as complete, so one repo with 42 done + 17 deferred showed "✓ Complete" (PR #1773). `1. [ ]` and `+ [ ]` tasks are invisible and archive reports "✓ Complete" (issue #1861). — [OpenSpec PR #1773](https://github.com/Fission-AI/OpenSpec/pull/1773); [OpenSpec #1861](https://github.com/Fission-AI/OpenSpec/issues/1861)
- Spec Kit `/speckit.converge` is the main spec↔code check in core, and it only appends tasks. One practitioner: "If people skip /converge, spec and code drift again. Tie acceptance criteria to tests so CI enforces the spec indirectly." — [Spec Kit June 2026 newsletter](https://github.com/github/spec-kit/blob/main/newsletters/2026-June.md); search summary of [vinitshahdeo substack](https://vinitshahdeo.substack.com/p/spec-driven-development-github-spec-kit)
- A third-party RAD_framework issue proposes a "post-merge convergence check: advisory plan↔code drift report for delivered features". This suggests demand for drift checks *after merge*, which core tools lack. — [RAD_framework #72](https://github.com/seanrreid/RAD_framework/issues/72)
- The tikalk Spec Kit fork adds a test gate, diff analysis and a 4-pillar assessment (Spec Compliance, Code Quality, Test Adequacy, Risk & Evidence) after converge. — [tikalk agentic-sdlc-spec-kit](https://github.com/tikalk/agentic-sdlc-spec-kit)
- codemyspec: Spec Kit, Kiro, Agent OS and others follow a "spec-first-then-drift" pattern, and sync in OpenSpec and Augment Intent is "aspirational and manual". — [codemyspec SDD 2026](https://codemyspec.com/blog/spec-driven-development)
- An HN commenter uses a task.md per work item, with the agent checking subtask gates and hooks printing the current open subtask so the agent "never deviates" (GuardRails). — [HN item 46961910](https://news.ycombinator.com/item?id=46961910)

### Inferences
- The failure modes the user sees (confident prose status, drifting child status) match the documented ones. Every tool lets the writer of the code also be the writer of its status.
- Design implication: keep claimed status (agent-written) and evidenced status (derived from commits/merges/test runs) as separate fields, and surface the difference. BMAD's dev-vs-QA field ownership and specwright's `[user]` tasks are partial precedents.
- A "blocked on human" task state is a recurring gap and maps to the ATTENTION queue.

### Gaps
- I found no tool that ties tasks to commit SHAs or merge events. If one exists, it was not visible in these searches.

## How are decisions and clarifications captured? Are milestones and exit criteria supported? Multi-agent?

### Takeaway
Decision capture is better than status tracking. Spec Kit `/clarify` writes dated Q→A sessions and a Resolved/Deferred/Outstanding table, and GSD and Agent OS keep decision files. All of them record a decision as accepted the moment it is written. None has a "proposed by agent, awaiting human ratification" state. Milestones exist in GSD (milestones/phases), Traycer (phases) and BMAD (epics, retros). Exit criteria are either acceptance criteria per requirement or human UAT. Parallel multi-agent execution appears in Kiro (waves), Intent (worktrees + verifier) and Traycer (any agent).

### Cited Findings
- Spec Kit clarify: `## Clarifications / ### Session YYYY-MM-DD / - Q: … → A: …`, with a Deferred status for items beyond quota (with rationale). — [clarify.md](https://github.com/github/spec-kit/blob/main/templates/commands/clarify.md)
- Spec Kit idea-assessment extension: `/speckit-assess-intake → research → define → shape → decide` writes to `.specify/assessments/<slug>/` with a `go | needs-clarification | kill` decision. — [Spec Kit repo](https://github.com/github/spec-kit)
- GSD: STATE.md holds "decisions, blockers, session memory", and per-phase CONTEXT.md holds implementation decisions. MILESTONES.md archives completed milestones. `/gsd-complete-milestone` tags the release. — [GSD USER-GUIDE](https://github.com/gsd-build/get-shit-done/blob/main/docs/USER-GUIDE.md)
- Agent OS shape.md records "shaping decisions and context" per spec. — [DeepWiki Agent OS](https://deepwiki.com/buildermethods/agent-os/5-configuration-and-standards)
- Traycer Phases are "manageable, logical chunks with milestones", each verified before the next. — [Traycer blog](https://traycer.ai/blog/build-with-intent-ship-with-confidence)
- BMAD QA gate status vocabulary PASS|CONCERNS|FAIL|**WAIVED** (v4). This is an explicit waiver state. — [BMAD user-guide fork](https://github.com/starktoony/BMAD-METHOD/blob/main/docs/user-guide.md)
- Kiro runs independent tasks concurrently in dependency-ordered waves. Intent runs specialists in separate worktrees, with a verifier before PR. — [Kiro specs](https://kiro.dev/docs/specs/); [softwareherald](https://softwareherald.com/augment-intent-living-spec-as-infrastructure-for-parallel-agents/)
- A DEV post (dated Nov 8, likely 2026) flags "Git Merge Hell" when many parallel feature specs merge delta specs into one main spec. — [DEV: SDD frameworks are a 2024 solution](https://dev.to/maximlogunov/why-spec-driven-development-frameworks-are-a-2024-solution-to-a-2026-problem-and-what-to-do-pi4)

### Inferences
- Copyable patterns: clarify's dated Q→A log with a Deferred bucket (feeds COVERAGE), Spec Kit's go/needs-clarification/kill verdict, BMAD's WAIVED gate state, and GSD's milestone archive + tag.
- Missing everywhere: a ratification workflow for decisions agents make *during implementation* (as opposed to clarifications the human answers up front), and a coverage view of scope nobody is working on.

### Gaps
- I couldn't confirm whether BMAD v6 keeps the QA gate YAML or its field-ownership rules.
- I found no tool with an independent "auditor" agent that runs on a schedule across the whole program. Intent's and Traycer's verifiers are per-task or per-phase.

## What do practitioners say about SDD at scale (stale specs, checkbox theater)?

### Takeaway
The critique is consistent from Oct 2025 to Nov 2026. The tools produce too much markdown to review and give a false sense of control. Specs go stale after shipping, and agents don't reliably follow their own specs or checklists. The field responded in two ways: lighter tools that lean on plan mode (Agent OS v3, Tessl's pivot), and delta-against-truth models (OpenSpec, promoted to Trial).

### Cited Findings
- Böckeler (2025-10-15): she would "rather review code than all these markdown files". Kiro turned a small bug into 4 user stories with 16 acceptance criteria ("sledgehammer to crack a nut"). Spec Kit generated so many files she never finished. Agents duplicated existing classes and over-applied constitution articles. She warns that spec-as-source may combine MDD's inflexibility with LLM non-determinism. — [Böckeler](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)
- Thoughtworks Radar Vol. 33 (Nov 2025): SDD in *Assess*. It warns about lengthy hard-to-review specs and a return to "heavy up-front specification and big-bang releases". — [Radar SDD technique](https://www.thoughtworks.com/radar/techniques/spec-driven-development); [Vol 33 PDF](https://www.thoughtworks.com/content/dam/thoughtworks/documents/radar/2025/11/tr_technology_radar_vol_33_en.pdf)
- Radar Vol. 34 (Apr 2026): Spec Kit and OpenSpec in Assess. Experienced engineers get the most value. — [Vol 34 PDF](https://www.thoughtworks.com/content/dam/thoughtworks/documents/radar/2026/04/tr_technology_radar_vol_34_en.pdf); [Radar Spec Kit](https://www.thoughtworks.com/radar/languages-and-frameworks/github-spec-kit)
- Radar Nov 2026: OpenSpec moved to *Trial* because "changes are deltas against a persistent source-of-truth spec". There is a new *Caution* entry, "AI-driven documentation sprawl", naming Spec Kit and BMAD. — [Radar OpenSpec](https://www.thoughtworks.com/en-us/radar/tools/openspec); [Radar techniques](https://www.thoughtworks.com/radar/techniques) (Nov 2026 details via search summary; not verified against the PDF)
- "The spec is excellent at time zero". Afterward, code gets patched and nobody updates the spec, because that work is unrewarded. The proposed fix is to make spec drift visible the way a failing test is. — [DEV: the stale spec is the part nobody fixed](https://dev.to/kunalsharda/spec-driven-development-with-ai-is-real-now-the-stale-spec-is-the-part-nobody-fixed-2001)
- "Stale traceability at machine speed is worse than none." After about 6 months you have "a second codebase with weaker tooling". Specs should describe the next change and mostly disappear after shipping. — [The Main Thread: SDD needs an exit strategy](https://www.the-main-thread.com/p/spec-driven-development-exit-strategy)
- Frisinger (tddbuddy, 2026-07-30): SDD is the "prose-spec drift trap, reborn". The test suite is the artifact that cannot drift silently. He cites Kent Beck on the assumption that nothing learned during implementation should change the spec. — [tddbuddy](https://www.tddbuddy.com/blog/spec-driven-is-the-drift-trap/)
- The OpenSpec HN front page (Sept 2026) top comment reportedly complained that every medium-or-larger change becomes many markdown docs to review. — [DEV OpenSpec tutorial](https://dev.to/remybuilds/openspec-tutorial-2026-spec-driven-development-in-claude-code-step-by-step-1k4p) (secondhand; HN thread URL not found)
- Augment's own guide warns that stale specs create false confidence. — [Augment guide](https://www.augmentcode.com/guides/what-is-spec-driven-development)
- Pro-SDD: an arXiv paper (Piskala, Jan 2026, 2602.00180) is cited for error reductions from refined specs versus loose prompts. — [DEV SDD in 2026 overview](https://dev.to/krlz/spec-driven-development-in-2026-what-it-is-the-tooling-and-how-teams-actually-use-it-2fk2) (secondary; paper not read)

### Inferences
- The critiques support the user's premise: prose status written by agents is the failure mode. Truth has to come from artifacts that can't drift silently (tests, git history), consistent with Frisinger and the "make drift visible like a failing test" argument.
- The documentation-sprawl Caution argues for a tool that *reads and derives* from existing artifacts instead of generating more markdown.

### Gaps
- I didn't locate the primary HN threads for Spec Kit or OpenSpec.
- The Nov 2026 Radar wording comes from a search summary, not the published blip text.

## Maturity, and verdicts (learn from / copy ideas / use directly)

### Takeaway
None of these tools covers TRUTH/ATTENTION/COVERAGE across a multi-week, multi-worktree program, so "use directly" doesn't fit the core need. Several are worth copying in specific places: OpenSpec's truth-vs-delta model, Spec Kit's clarify/converge mechanics, GSD's state-validate and milestones, BMAD's field ownership and gate states, and Traycer/Intent's verifier loop.

### Cited Findings (maturity)
| Tool | License | Stars / status | Last release (as found) |
|---|---|---|---|
| GitHub Spec Kit | MIT | ~148.9k ([repo](https://github.com/github/spec-kit)); ~140k in Oct 2026 per [felipefontoura](https://felipefontoura.com/articles/spec-driven-development-tools-compared/) | v1.3.2, 2026-11-06 ([releases](https://github.com/github/spec-kit/releases)) |
| OpenSpec | MIT | ~74.4k; Radar Trial Nov 2026 ([repo](https://github.com/Fission-AI/OpenSpec)) | active (not stated on page) |
| BMAD Method | MIT | ~55.2k; v6.12 stable, v7 alpha ([repo](https://github.com/bmad-code-org/BMAD-METHOD)) | v7 ETA mid-Nov 2026 |
| GSD / GSD Core | MIT | ~64.4k (original, archived 2026) ([farasat.me](https://www.farasat.me/blog/get-shit-done-claude-code)) | now `@opengsd/gsd-core` |
| Agent OS | MIT | v3 2026-01-22 ([discussion #310](https://github.com/buildermethods/agent-os/discussions/310)) | v3 |
| Kiro | proprietary (AWS; not verified) | docs updated Oct 2026 ([specs](https://kiro.dev/docs/specs/)) | n/a |
| Tessl | commercial | Framework closed beta, never GA; pivoted Jan 2026 ([CodeMySpec](https://codemyspec.com/blog/tessl-review)) | n/a |
| Augment Intent | commercial | public beta 2026-02-10 ([rywalker](https://rywalker.com/research/augment-intent)) | n/a |
| Traycer | commercial | active ([docs](https://docs.traycer.ai/extension/tasks/yolo-mode)) | n/a |

### Verdicts
- **GitHub Spec Kit — copy ideas.** Copy the clarify session log with its Resolved/Deferred/Outstanding table, the go/needs-clarification/kill verdict, and converge's append-only "gaps become tasks" loop. It is too feature-scoped and too heavy to run a program.
- **OpenSpec — copy ideas.** Its persistent truth (`specs/`) vs in-flight deltas (`changes/`) split, and verify's Completeness/Correctness/Coherence rubric, are the best model for "claimed vs actual". Its checkbox-based completion and non-blocking verify are exactly what to avoid.
- **AWS Kiro — learn from.** Wave-based parallel task execution and Stop/PostToolUse hooks with `confirm` are good mechanics, but it is IDE-bound and spec-first only.
- **BMAD Method — copy ideas.** Field ownership (dev agent may tick tasks but not Status or QA Results), a separate gate file, and the PASS/CONCERNS/FAIL/WAIVED vocabulary map directly to claimed-vs-ratified. The method itself is heavy and on the Radar's sprawl Caution list.
- **GSD / GSD Core — copy ideas (closest prior art).** ROADMAP/STATE/MILESTONES files, milestone completion with release tags, and `state validate`/`state sync` drift detection against disk are near the hub-epic model. Extend the evidence source from the filesystem to git and test results.
- **Agent OS v3 — learn from.** Its retreat to plan mode plus standards injection is a signal that durable per-feature specs don't pay off. shape.md for decisions is a small idea.
- **Tessl — learn from.** Spec-as-source stalled (non-determinism, never GA). Don't build on the premise that specs regenerate code.
- **Augment Intent — learn from.** Coordinator, worktree-isolated specialists and a verifier before PR is the right shape for parallel worktrees. But a living spec that updates to whatever was built (including bugs) is the drift anti-pattern. Commercial and closed.
- **Traycer — learn from.** Phases as milestones with a verification gate per phase, configurable severity, and skip-verification as an explicit (auditable) choice. Commercial.
- **Claude Code plan mode / plan files — use directly (as input).** Point `plansDirectory` into the repo so plans from every worktree are committed and can be ingested by the new tool as "intent" records. Plan mode itself does no tracking.
- **Cursor plans — learn from.** Same pattern ("Save to workspace"), with no tracking.

### Inferences
- The new tool's differentiator would be *evidence-derived status across many changes over time*, plus a *ratification queue for agent-made decisions*. No surveyed tool does either. The nearest pieces are GSD `state validate` (doc vs disk), Spec Kit converge and OpenSpec verify (LLM, per change), and BMAD's gate files (ratification, per story).

### Gaps
- Taskmaster AI, Superpowers and specwright were seen but not evaluated, and may hold more prior art on evidence tasks or user-only tasks.
