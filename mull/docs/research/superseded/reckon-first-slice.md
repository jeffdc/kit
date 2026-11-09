# reckon: first slice (draft for review)

*"reckon" is a working name, borrowed from navigation: dead reckoning estimates where you are from claimed speed and heading; a fix tells you where you actually are. This doc lives beside the prior-art report until the tool has its own directory.*

## Direction from discussion (2026-11-09)

This shapes the next redraft. The slice below predates it.

- **The work alternates between being independent of any one model and depending on whichever model is in use.** Model quality has drifted before and will again, so the process must hold up when the model has a bad week:
  - Don't rely on the model's judgment for anything a check can verify.
  - Keep units small enough that a weaker model still succeeds.
  - Make failure show up fast.
  - Keep the process identical across models, with an optional canary task to measure drift rather than feel it.
- **Jeff needs to be more involved, not less.** His judgment and domain knowledge are the main defense. The tool should make his involvement effective, not reduce it.
- **Failure pattern to design against:**
  - Design slides into building in the same session.
  - The scope is never something Jeff understands, so the agent does what it wants.
  - The result is code he doesn't understand, guardrails ignored, slop, and features that may not work.
- **So the "before" half matters as much as the evidence half.** That means three things:
  - **Work orders.** A bounded, approved piece of a matter with scope, out-of-scope, deletions, calls, acceptance criteria and a size budget. Jeff must be able to explain it back before building starts.
  - **Checkpoints.** Each step ends with a walkthrough, decisions and evidence, and Jeff approves before the next step. Steps are sized to roughly 10–15 minutes of human understanding.
  - **Evidence.** The reconciler below, checking each work order against its own scope and budget.
