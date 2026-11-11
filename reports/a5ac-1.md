---
item: a5ac
"n": 1
passed: true
commit: 594a9c3c992fce8e6ad1d0989c04c07de22fb395
base: 66646b17eff935a78e07d3296bf5831a9c535e7e
at: "2026-11-11T01:42:09Z"
by: /Users/jeff/dev/kit
summary: 'mull serve now shows the repositories that aren''t on mull yet, as the prototype''s lower-left list. A new endpoint, GET /api/unmulled, looks one level down in the directories the registered repositories live in (today ~/dev) for git repositories with no mull branch: those holding old .mull/ data, listed as needing converting whatever their age, and others with a commit on any ref in the last 60 days. Linked worktrees and dot-directories are skipped. The scan is kept for five minutes and never writes or watches anything. The sheet index''s foot shows them as Needs converting and Not on mull yet (then any registered repo that can''t be read, then the model link), and All projects lists them in grouped rows under its table, each with what to do: an intake, or mull init. On this machine it finds anxiety, deadtree, music_tools, oaks, reprise, stejar and trove to convert. Documented in mull guide json.'
tour:
  - Run make build && ./mull serve and open http://localhost:6855/.
  - 'At the bottom of the sheet index: Needs converting lists your repos with old .mull/ data (anxiety, deadtree, music_tools, oaks, reprise, stejar, trove). Hover a name to see its path.'
  - On All projects, the last title-block field counts them, and under the projects table two grouped sections list each repo with its path, when it was last committed to, and what to do (an intake for old mull data, mull init for a repo never on mull).
  - Make a new git repo in ~/dev with a commit (git init, then a commit). Within five minutes (or at once after restarting mull serve) it appears under Not on mull yet.
  - curl localhost:6855/api/unmulled shows the same list as JSON, with the directories it looked in.
checks:
  tree: 0a3a03be0f4548cbc2721711b84416a9420eb19d
  commit: 594a9c3c992fce8e6ad1d0989c04c07de22fb395
  at: "2026-11-11T01:41:47Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 21725
changes:
  - layer: other
    files:
      - status: M
        path: mull/docs/serve/DESIGN.md
        added: 1
        removed: 1
      - status: M
        path: mull/internal/guide/json.md
        added: 1
        removed: 0
      - status: M
        path: mull/internal/serve/serve.go
        added: 2
        removed: 0
      - status: A
        path: mull/internal/serve/unmulled.go
        added: 174
        removed: 0
      - status: M
        path: mull/internal/serve/web/api.js
        added: 3
        removed: 0
      - status: M
        path: mull/internal/serve/web/app.js
        added: 17
        removed: 11
      - status: M
        path: mull/internal/serve/web/pages.js
        added: 31
        removed: 5
  - layer: tests
    files:
      - status: A
        path: mull/internal/serve/unmulled_test.go
        added: 94
        removed: 0
