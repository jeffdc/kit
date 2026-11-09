# taxogall, recast: plan · where we are · needs you

A test of the one-view shape using real data. Sources:

- gallformers matter 50ff as read earlier on 2026-11-09. It has since been modified (uncommitted) by another session.
- The other open matters.
- git on `taxogall` and the other branches and worktrees.

Nothing here was changed in gallformers. The "My guess" column is only a suggestion to speed up triage. You decide.

---

## The one view

```
TAXOGALL                                  2026-11-09 · taxogall 166 commits ahead of main
PLAN       1 Admin ◐   2 Merge ✓   3 Phase 2 + gaps ◐   4 Notes ·   5 Sources→prod copy ·   6 Cutover ·
IN FLIGHT  cb28 Add a taxon from iNat        6 commits since 11-06   ⚠ not in the plan
           c800 Guardrails                   landed 11-09; harness-neutral rework uncommitted
           8861 Plants kept twice            1 commit 11-09          ⚠ not in the plan
           b230 Review cleanup + audit       planned 11-09, no commits yet
           d3b0 Unified edit model           named "next" on 11-07; only its planning commit since
NEEDS YOU  21 decisions · 8 questions · 22 loose ends
```

**The first thing this view shows:** what's in flight has drifted from the plan.

- On 11-07, 50ff said the next steps were d3b0, then a full audit, then f14c, then step 4.
- Since then, work went to:
  - cb28, adding a taxon from iNat, which isn't in 50ff at all;
  - plant merges (8861);
  - the guardrails (c800);
  - the review cleanup (b230).
- Some of that may be the audit under another name. Nothing records the switch, and nobody decided it.

---

## Plan

The steps are from 50ff's "Order to release". "Status" is what 50ff itself claims; nothing is linked to commits, so none of it is verified.

| # | Part | Status | Remaining, per 50ff |
|---|---|---|---|
| 1 | How the admin works: curation (ec2c), design system (bc0d) | ◐ | bc0d "Handoff" items; ec2c; f14c ergonomics |
| 2 | Merge the two branches | ✓ 11-06 (`e078704f`) | none |
| 3 | Phase 2 (f923) and remaining gaps | ◐ | f923 tasks 1–10 claimed built; then d3b0 → full audit → f14c |
| 4 | Gallformers notes (20e5) | · | Not started; 3 open questions (below) |
| 5 | Process Sources into a production copy (f465, with Adam) | · | Cost levers, missed-names display, reference answers, where the producer runs, curator onboarding |
| 6 | Cut over and retire the old system (b5bd M5–M6) | · | Rehearsals, timing, replay log, rollback |
| after | Post-release cleanup (b5bd M7) | · | Drop old tables, docs, runbooks |

**Done, per 50ff:**
- 294c import contract
- 9449 back-end phase 1
- 803b public pages
- 86e7 gall names
- a856, 6611, d213, f14c admin and review. 50ff also lists f14c as still open under part 1, so it contradicts itself.

**Done, per mull status:** 87a1 fact inspector, c8c3 back-end fixes.

**Not part of the plan:**
- **b016** (test-suite alignment) is marked active, with no commits since 11-06.
- **About 45 other open matters** are outside the plan. 50ff names 15 of them as "Later". The rest have no home (see loose ends).

---

## Needs you: decisions agents made (21)

These are 50ff's "For Jeff's review (the 2026-11-06/07 build)", which before that existed "only in a session summary". Each one is already built.

| # | Decision | Area | My guess |
|---|---|---|---|
| D1 | A non-plant's first common name raises a taxonomy question | Phase 2 | Accept |
| D2 | Taxa created by an import are accepted as created (name and placement) | Phase 2 | Look |
| D3 | A paper agreeing with the tree raises no question but stays "awaiting" | Phase 2 | Accept |
| D4 | A split may keep A among its daughters | Phase 2 | Accept |
| D5 | Split questions suggest a daughter from shared Hosts (never bulk-accepted) | Phase 2 | Accept |
| D6 | A retracted split or lump leaves moved Galls where they are | Phase 2 | Look |
| D7 | A revalidated name outranks the old synonymy even after a later rename ("one-line change if you meant only while current") | Phase 2 | **Decide** |
| D8 | A Gallformers taxonomy batch moves the tree without acceptance | Phase 2 | Look |
| D9 | Lumping B declines real Sources' statements about B | Phase 2 | **Decide** |
| D10 | Cutover carry-over records no curator on its acceptances | Phase 2 | Accept |
| D11 | Applying from the tray leaves Edit mode unless every item is held | Curation | Accept |
| D12 | Any admin can undo anyone's decision | Curation | **Decide** (policy) |
| D13 | Undo refuses decisions recorded before tree-edit recording (splits, lumps, vocabulary trait terms) | Curation | Accept |
| D14 | A correction to a taxonomic statement awaits a curator unless "Accept it now" | Curation | Accept |
| D15 | Creating a citation is logged as a Source edit | Curation | Accept |
| D16 | Imports and migrations don't refresh the dashboard live | Curation | Accept |
| D17 | WCVP fills only blank plant authorships; ambiguous names are listed | WCVP | Accept |
| D18 | Replaying a WCVP link change reads WCVP to fill authorship | WCVP | Accept |
| D19 | Host-range "select all" is in the table header | Look | Accept |
| D20 | Image-manager copy-mode "select all" is in the header | Look | Accept |
| D21 | The review options' Recommended chip goes after the tags | Look | Accept |