- **Limits are soft, but impossible to miss.**
  - There is no hard brake. When a work order passes its budget, that becomes the top item everywhere:
    - first line of `reckon brief`;
    - the `commit-msg` hook output on every commit linked to it (agents see this whatever harness they're in);
    - `reckon status`.
  - The AGENTS.md protocol tells agents what to do: when over budget, finish the current step, write its checkpoint, and start nothing new.
- **Every Claude-specific piece gets made generic.** That includes the guardrails already built in gallformers (c800 tasks 4–6: `.claude/hooks`, `.claude/agents`). Gates move to git hooks, CI and plain scripts. The reviewer is any model, given a packet built from the diff.

## The problem, in one line

Across a long multi-agent program, every status you can read was written by an agent, so you can't tell where the program really stands. reckon shows what's claimed and what git proves, and makes the gap between them the main thing you see.

## Ground rules

1. **No dependence on any LLM vendor or harness.**
   - reckon's only interfaces are **git, files and a CLI**.
   - An agent learns the protocol from a short plain-text block in `AGENTS.md`, so any harness that can run a shell command can take part.
   - Enforcement goes in **git hooks**, which are the same under every harness.
   - Harness-specific integrations (Claude hooks, Codex config…) are optional add-ons that call the same CLI. Nothing requires them.
   - **Test for every feature:** it works with a human typing the commands.
2. **reckon never calls an LLM.**
   - When a job needs judgment (attributing old commits, a skeptical audit), reckon **exports a task bundle**: a prompt plus evidence, as markdown.
   - It then **imports a structured answer** as JSON lines.
   - Any model, in any harness, can do the middle step, including you pasting the bundle into a chat.
3. **Claims and evidence are separate.**
   - *Claims* are anything an agent or person wrote: mull status, hub documents, commit messages saying "done".
   - *Evidence* is what git produced: commits, their reachability from a branch, merges. Later, check commands' exit codes are added.
   - The two are stored separately and never merged into one "done".
4. **Read-mostly.**
   - reckon writes only its own store. It never edits `.mull/`, code or branches.
   - Truth is derived from git, so no agent cooperation is needed beyond following the linking convention.
5. **One store per repo, shared by every worktree.**
   - The store lives under `$(git rev-parse --git-common-dir)/reckon/`.
   - Every worktree resolves to the same place, so there are no per-worktree copies to drift apart.

## What slice 1 delivers

**Goal:** run `reckon brief` on gallformers and get an honest answer to "where is taxogall?", with every claim marked as backed by evidence, contradicted by it, or not checked.

### 1. The program file (human-owned)

A small YAML file that gives the program its shape. You write it once with any agent, using 50ff's "Order to release", and change it rarely.

```yaml
program: taxogall
integration: taxogall        # where work lands
release: main                # where it ships
claims: mull                 # read matter status from .mull/
milestones:
  - id: m1
    title: How the admin works
    items: [ec2c, f14c, bc0d]
    exit:
      - landed: items        # every item's linked commits are on `integration`
      - human: "Curation flow signed off"
  - id: m2
    title: Phase 2 and remaining gaps
    items: [f923, 803b, d3b0]
    exit:
      - landed: items
      - check: "mix test"    # exit code recorded against the HEAD sha
  # m3 notes (20e5), m4 sources into a prod copy (f465), m5 cutover (b5bd)…
```

There are three kinds of exit criterion:

- `landed` is computed from git.
- `check` is a shell command, and reckon records its exit code and the commit it ran against.
- `human` is a sign-off only you can record, with `reckon signoff`.

A milestone is reached only when all of its criteria are met. Agents can suggest changes to the program file, but only you edit it.

### 2. The linking convention

Evidence needs a link from commits to matters. Gallformers today has almost none: 28 of 179 commits since Oct 20 mention a matter ID in passing, there are no trailers, and no live branch carries an ID.

- **Commit trailer:** `Matter: f923`. Several are allowed, and `Matter: none` marks deliberate housekeeping.
- **Branch prefix:** `f923-tree-facts` links every commit unique to that branch.
- **`reckon init` prints a 6-line `AGENTS.md` block** that teaches agents the convention. You paste it in.
- **`reckon hook install`** adds a `commit-msg` hook that **warns** (doesn't block) when a commit on a program branch has no `Matter:` trailer. It asks before installing.

### 3. Backfilling history (first use of the bundle pattern)

- `reckon attribute export` writes a bundle containing every unlinked commit on `integration` since the program started (message, files touched, date), an index of matters (ID, title, first lines of body), and instructions.
- Any LLM returns lines like `{"commit": "…", "matter": "f923", "confidence": "high|low", "why": "…"}`.
- `reckon attribute import` stores these as **inferred** links, a weaker kind than trailers or branch names. The brief always marks them as inferred.
- `reckon attribute accept <id|--all-high>` promotes them after you've looked.

### 4. Reconciling claims with evidence (deterministic, no LLM)

**Evidence state** for each matter:

| State | Meaning |
|---|---|
| `released` | All linked commits are reachable from `release` |
| `landed` | All linked commits are reachable from `integration` |
| `in-flight` | Some linked commits are only on unmerged branches or worktrees |
| `none` | No linked commits |

Each state also records the date of the matter's last linked commit.

**Claim state** is the matter's mull status. reckon reads it **from `integration` via `git show` and from every worktree**, and flags any matter whose copies disagree.

**Verdicts**, worst first:

| Verdict | When |
|---|---|
| **UNVERIFIED** | Claimed done, evidence `none` |
| **NOT LANDED** | Claimed done, evidence `in-flight` |
| **UNDER-CLAIMED** | Evidence `landed`, claim still raw/refined/planned (the 50ff pattern) |
| **STALE** | Claimed active or planned and in a current milestone, with no linked commit in N days (default 7) |
| **DIVERGED** | The mull copies in different worktrees disagree |
| **OK** | Claim and evidence agree |

`landed` means the code is on the branch, not that it works. A milestone's `check` and `human` criteria cover that, and the brief never says more than the evidence supports.

### 5. The brief

`reckon brief [--since-last] [--json]` produces markdown in a fixed order: problems first, progress last.

1. **Milestones.** Exit criteria met or unmet, each with its evidence: commit SHA, check result and date, or sign-off.
2. **Drift.** UNVERIFIED and NOT LANDED first, then UNDER-CLAIMED, STALE and DIVERGED.
3. **Unattributed work.** Commits on `integration` with no link, as a count plus the most recent ones.
4. **Loose ends.** Unmerged branches and worktrees with their age and commits ahead, and worktrees with uncommitted changes.
5. **Since you last looked.** Matters whose evidence changed, landed commits, and new drift since `reckon seen`.

An illustrative sample, not real output:

```
taxogall · m2 of 6 · 2026-11-09
m1 How the admin works ........ landed 3/3 · human sign-off missing
m2 Phase 2 and remaining gaps . landed 1/3 · mix test ✗ @61289a21 (11-09)

DRIFT
  UNDER-CLAIMED 9449 Back-end schema       status planned · 31 commits landed (last 11-02)
  NOT LANDED    87a1 Fact inspector        status done · 4 commits only on fact-inspector
  STALE         a856 Step 10 minimal admin planned · no commits since 10-30
UNATTRIBUTED  97 commits on taxogall have no link (run: reckon attribute export)
LOOSE ENDS    python-ingestion-pipeline: 112 days, 40 ahead · worktree dirty
```

### 6. The CLI for slice 1

| Command | Does |
|---|---|
| `reckon init` | Creates the store and a starter program file, and prints the AGENTS.md block |
| `reckon hook install\|uninstall` | Adds or removes the warn-only `commit-msg` hook |
| `reckon status [--json]` | Per-matter claim, evidence and verdict table |
| `reckon brief [--since-last] [--json]` | The briefing |
| `reckon seen` | Moves your "last looked" marker to now |
| `reckon check [milestone]` | Runs `check` criteria and records exit code, SHA and time |
| `reckon signoff <milestone> <criterion>` | Records a human sign-off |
| `reckon attribute export\|import\|accept` | Backfills commit links via any LLM |

Every command takes `--json`, so the later GUI and any agent read the same output.

## Not in slice 1 (and when)

| Slice | Adds |
|---|---|
| **2: Attention** | `reckon decide` and `reckon ask` for agents (protocol in AGENTS.md), and a queue to work through: accept, edit, respond, ignore. Decisions start as *executed, awaiting ratification*. A bundle sweep pulls decisions buried in prose hubs, like 50ff's "For Jeff's review". |
| **3: Coverage and auditor** | Scope with no claim and no activity, deferred items past their review date, and TODOs added with no matter. `reckon audit export` gives an evidence-only bundle with a skeptical-auditor prompt that never sees the workers' summaries; `audit import` stores the result next to the brief. |
| **4: Cockpit GUI** | A Mac app on top of `--json`: brief first, with links out to the runner of your choice. Native vs web is still open. |
| **5: Portfolio** | All repos, using the registry idea from the mull GUI plan. |
| Later | CI results as evidence (`gh`/forge APIs), test-tampering checks, git notes so links survive squash merges. |

Slice 1 also leaves out: harness add-ons, extracting claims from prose (mull status is the only claim source for now), and any change to mull.

## Done when (acceptance on gallformers)

1. `program.yml` exists for taxogall, with 50ff's six release steps as milestones.
2. After backfill, **≥90% of commits** on `taxogall` since program start are linked, explicitly or by accepted inference.
3. `reckon status` reproduces the known mismatch: every item 50ff calls done gets a verdict backed by evidence, or is flagged UNVERIFIED.
4. `reckon brief` runs in under 2 s and fits on one screen for the summary sections.
5. **Works under every harness:** the same session of commands gives identical output when run by you by hand, by Claude Code and by Codex. No code path checks which harness is running.
6. A week of agent commits under the AGENTS.md block plus the warn-only hook shows ≥80% of new commits carrying a `Matter:` trailer. If not, slice 2 needs a stronger enforcement design before anything else.

## Build notes

- **Go, single binary,** as for mull and watchmen. It's a new kit tool with no shared code, so it has its own small reader for mull's frontmatter, and the format is documented in mull's README.
- **git through the `git` CLI** (`rev-list`, `merge-base --is-ancestor`, `show`, `worktree list --porcelain`, `interpret-trailers`), not a Go git library. This matches what users see and handles worktrees correctly.
- **Store layout:** `<common-dir>/reckon/{program.yml, links.jsonl, inferred.jsonl, checks.jsonl, signoffs.jsonl, cursor}`. Append-only files make history auditable. `reckon export` gives a backup.

## Open questions for review

1. **Name.** "reckon" or something else?
2. **Where `program.yml` lives.**
   - The shared git common dir (above): shared, but **not versioned or pushed**.
   - Or committed on the integration branch: versioned, but per-branch.
   - Or a dedicated `refs/reckon/*` ref: both, at the cost of complexity.
   - It's the one file you own and care about.
3. **Is mull the long-term claim source?** Or does reckon end up holding program intent and mull goes back to being an idea notebook?
4. **`Matter:` as the trailer key.** It ties the convention to mull's vocabulary. `Work:` or `Ref:` would be neutral if other trackers ever count as claim sources.
5. ~~**Warn-only hook.**~~ *Answered: no hard brake. Limits warn, loudly and everywhere; see "Direction" above.*
7. **Work order vs matter.** Is a work order a new record type in reckon, a mull matter with extra fields, or a child matter?
8. **Checkpoint record.** What must a checkpoint contain for Jeff to understand a step in 10–15 minutes? Where does approval get recorded?
6. **Milestones from 50ff.** Are its six release steps the right milestones, or is that list itself stale?
