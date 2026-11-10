---
item: 3deb
"n": 1
passed: true
commit: 9ab6d86856c5560014d9df1238ccd39b6d92bd72
base: 5f1df6b2006a11f622f68c6c9ebf484deeb3d996
at: "2026-11-10T23:02:24Z"
by: /Users/jeff/dev/kit/.claude/worktrees/agent-aaa30f2ae60d58a47
summary: mull serve now knows what changed since you last looked. It keeps, per machine in ~/.config/mull/seen.json (never on the mull branch), the mull branch commit last marked seen for each project; GET /api/projects/{id}/revision returns the revision schedule (tip and last-seen revision numbers, and every plan, plan doc, phase, phase doc, item and inbox entry that differs, with before/after for stages and other short fields), POST /api/projects/{id}/seen marks a revision seen (JSON only; cross-origin posts refused), and the project list carries each project's revision number and changed count for the Revision column. The comparison is in internal/mull (Revision, RevisionAt), over decoded records, so migrations and undone edits are not noise. A project's first read records its tip as the baseline.
tour:
  - curl -s http://localhost:6855/api/projects/nope/revision   - 404 not_found
checks:
  tree: 3eb67b9b70a6e5efd7af57f296879092d3c7d7f3
  commit: 9ab6d86856c5560014d9df1238ccd39b6d92bd72
  at: "2026-11-10T23:02:19Z"
  by: /Users/jeff/dev/kit/.claude/worktrees/agent-aaa30f2ae60d58a47
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 5050
changes:
  - layer: other
    files:
      - status: M
        path: mull/cmd/serve.go
        added: 2
        removed: 1
      - status: M
        path: mull/internal/guide/json.md
        added: 7
        removed: 1
      - status: A
        path: mull/internal/mull/revision.go
        added: 505
        removed: 0
      - status: A
        path: mull/internal/seen/seen.go
        added: 142
        removed: 0
      - status: A
        path: mull/internal/serve/resolve.go
        added: 21
        removed: 0
      - status: A
        path: mull/internal/serve/revision.go
        added: 170
        removed: 0
      - status: M
        path: mull/internal/serve/serve.go
        added: 7
        removed: 2
      - status: M
        path: mull/internal/store/store.go
        added: 10
        removed: 0
  - layer: tests
    files:
      - status: A
        path: mull/internal/mull/revision_test.go
        added: 196
        removed: 0
      - status: A
        path: mull/internal/seen/seen_test.go
        added: 104
        removed: 0
      - status: A
        path: mull/internal/serve/revision_test.go
        added: 246
        removed: 0
      - status: M
        path: mull/internal/store/store_test.go
        added: 24
        removed: 0