My guesses: 15 "Accept", 6 that need a real look. If that's right, a batch accept clears most of this queue in one action.

## Needs you: questions (8)

| # | Question | Source | Blocks |
|---|---|---|---|
| Q1 | `file_drop` can't be reached by keyboard. Fixing it changes the upload hooks: approve? | 50ff review | Nothing |
| Q2 | Did split undos ever run on preview? If yes, a compatibility path is needed for pre-`90a55ef0` undos | 50ff review | Safe undo on preview data |
| Q3 | Is the order still d3b0 → audit → f14c, given what's actually in flight? | This recast | Part 3 |
| Q4 | Scope of the curated taxonomy | 50ff Open / 8567 | Part 3/4 |
| Q5 | Traits that vary by Host | 50ff Open / 8567 | Part 3 |
| Q6 | Do undescribed notes' "observed in…" become occurrence Facts? | 20e5 | Part 4 |
| Q7 | Judgement notes: by LLM or by curator? | 20e5 | Part 4 |
| Q8 | Does a note's taxonomic position become a Gallformers taxonomic Fact? | 20e5 | Part 4 |

## Loose ends (22)

| # | Loose end | Source | My guess |
|---|---|---|---|
| L1 | **Prod:** Sources admin list crashes on species-source events | 2c8c #1 | Do (fix branch off main) |
| L2 | **Prod:** e2e broken on main; fixed only on taxogall | 2c8c #2 | Park until the merge to main |
| L3 | `fix-content-image-upload-events`: 3 commits "awaiting push / PR" since 10-27 | 2c8c #3, git | Do (push and open the PR) |
| L4 | `put_flash(:warning)` never shows | 2c8c inventory | Do (b230 / design system) |
| L5 | Component flash with auto-dismiss is never cleared on the server | 2c8c inventory | Do |
| L6 | `.input` error text probably not red (DaisyUI leftovers); not checked in a browser | 2c8c inventory | Do |
| L7 | Place names probably rendered in italics | 2c8c inventory | Do |
| L8 | Vocabulary page has two h1s | 2c8c inventory | Do |
| L9 | Canonical URL emitted as `<meta>`; search engines ignore it (prod) | 2c8c | Do |
| L10 | Source page: retraction marks and "Connected Galls" disagree | 2c8c | Decide |
| L11 | Unsupported Facts are public on the Source page | 2c8c | Decide |
| L12 | Host page: genus Galls missing on species, variety breadcrumb, `?source=` link ignored | 2c8c | Do |
| L13 | Dev compile fails on a Boundary reference in `fact_edit_mock.ex` | c800 | Do |
| L14 | Superseded by d3b0: remove `cited_source_id`/`cites_as_written`, rework `BasisPicker`, merge the two Gallformers Sources, move ID notes out of Source 58 | 50ff review | Do (fold into d3b0) |
| L15 | Branch `worktree-agent-aaf6…`: 4 commits not on taxogall, last 10-27 | git | Look, then drop |
| L16 | Branch `quercus-leafminer-key`: 2 commits, last 09-08 | git | Look |
| L17 | Worktree `python-ingestion-pipeline`, last commit 05-15 | git | Drop? |
| L18 | Worktree `plant-merges-check`, detached HEAD | git | Drop after 8861 |
| L19 | 17 uncommitted changes in the main checkout (c800 rework, 50ff, 6611, cb28, CLAUDE.md…) | git | Do (lands with the c800 rework) |
| L20 | "Planned" for months, no plan part: 220d rate limiting (Feb), 0fc6 user management (Mar), 41cf reverse proxy (Apr), b9e5 WCVP backfill (Apr) | mull | Park or drop |
| L21 | About 30 raw matters from Feb–Oct with no plan part and not in 50ff's "Later" list | mull | Batch: park or drop |
| L22 | The docket lists 85c0, f465 and b016 and is stale; status drift: 7 items 50ff calls done are still raw or planned | mull | Drop (the new tool replaces both) |

---

## What the exercise says about the shape

**It fits.**
- Every section of 50ff landed somewhere, and nothing needed a new category.
- The inbox holds about 51 items. Roughly 30 look clearable in seconds: batch accepts, drops, "do" items that fold into existing work. That leaves about 20 that actually need your thinking.
- Today those 20 are spread over two matters and a session summary.

**It strains in four places.**
1. **"In flight" is the weakest signal.** Matter IDs appear in only some commit messages, so whether something is "in flight" is inferred from mentions. cb28 shows clearly only because of its branch name. A light linking habit (an ID in the branch name or the commit) is still needed. Without it, this section stays partly guesswork.
2. **Work that isn't in the plan needs its own flag.** cb28, 8861, c800 and b016 are real work outside the six steps. The view should raise "not in the plan" as a question for you: fold it into a part, or accept it as a side task. Otherwise the plan quietly stops describing what's happening.
3. **"Done" here is 50ff's word.** The ✓ and ◐ marks are claims. The view needs at least a "claimed" versus "seen in git" distinction, or it inherits 50ff's problem.
4. **The backlog outside the plan is big:** about 45 matters. A one-off triage (park or drop, mostly in batch) is a prerequisite. After that, loose ends should stay small if agents capture them as they go.

**Questions for you:**
- Does this view, filled in for real, give you "where are we"?
- Is the inbox at about 50 items the right size to work through once, or should the old backlog (L20–L22) be handled separately?