design: |-
  Server: internal/serve/unmulled.go, registered by Handler like the other endpoints. rootsOf takes the parent directory of each registered path (sorted, unique): no configuration, and it follows the repos wherever they are. scan reads each root one level down, takes directories with a .git entry (skipping dot-directories), and checks them concurrently. check runs git through internal/git: a linked worktree (MainWorktree differs) is skipped because its main worktree stands for it; a repo with refs/heads/mull is on mull, registered or not, and skipped; the newest commit on any ref comes from one for-each-ref --sort=-committerdate --count=1; a .mull/ directory makes it old_mull at any age, otherwise a commit within 60 days makes it not_on_mull, else it's left out. Results sort old mull first, then by name, case-insensitively. The scanner keeps the last result with its roots and time, and rescans when it's 5 minutes old or the roots differ, under a mutex so concurrent requests share one scan. Unreadable roots list nothing. The endpoint and its shape are in mull guide json.

  UI: api.unmulled is fetched with the project list (so every live event and every reconnect refreshes it, cheaply, from the cache). pages.notOnMull sorts what isn't on mull into convert, not yet, and broken (registered repos that can't be read and aren't already in the unmulled list), so a registered repo without a mull branch shows once. The rail foot renders the three groups and the model link; All projects' sixth field becomes Not on mull yet, and its table gains grouped rows (Needs converting, Not on mull yet) saying what to do.

  Tests: TestUnmulled builds a root holding a registered repo on mull, an unregistered one on mull, a recent repo, a dormant one, an old-mull one, a dot-directory, a plain directory and a linked worktree, and checks exactly the old-mull and recent repos come back, then that a new repo appears only once the scan is 5 minutes old. TestUnmulledEndpoint checks a missing root answers an empty list.

  Rejected: a configured list of scan directories (the human chose no configuration); watching the directories (mull serve shouldn't hold watches on repos it doesn't own); listing every git repo regardless of age (15 dormant repos would bury the ones that matter).

  Verified against this machine's ~/dev with a scratch registry: the seven old-mull repos, oaks-rust-parser correctly skipped as a worktree of oaks, a fresh scratch repo listed as Not on mull yet; the foot and All projects at 1440 light and 400 dark with no console errors.
decisions:
  - id: 2b80
    text: Registered repos with no mull branch appear only in the unmulled groups; the sheet index's foot shows Needs converting, Not on mull yet and Can't read (any other unreadable registered repo); All projects' last field counts the not-on-mull repos
    detail: One place per repo, no duplicates; the prototype's last field was Not on mull yet
  - id: bb29
    text: 'GET /api/unmulled lists the repos without a mull branch one level down in the registered repos'' parent directories: old .mull/ data always (old_mull), others only with a commit on any ref in the last 60 days (not_on_mull); linked worktrees and dot-directories skipped'
    detail: The shape the human agreed on 2026-11-11; a linked worktree's main worktree stands for it (oaks-rust-parser is a worktree of oaks)
  - id: f2fa
    text: mull serve keeps an unmulled scan for 5 minutes, or until the set of directories changes, and scans on the next request after that; the page fetches it with the project list
    detail: 'About 30 repos and a few git calls each: cheap enough on demand, and no watching of directories mull doesn''t own'
---

# Done report 1: Not on mull yet: list recently worked-on repos beside the registered ones that aren't on mull, and repos with old .mull/ data that need converting (a5ac)

**Status:** PASSED  
**Commit:** 594a9c3c992f  
**By:** /Users/jeff/dev/kit at 2026-11-11T01:42:09Z

## What was built

### Summary

mull serve now shows the repositories that aren't on mull yet, as the prototype's lower-left list. A new endpoint, GET /api/unmulled, looks one level down in the directories the registered repositories live in (today ~/dev) for git repositories with no mull branch: those holding old .mull/ data, listed as needing converting whatever their age, and others with a commit on any ref in the last 60 days. Linked worktrees and dot-directories are skipped. The scan is kept for five minutes and never writes or watches anything. The sheet index's foot shows them as Needs converting and Not on mull yet (then any registered repo that can't be read, then the model link), and All projects lists them in grouped rows under its table, each with what to do: an intake, or mull init. On this machine it finds anxiety, deadtree, music_tools, oaks, reprise, stejar and trove to convert. Documented in mull guide json.

### UI tour

1. Run make build && ./mull serve and open http://localhost:6855/.
2. At the bottom of the sheet index: Needs converting lists your repos with old .mull/ data (anxiety, deadtree, music_tools, oaks, reprise, stejar, trove). Hover a name to see its path.
3. On All projects, the last title-block field counts them, and under the projects table two grouped sections list each repo with its path, when it was last committed to, and what to do (an intake for old mull data, mull init for a repo never on mull).
4. Make a new git repo in ~/dev with a commit (git init, then a commit). Within five minutes (or at once after restarting mull serve) it appears under Not on mull yet.
5. curl localhost:6855/api/unmulled shows the same list as JSON, with the directories it looked in.

