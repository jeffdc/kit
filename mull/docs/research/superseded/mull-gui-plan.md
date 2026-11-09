# mull GUI: discovery and plan (draft)

Status: draft for discussion. Native vs web is deliberately undecided; this plan is built so that decision comes last, with evidence.

## Decisions so far (2026-11-09)

- **The core need is an overview.** First, which projects exist. Then, within each project, its overall shape and status: what's open, in progress, planned, and so on. The TUI doesn't give you either.
- **Reading matters is secondary.** Reading in the tool would be nice, but opening an external editor/viewer is acceptable.
- **All projects, with a switcher**, not one project per window.
- **Read first; light actions later.**
- **Mac only for now.** No hosting or sync.
- **A remote briefing is also wanted.** When you look at agent sessions remotely, you want to ask a skill for a briefing that the agent shows in its own window. This is a second consumer of the same overview data, not part of the GUI.
- **Remove sessions.** The feature never took off (2 log files ever, both in gallformers). It goes before any GUI work, so nothing new is built around it. See Phase 0b.

## 1. What mull is, as a product

Mull is a store of *matters* for solo projects, written mostly by an LLM and read and steered by a human.

- **Matter**: markdown file + YAML frontmatter in `.mull/matters/`. Status (`raw → refined → planned → (active) → done | dropped`), tags, effort, epic, `docs` (paths to plan docs), links (`relates`, `blocks`/`needs`, `parent`), arbitrary extra keys, body.
- **Docket**: ordered list of matter IDs with optional notes (`docket.yml`).
- **Epics**: free-form string labels.
- **Sessions**: dated work logs tied to matters (`.mull/sessions/`).
- **Archive**: done/dropped files moved to `.mull/archive/`.
- **Interface**: CLI with JSON in/out; Claude Code drives it via the skill or `mull prime --context` hooks. Data is per-repo and version controlled.

The CLI's own framing: "the primary interface is conversational." The human's job is mostly to **read, review, decide, and steer**, not to type metadata.

## 2. How it's actually used (survey of ~/dev, 2026-11-09)

13 real projects have `.mull/` (plus worktree copies). Highlights:

| Project | Matters | Archived | Avg body lines | Links | Epics | Docket | Last update |
|---|---|---|---|---|---|---|---|
| gallformers | 70 | 114 | 110 | 49 files | 12 | 3 | 2026-11-09 |
| oaks | 48 | – | 24 | 20 | 3 | 1 | 2026-11-02 |
| stejar | 40 | – | 238 | 39 | 3 | 0 | 2026-11-06 |
| music_tools | 19 | – | 49 | 16 | 7 | 0 | 2026-10-18 |
| reprise | 18 | – | 14 | 0 | 6 | 5 | 2026-04-11 |
| trove | 17 | – | 178 | 15 | 1 | 2 | 2026-05-06 |
| anxiety | 13 | – | 118 | 0 | 0 | 0 | 2026-03-23 |
| kit/mull, forage, others | 1–9 each | | | | | | |

What this says:

1. **Matters are design documents, not tickets.** Active projects average 100–240 lines per matter; the largest stejar matter is 1,224 lines. Reading is the dominant human activity.
2. **Matters reference each other constantly.** 146 inline "matter abcd" references in bodies, structured links in ~140 files, `parent` used in 13 (e.g. stejar's design doc 3235 with its build steps as children). Navigating between matters is the second job.
3. **The docket is lightly used and docket notes are never used** (0 across all projects). **Sessions are almost unused** (2 files total, both gallformers).
4. **Backlogs are raw-heavy** (gallformers: 51 of 70 raw). Grooming could be a job, but isn't happening today.
5. **Work spans many repos at once.** stejar, gallformers and oaks were all touched in the last week.
6. **Worktrees carry diverged copies of `.mull/`** (gallformers has three worktrees with different matter, archive and docket counts). Any multi-project view has to pick a canonical copy.
7. **Claude writes while you're looking.** Changes land from another process, often mid-session.

## 3. The TUI today

`mull-tui` is a separate binary (~1.5k lines, Bubble Tea) that imports `internal/storage` directly. It was last touched 2026-04.

- **Views**: Matters (filter open/closed/all/single status/epic, title search, sort title/created/updated/status), Docket (ordered), Detail (metadata + Glamour-rendered body in a scroll viewport).
- **Actions**: copy a status-aware prompt to the clipboard (`p`), toggle docket (`d`), cycle status (`S`), done/drop (`D`/`X`), open the file in the default app (`O`), light/dark (`T`).
- **Live reload** via fsnotify with a 100ms debounce.
- **Scope**: the single `.mull/` in the cwd.

### Why it probably doesn't stick (hypotheses to confirm)

- **It only works in one project at a time and from a terminal in that repo**, which is where Claude already is. So it competes with asking Claude.
- **Reading a 200+ line design doc one line at a time** (j/k) in a viewport, with no outline, is worse than opening the file.
- **Links are bare IDs you can't follow**; there are no backlinks, and inline "matter 3235" references are plain text.
- **Its actions are the low-value ones** (cycle status, toggle docket). The high-value jobs (read, review what changed, see structure, overview) aren't served.
- **Nothing tells you what Claude changed** since you last looked.

## 4. Candidate jobs for a GUI

Ranked by my guess at value. Section 7 asks you to confirm or re-rank them.

| # | Job | What it needs |
|---|---|---|
| A | **Read a matter well** | Rendered markdown, outline/TOC for long bodies, clickable refs (frontmatter links *and* inline "matter abcd"), backlinks, open `docs` paths, readable typography |
| B | **Overview across projects** | Project switcher; per-project counts by status; recently updated; active/planned; docket head |
| C | **See what changed** | "Updated since you last looked" markers; per-matter diff (git or snapshot); live updates while Claude works |
| D | **See structure** | Parent tree (design → steps), needs/blocks graph with blocked items highlighted, epic grouping |
| E | **Groom** | Bulk status changes, drop, drag-to-reorder the docket, raw-backlog triage view |
| F | **Hand off to Claude** | Copy prompt (exists); maybe "start a Claude Code session on this matter" |
| G | **Capture** | Quick add from anywhere (global hotkey) |
| H | **Edit bodies** | Probably *not*: it conflicts with Claude's writes. "Open in editor" may be enough |

My original hypothesis was that A + B + C were the core. **Your answers re-rank them:**

1. **B: portfolio overview.** Every project, with enough signal to see where things stand: counts by status, what's in progress, last activity.
2. **D, reframed as "project shape".** Inside one project: matters grouped by lifecycle (open / in progress / planned / done), with epics and parent trees as the grouping that shows the shape. The dependency graph is optional.
3. **A, light version.** A basic reader, plus "open in editor/viewer" as a fallback. TOC and backlinks are nice-to-haves.
4. **E/F later.** Status changes, docket and copy prompt come after v1.
5. **C, G and H are out for now.** C (what changed) could return if the overview makes you want it.

## 5. Architecture constraints (true for native or web)

1. **One source of truth for writes.** Bidirectional links, docket cleanup on done/drop, and frontmatter formatting live in `internal/storage`. A GUI must never re-implement them. Its writes go through that code, either linked directly (Go) or via the `mull` CLI's JSON.
2. **Reads can be richer than the CLI today.** The GUI needs computed views the CLI doesn't produce: backlinks, resolved inline refs, parent trees, "blocked" status. Put these in mull (Go, tested) so every front end and Claude can use them.
3. **A project registry.** Options:
   - (a) The CLI records each project's root in `~/.config/mull/projects.yml` whenever it runs. This is automatic and covers everything Claude touches.
   - (b) Scan `~/dev` for `.mull/`.
   - (c) Manual add only.

   Recommend (a) plus manual add/remove, excluding `.worktrees/` by default.
4. **Live updates.** Watch each open project, as the TUI already does.
5. **"Since you last looked" state** is per-user UI state, not project data. It lives in the GUI's own storage, never in `.mull/`.
6. **Inline ref detection.** Match `matter [0-9a-f]{4}` and bare 4-hex tokens only when they resolve to a known ID, so hashes and colors aren't linked by mistake.
7. **Start read-only.** v1 is a reader. That removes write-conflict risk with Claude and most of the complexity.

## 6. Plan

### Phase 0: settle the jobs (done 2026-11-09)

See "Decisions so far" at the top and the re-ranked list in section 4.

### Phase 0b: remove sessions

- **Code:**
  - Delete `cmd/session.go`, `internal/storage/session.go` and its test, and `internal/model/session.go`.
  - Remove `sessionsDir` from `Store`. Today `storage.New` creates an empty `.mull/sessions/` in every project on every run.
- **LLM-facing text:**
  - Drop the session lines from `mull prime --context` (`cmd/prime.go`).
  - Drop the "Sessions" section from `skills/SKILL.md`.
  - Otherwise Claude keeps trying to call the removed commands.
- **Existing data:**
  - Leave the files alone; mull simply stops reading them. That covers the 2 gallformers logs (and their worktree copies) and the empty `sessions/` dirs everywhere.
  - Optionally, `mull doctor --fix` removes an *empty* `.mull/sessions/`. It never touches a non-empty one.
  - Before any cleanup, you decide whether the gallformers logs get appended to their matters or deleted.
- **mull's own backlog:** drop matter `fdad` ("Session logs for mull", still raw).
- **Docs:** remove sessions from section 1 of this plan's data model once it's done.

### Phase 1: the overview data layer in mull (shared by the GUI and the briefing)

All of this is Go in mull, tested with `t.TempDir()`, and independent of the GUI platform.

- **Project registry.** `mull projects list|add|rm`, stored in `~/.config/mull/projects.yml`. Every `mull` invocation records its project root, so anything Claude touches shows up. `.worktrees/` paths are excluded (or folded into their main checkout).
- **`mull overview --json`, the portfolio.** For each project: name, path, counts by status, in-progress and planned matters (ID + title), docket head, epics with counts, last-updated date, and whether `.mull/` has uncommitted changes.
- **`mull overview <project> --json`, the project shape.** Matters grouped by lifecycle bucket, with epic and parent tree, and blocked flags from `needs`. Bodies are omitted; the reader fetches them on demand via `mull show`.
- **Make `active` official as "in progress".** You want to see what's in progress, and `active` already means that in the model, the TUI and real data (gallformers, stejar, music_tools). Update the skill, the README lifecycle and `mull schema` to match.
- **Lifecycle buckets defined once, in Go.** For example: Open = raw + refined, Planned = planned, In progress = active, Closed = done + dropped. The GUI, the briefing and `prime` then agree.

### Phase 1b: `mull brief` and the briefing skill (ships before any GUI)

- `mull brief [--project <name>]` renders the same overview as compact markdown: a portfolio table, then per-project in-progress / planned / notable-raw items. Agent windows can display it as-is.
- A small skill (or a section in the existing mull skill): "brief me" → run `mull brief` and show the output verbatim, adding only a sentence or two of commentary if asked. It works from any remote agent session that has `mull` on PATH and can read the repos.
- This gives you the overview **this week**, in text, and checks whether the overview data is the right data before a GUI is built on it.

### Phase 2: read-only GUI prototype

- **Screens:** portfolio (all projects) → project shape (lifecycle columns or grouped list, epic/parent grouping) → matter (rendered body + metadata, with "Open in editor" / "Reveal in Finder").
- **Live reload** when any registered project's `.mull/` changes.
- **Build it the fastest way.** That's likely `mull serve` (localhost only) serving one embedded page and the Phase 1 JSON, because it reuses `internal/storage` with no IPC. This tests the *screens*, not the platform.
- **Use it for ~2 weeks.** Note when you open it and what's missing (a mull matter is a fine place for that log).

### Phase 3: platform decision, with evidence

Mac only settles out hosting and sync. What's left to decide:

| If Phase 2 shows you want… | Leans |
|---|---|
| A real app: Dock icon, menu-bar summary, global hotkey, always-open window | Native SwiftUI (reading Phase 1 JSON via the CLI) or Tauri |
| Something you open now and then, cheapest to maintain, Go as the only codebase | Keep `mull serve` + web UI; optionally wrap it (Wails) |
| Rich markdown and graph rendering | Web tech (SwiftUI would embed a WebView for the body anyway) |

You already know Tauri from stejar and PWAs from forage, so either path is familiar.

### Phase 4: build v1 for real

- The Phase 2 screens on the chosen platform.
- Light actions, all through mull's storage: status change, done/drop, docket add/remove/reorder, copy prompt.
- Reader extras if you miss them: TOC for long matters, clickable "matter abcd" refs, backlinks.

### Phase 5: retire the TUI

Remove `cmd/mull-tui`, `internal/tui`, the Bubble Tea/Glamour/fsnotify deps (if the GUI doesn't reuse fsnotify), and the `build-tui` target. Update `CLAUDE.md`, `README.md` and the Makefile.

## 7. Open questions

Answered: when you'd use it, scope, read vs act, devices (see "Decisions so far").

Still open:

1. **Worktrees.** Is the main checkout's `.mull/` always canonical? Are the worktree copies expected to merge back? This decides whether the registry ignores worktrees or shows them as variants.
2. **Lifecycle buckets.** Is Open = raw + refined, Planned, In progress = active, Closed = done + dropped the shape you want to see? Or should raw sit apart as an "inbox"?
3. **The docket** is barely used. Should the overview show it (which might revive it), or leave it out? (Sessions: decided, removed.)
4. **What counts as a project's "shape"** beyond status: epics, the parent tree, or both? What should a project look like when it has neither (reprise, anxiety)?
5. **The remote briefing.** Is everything you'd ask for remotely on this Mac (repos under `~/dev`), or does the briefing need to work where the repos are cloned somewhere else?
6. **Granularity: tickets or plans?** What level should a matter sit at: an intent/design doc, an implementation plan, or a ticket? This decides what the overview shows: kanban columns of tasks, or what's decided, what's open and what's waiting on you.