design: |-
  Shape: three layers, each with one job. internal/mull/revision.go holds the logic (Revision(since), RevisionAt(commit)): it loads the records at the last-looked commit and at the tip through a new store.ViewAt (the existing loadCommit, exported as a view), decodes both with the same load() every command uses, and compares them record by record. internal/seen is the per-machine state file. internal/serve/revision.go is thin: resolve the ID, read the look, call mull, write JSON; it is registered from Handler with one line.

  Why compare decoded snapshots rather than walk the commits: the question is "what is different now from what I saw", so a change undone before you look is no change, and a migration across your look shows only what it changed in meaning (general retired, items unplanned, needs converted) rather than every record it rewrote in the new format. Updated times are ignored for the same reason (decision 4ce3 keeps format-only rewrites' times anyway). The cost is two snapshot loads: about 250ms on kit's real branch for 5 or 83 changes (checked read-only against kit, not in the test suite). One log over since..tip gives each change's time, so the schedule is newest first.

  Change shape: type/id/title/plan/phase/kind/state plus fields with from/to for short fields only. That is what the prototype's "What changed" column needs (committed to in-review, started in <worktree>, question settled) without duplicating notes or docs, which the view endpoints serve. Docs are their own records (plan_doc, phase_doc), as they are their own files and the prototype clouds them separately. Config, reports and checks are not listed: a finished item shows as its stage change and its review entry.

  Revision number: commits on the mull branch up to the commit (rev-list --count). It rises with every write, as the prototype's Rev 14 does, and needs nothing stored.

  Baseline: a project never seen (or whose stored commit the repo doesn't have, e.g. re-cloned) gets its tip recorded as the baseline the first time serve reads it, from the revision endpoint or the project list. Without it, nothing would ever be clouded, since the prototype disables Mark revision seen when nothing changed. Clouding everything on first look was the other option; it is noise. seen.Baseline only writes if the look is still the one read, so a concurrent Mark is never overwritten.

  State file: ~/.config/mull/seen.json, {repos: {path: {commit, at}}}, keyed by the registered main-worktree path like the registry. JSON because only mull writes it. Every write is a temp file in the same directory, fsynced and renamed over it, under an in-process mutex; two processes can lose a write (last wins), never tear the file. A corrupt file is an error and is never overwritten, so one bad byte doesn't drop every project's look.

  POST /seen takes an optional {commit}: the revision the page showed. Marking that, not the tip, keeps a write that lands between render and click clouded. It must be the tip or an ancestor (invalid_argument otherwise). It returns the new revision so the UI can redraw at once.

  Cross-origin: decision 923d's Host check stops reading through DNS rebinding but not a cross-site form POST. Writes now need Content-Type application/json (a form can't send it without a CORS preflight, and serve grants none) and are refused when Origin isn't the server's own or Sec-Fetch-Site is anything but same-origin or none. curl with the JSON header still works (the tour). Tested for foreign origin, another port, null origin, cross-site and same-site fetches, form and text/plain bodies, no content type and a foreign Host, each leaving the look untouched.

  Project list: serve.Project gains revision {number, changes, seen_at}, computed in the same goroutine as the summary. It is not in mull.Summary, which stays free of per-machine state (and keeps 5b48's edits to Summary apart from mine). Errors: unknown ID 404, bad commit 400, registered-but-unreadable 409 with its mull code.

  Docs: json.md's mull serve section describes the state file, the baseline, Revision and Change, the write rules and both endpoints; it no longer says serve "only reads" (it never writes mull data). Captured: bug e244 (Summary.last_change is not UTC), idea cd46 (name the doc sections that changed, as the prototype does).
decisions:
  - id: 06a9
    text: 'The first time mull serve reads a project (its revision or the project list), or when its last look names a commit the repository doesn''t have, it records the tip as the baseline with the current time: nothing is clouded until the mull branch moves after that'
    detail: 'Alternatives: everything new on first look (all records clouded: noise), or no baseline until the first Mark seen (but the button is disabled when nothing changed, so clouds would never start). A GET writing local state is accepted for this one case; the baseline never overwrites a look someone marked meanwhile'
  - id: 516f
    text: 'mull serve keeps the last look in ~/.config/mull/seen.json ($XDG_CONFIG_HOME honoured): {repos: {<main worktree path>: {commit, at}}}, in a new package internal/seen, replaced atomically (temp file in the same dir, then rename) under an in-process mutex'
    detail: JSON because only mull writes it (the registry is plain text so the human can edit it); beside the registry so all per-machine state is in one dir; keyed by the registered path like the registry. Concurrent writers in two processes can lose one write (last wins), never tear the file; a corrupt file is an error, never overwritten
  - id: c431
    text: 'A revision''s number is the count of commits on the mull branch up to it (git rev-list --count): Rev N grows with every write'
    detail: Matches the prototype's Rev 14 rising as the branch moves; derived from git, so nothing to store and the same on every machine with the same branch
  - id: b70a
    text: 'What changed is the records that differ, as decoded, between the state at the last-looked commit and at the tip (mull.Revision, over store.ViewAt), not a walk of every commit: a change of format alone (mull migrate) or one undone before you look is no change, the updated time is ignored, docs are their own records (plan_doc, phase_doc), and config, reports and checks are not listed'
    detail: Two snapshot loads (about 250ms on kit) and one log call for each change's time. Comparing decoded records makes a migration across the look show only its real meaning changes (general retired, items unplanned, needs converted)
  - id: dd5f
    text: 'A Change is {type, id, title, plan?, phase?, kind?, change: added|changed|removed, state?, fields?: [{field, from?, to?}], at}: from/to only for short fields (state, title, plan, phase, claim worktree, settled action, needs), never for text (notes, summary, entry text, detail, docs); at is the newest commit since the look that touched it, and changes are newest first'
    detail: Enough for the revision schedule's what-changed column (committed to in-review, a question settled decide) without shipping text the UI reads from the view endpoints anyway
  - id: 4caa
    text: GET /api/projects/{id}/revision is {id, tip, since, baseline?, changes, seen_at}; POST /api/projects/{id}/seen takes an optional {commit} (the revision the page showed; the tip if absent), which must be the tip or an ancestor of it, and returns the same shape
    detail: Marking the shown commit rather than the tip means a write landing between rendering and clicking stays clouded. Marking an earlier commit is allowed (it re-clouds), since it is only the human's own local state
  - id: 55fe
    text: POST requires Content-Type application/json (415 not_allowed otherwise) and refuses an Origin other than the server's own or a Sec-Fetch-Site other than same-origin or none (403 not_allowed); curl with the JSON header works
    detail: 'A cross-site page can only send a JSON POST after a CORS preflight, which mull serve never grants; the Origin and Sec-Fetch-Site checks are a second guard for browsers that send them. No CSRF token: a read-first UI has nothing to issue it on'
  - id: b77d
    text: 'GET /api/projects gives each readable project revision: {number, changes, seen_at}, a field on serve.Project rather than in mull.Summary; a registered but unreadable project answers the revision endpoint with 409 and its code (not_a_repo, not_initialized, unsupported_format)'
    detail: Summary stays free of per-machine state; the all-projects table's Revision column needs Rev N and the changed count. 409 because the project exists but its state prevents the read, unlike 404 for an unknown ID
---

# Done report 1: Last looked and revisions: what changed since you last looked (3deb)

**Status:** PASSED  
**Commit:** 9ab6d86856c5  
**By:** /Users/jeff/dev/kit/.claude/worktrees/agent-aaa30f2ae60d58a47 at 2026-11-10T23:02:24Z

## What was built

### Summary

mull serve now knows what changed since you last looked. It keeps, per machine in ~/.config/mull/seen.json (never on the mull branch), the mull branch commit last marked seen for each project; GET /api/projects/{id}/revision returns the revision schedule (tip and last-seen revision numbers, and every plan, plan doc, phase, phase doc, item and inbox entry that differs, with before/after for stages and other short fields), POST /api/projects/{id}/seen marks a revision seen (JSON only; cross-origin posts refused), and the project list carries each project's revision number and changed count for the Revision column. The comparison is in internal/mull (Revision, RevisionAt), over decoded records, so migrations and undone edits are not noise. A project's first read records its tip as the baseline.

### UI tour

1. curl -s http://localhost:6855/api/projects/nope/revision   - 404 not_found

### Checks (run by mull)

- `make done-checks`: pass, 5.0s

## How it was built

### Design notes

Shape: three layers, each with one job. internal/mull/revision.go holds the logic (Revision(since), RevisionAt(commit)): it loads the records at the last-looked commit and at the tip through a new store.ViewAt (the existing loadCommit, exported as a view), decodes both with the same load() every command uses, and compares them record by record. internal/seen is the per-machine state file. internal/serve/revision.go is thin: resolve the ID, read the look, call mull, write JSON; it is registered from Handler with one line.

Why compare decoded snapshots rather than walk the commits: the question is "what is different now from what I saw", so a change undone before you look is no change, and a migration across your look shows only what it changed in meaning (general retired, items unplanned, needs converted) rather than every record it rewrote in the new format. Updated times are ignored for the same reason (decision 4ce3 keeps format-only rewrites' times anyway). The cost is two snapshot loads: about 250ms on kit's real branch for 5 or 83 changes (checked read-only against kit, not in the test suite). One log over since..tip gives each change's time, so the schedule is newest first.

Change shape: type/id/title/plan/phase/kind/state plus fields with from/to for short fields only. That is what the prototype's "What changed" column needs (committed to in-review, started in <worktree>, question settled) without duplicating notes or docs, which the view endpoints serve. Docs are their own records (plan_doc, phase_doc), as they are their own files and the prototype clouds them separately. Config, reports and checks are not listed: a finished item shows as its stage change and its review entry.

Revision number: commits on the mull branch up to the commit (rev-list --count). It rises with every write, as the prototype's Rev 14 does, and needs nothing stored.

Baseline: a project never seen (or whose stored commit the repo doesn't have, e.g. re-cloned) gets its tip recorded as the baseline the first time serve reads it, from the revision endpoint or the project list. Without it, nothing would ever be clouded, since the prototype disables Mark revision seen when nothing changed. Clouding everything on first look was the other option; it is noise. seen.Baseline only writes if the look is still the one read, so a concurrent Mark is never overwritten.

State file: ~/.config/mull/seen.json, {repos: {path: {commit, at}}}, keyed by the registered main-worktree path like the registry. JSON because only mull writes it. Every write is a temp file in the same directory, fsynced and renamed over it, under an in-process mutex; two processes can lose a write (last wins), never tear the file. A corrupt file is an error and is never overwritten, so one bad byte doesn't drop every project's look.

POST /seen takes an optional {commit}: the revision the page showed. Marking that, not the tip, keeps a write that lands between render and click clouded. It must be the tip or an ancestor (invalid_argument otherwise). It returns the new revision so the UI can redraw at once.

Cross-origin: decision 923d's Host check stops reading through DNS rebinding but not a cross-site form POST. Writes now need Content-Type application/json (a form can't send it without a CORS preflight, and serve grants none) and are refused when Origin isn't the server's own or Sec-Fetch-Site is anything but same-origin or none. curl with the JSON header still works (the tour). Tested for foreign origin, another port, null origin, cross-site and same-site fetches, form and text/plain bodies, no content type and a foreign Host, each leaving the look untouched.

Project list: serve.Project gains revision {number, changes, seen_at}, computed in the same goroutine as the summary. It is not in mull.Summary, which stays free of per-machine state (and keeps 5b48's edits to Summary apart from mine). Errors: unknown ID 404, bad commit 400, registered-but-unreadable 409 with its mull code.

Docs: json.md's mull serve section describes the state file, the baseline, Revision and Change, the write rules and both endpoints; it no longer says serve "only reads" (it never writes mull data). Captured: bug e244 (Summary.last_change is not UTC), idea cd46 (name the doc sections that changed, as the prototype does).

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **06a9** The first time mull serve reads a project (its revision or the project list), or when its last look names a commit the repository doesn't have, it records the tip as the baseline with the current time: nothing is clouded until the mull branch moves after that
  Why: Alternatives: everything new on first look (all records clouded: noise), or no baseline until the first Mark seen (but the button is disabled when nothing changed, so clouds would never start). A GET writing local state is accepted for this one case; the baseline never overwrites a look someone marked meanwhile
- **516f** mull serve keeps the last look in ~/.config/mull/seen.json ($XDG_CONFIG_HOME honoured): {repos: {<main worktree path>: {commit, at}}}, in a new package internal/seen, replaced atomically (temp file in the same dir, then rename) under an in-process mutex
  Why: JSON because only mull writes it (the registry is plain text so the human can edit it); beside the registry so all per-machine state is in one dir; keyed by the registered path like the registry. Concurrent writers in two processes can lose one write (last wins), never tear the file; a corrupt file is an error, never overwritten
- **c431** A revision's number is the count of commits on the mull branch up to it (git rev-list --count): Rev N grows with every write
  Why: Matches the prototype's Rev 14 rising as the branch moves; derived from git, so nothing to store and the same on every machine with the same branch
- **b70a** What changed is the records that differ, as decoded, between the state at the last-looked commit and at the tip (mull.Revision, over store.ViewAt), not a walk of every commit: a change of format alone (mull migrate) or one undone before you look is no change, the updated time is ignored, docs are their own records (plan_doc, phase_doc), and config, reports and checks are not listed
  Why: Two snapshot loads (about 250ms on kit) and one log call for each change's time. Comparing decoded records makes a migration across the look show only its real meaning changes (general retired, items unplanned, needs converted)
- **dd5f** A Change is {type, id, title, plan?, phase?, kind?, change: added|changed|removed, state?, fields?: [{field, from?, to?}], at}: from/to only for short fields (state, title, plan, phase, claim worktree, settled action, needs), never for text (notes, summary, entry text, detail, docs); at is the newest commit since the look that touched it, and changes are newest first
  Why: Enough for the revision schedule's what-changed column (committed to in-review, a question settled decide) without shipping text the UI reads from the view endpoints anyway
- **4caa** GET /api/projects/{id}/revision is {id, tip, since, baseline?, changes, seen_at}; POST /api/projects/{id}/seen takes an optional {commit} (the revision the page showed; the tip if absent), which must be the tip or an ancestor of it, and returns the same shape
  Why: Marking the shown commit rather than the tip means a write landing between rendering and clicking stays clouded. Marking an earlier commit is allowed (it re-clouds), since it is only the human's own local state
- **55fe** POST requires Content-Type application/json (415 not_allowed otherwise) and refuses an Origin other than the server's own or a Sec-Fetch-Site other than same-origin or none (403 not_allowed); curl with the JSON header works
  Why: A cross-site page can only send a JSON POST after a CORS preflight, which mull serve never grants; the Origin and Sec-Fetch-Site checks are a second guard for browsers that send them. No CSRF token: a read-first UI has nothing to issue it on
- **b77d** GET /api/projects gives each readable project revision: {number, changes, seen_at}, a field on serve.Project rather than in mull.Summary; a registered but unreadable project answers the revision endpoint with 409 and its code (not_a_repo, not_initialized, unsupported_format)
  Why: Summary stays free of per-machine state; the all-projects table's Revision column needs Rev N and the changed count. 409 because the project exists but its state prevents the read, unlike 404 for an unknown ID

### Changes by layer

**other**: 8 files, +864 −4
- M mull/cmd/serve.go (+2 −1)
- M mull/internal/guide/json.md (+7 −1)
- A mull/internal/mull/revision.go (+505 −0)
- A mull/internal/seen/seen.go (+142 −0)
- A mull/internal/serve/resolve.go (+21 −0)
- A mull/internal/serve/revision.go (+170 −0)
- M mull/internal/serve/serve.go (+7 −2)
- M mull/internal/store/store.go (+10 −0)

**tests**: 4 files, +570 −0
- A mull/internal/mull/revision_test.go (+196 −0)
- A mull/internal/seen/seen_test.go (+104 −0)
- A mull/internal/serve/revision_test.go (+246 −0)
- M mull/internal/store/store_test.go (+24 −0)