### Checks (run by mull)

- `make done-checks`: pass, 21.7s

## How it was built

### Design notes

Server: internal/serve/unmulled.go, registered by Handler like the other endpoints. rootsOf takes the parent directory of each registered path (sorted, unique): no configuration, and it follows the repos wherever they are. scan reads each root one level down, takes directories with a .git entry (skipping dot-directories), and checks them concurrently. check runs git through internal/git: a linked worktree (MainWorktree differs) is skipped because its main worktree stands for it; a repo with refs/heads/mull is on mull, registered or not, and skipped; the newest commit on any ref comes from one for-each-ref --sort=-committerdate --count=1; a .mull/ directory makes it old_mull at any age, otherwise a commit within 60 days makes it not_on_mull, else it's left out. Results sort old mull first, then by name, case-insensitively. The scanner keeps the last result with its roots and time, and rescans when it's 5 minutes old or the roots differ, under a mutex so concurrent requests share one scan. Unreadable roots list nothing. The endpoint and its shape are in mull guide json.

UI: api.unmulled is fetched with the project list (so every live event and every reconnect refreshes it, cheaply, from the cache). pages.notOnMull sorts what isn't on mull into convert, not yet, and broken (registered repos that can't be read and aren't already in the unmulled list), so a registered repo without a mull branch shows once. The rail foot renders the three groups and the model link; All projects' sixth field becomes Not on mull yet, and its table gains grouped rows (Needs converting, Not on mull yet) saying what to do.

Tests: TestUnmulled builds a root holding a registered repo on mull, an unregistered one on mull, a recent repo, a dormant one, an old-mull one, a dot-directory, a plain directory and a linked worktree, and checks exactly the old-mull and recent repos come back, then that a new repo appears only once the scan is 5 minutes old. TestUnmulledEndpoint checks a missing root answers an empty list.

Rejected: a configured list of scan directories (the human chose no configuration); watching the directories (mull serve shouldn't hold watches on repos it doesn't own); listing every git repo regardless of age (15 dormant repos would bury the ones that matter).

Verified against this machine's ~/dev with a scratch registry: the seven old-mull repos, oaks-rust-parser correctly skipped as a worktree of oaks, a fresh scratch repo listed as Not on mull yet; the foot and All projects at 1440 light and 400 dark with no console errors.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **2b80** Registered repos with no mull branch appear only in the unmulled groups; the sheet index's foot shows Needs converting, Not on mull yet and Can't read (any other unreadable registered repo); All projects' last field counts the not-on-mull repos
  Why: One place per repo, no duplicates; the prototype's last field was Not on mull yet
- **bb29** GET /api/unmulled lists the repos without a mull branch one level down in the registered repos' parent directories: old .mull/ data always (old_mull), others only with a commit on any ref in the last 60 days (not_on_mull); linked worktrees and dot-directories skipped
  Why: The shape the human agreed on 2026-11-11; a linked worktree's main worktree stands for it (oaks-rust-parser is a worktree of oaks)
- **f2fa** mull serve keeps an unmulled scan for 5 minutes, or until the set of directories changes, and scans on the next request after that; the page fetches it with the project list
  Why: About 30 repos and a few git calls each: cheap enough on demand, and no watching of directories mull doesn't own

### Changes by layer

**other**: 7 files, +229 −17
- M mull/docs/serve/DESIGN.md (+1 −1)
- M mull/internal/guide/json.md (+1 −0)
- M mull/internal/serve/serve.go (+2 −0)
- A mull/internal/serve/unmulled.go (+174 −0)
- M mull/internal/serve/web/api.js (+3 −0)
- M mull/internal/serve/web/app.js (+17 −11)
- M mull/internal/serve/web/pages.js (+31 −5)

**tests**: 1 files, +94 −0
- A mull/internal/serve/unmulled_test.go (+94 −0)
