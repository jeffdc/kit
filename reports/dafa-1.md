---
item: dafa
"n": 1
passed: true
commit: 8e7e3b94ad4b4e0306f5fe981122e0bcc05e211e
base: 3cbfb013a1640974ccfc0f5be9423f3d74de327c
at: "2026-11-11T14:44:35Z"
by: /Users/jeff/dev/kit
summary: 'make bench measures mull''s reads. It seeds a large repository (5000 items, 10935 inbox entries, 27403 mull commits, 4161 commits on main) in 45s and times every serve page in-process, over HTTP through this tree''s handler, and against a running server (-url), with git calls counted. Baseline on the large seed: load 237ms, every page 460-650ms, revision 1.1s at 10 commits back and 4.8s at 10000, project list 5.4s. On kit, the installed server is 3.5-4x slower than the same handler run in the foreground (fa47). Later mull-data phases are judged by these numbers.'
tour:
  - 'In mull/, run: make bench BENCH_FLAGS="-n 3 -url http://127.0.0.1:6855"'
  - The first run seeds .dev/bench/large (about 45s); later runs reuse it.
  - 'See two tables, kit and large: a row per page with in-process, http and live medians, the cold first call, bytes, and git calls.'
  - make bench BENCH_SIZE=small for a quick run; BENCH_FLAGS=-json for JSON.
checks:
  tree: 196891978bddd3b94d4d70d9b23242ecf858a6b6
  commit: 8e7e3b94ad4b4e0306f5fe981122e0bcc05e211e
  at: "2026-11-11T14:43:07Z"
  by: /Users/jeff/dev/kit
  passed: true
  results:
    - command: make done-checks
      exit: 0
      duration_ms: 87988
changes:
  - layer: other
    files:
      - status: M
        path: mull/CLAUDE.md
        added: 4
        removed: 0
      - status: M
        path: mull/Makefile
        added: 10
        removed: 1
      - status: A
        path: mull/internal/bench/bench.go
        added: 354
        removed: 0
      - status: A
        path: mull/internal/bench/seed.go
        added: 552
        removed: 0
      - status: M
        path: mull/internal/git/git.go
        added: 9
        removed: 0
      - status: A
        path: mull/scripts/bench/main.go
        added: 183
        removed: 0
  - layer: tests
    files:
      - status: A
        path: mull/internal/bench/bench_test.go
        added: 128
        removed: 0
design: |-
  Two halves, in internal/bench, with a thin command in scripts/bench and make bench on top.

  Seeding (seed.go). A generator lays out events in time: plans created, doc'd, agreed with phases chained by needs, closed; planning-session decisions; items that are captured (some triaged, some dropped) or added, committed, started with a claim, given decisions and sometimes a question, finished with a report and a review entry carrying the decisions, and accepted (settling the review and its decisions). Each item's work lands on main as a commit with a Mull trailer, so the board's git links (and done-not-in-git) behave as in a real repo. Fates are weighted so done plans are ~90% done and open plans carry every stage. Events are sorted by time and streamed to git fast-import as one commit each on refs/heads/mull (and main). Records are encoded with model.Encode with the same bodies mull's put* functions use, and every path is checked against model.ValidDataPath. A seeded PCG makes it deterministic for a given seed and end time (tested).

  I rejected seeding through mull's ops (Apply/batch): every write reloads the whole branch, so ~27k commits that way is O(n^2) and takes most of an hour. That is itself a finding for phase 2. Fidelity is covered by tests instead: every stage and plan state appears, no done-not-in-git problems, and the repository accepts a real mull write (format 2, no legacy records).

  Measuring (bench.go). For each repository it reads the shape, then picks targets: the whole-branch load, the summary, overview, tree, backlog, unplanned and board, the busiest open and done plans, a phase of the open plan, an item in review, one active and one done, and revision from several distances back. Each target runs once cold, then N times; it reports the median, min and max, and git calls from internal/git's new counter (one atomic add per runner; every git call already goes through there). The same targets go over HTTP through serve.Handler in an httptest server (so git calls still count), and optionally against a live server by matching project paths. For the revision endpoint it marks the last look N commits back in per-machine state, which run points at a temp XDG_CONFIG_HOME so the real registry and last looks are never touched.

  make bench runs on this repository's own mull branch (read only) and on .dev/bench/$(BENCH_SIZE), seeding it once. CLAUDE.md now says to measure before and after read-performance changes.
decisions:
  - id: 6f63
    text: The bench seeder writes the mull branch with git fast-import, encoding records with internal/model, instead of running mull's ops
    detail: 'Each op reloads the whole branch, so ~27k commits through ops take most of an hour; fast-import builds them in 45s. Fidelity is checked by tests: every stage and plan state present, done work found on main, the result writable.'
  - id: dacd
    text: make bench reads this repository's real mull branch (read only) as well as the seeded one, with per-machine state pointed at a temp dir
    detail: The real branch is the baseline the human feels; nothing is written to it, and seen/registry go to a disposable XDG_CONFIG_HOME.
  - id: df50
    text: internal/git counts the git processes it starts (git.Calls), so the benchmark reports git calls per page
    detail: Every git call already goes through internal/git; an atomic counter there is one line per runner and needs no tracing setup.
  - id: ebbc
    text: The benchmark is a dev tool (scripts/bench, internal/bench, make bench), not a mull subcommand
    detail: It's for working on mull, not for agents or the human; keeping it out of the CLI keeps mull's surface unchanged.
---

# Done report 1: Benchmark: seed a large mull repository and time every serve page, with git calls counted (dafa)

**Status:** PASSED  
**Commit:** 8e7e3b94ad4b  
**By:** /Users/jeff/dev/kit at 2026-11-11T14:44:35Z

## What was built

### Summary

make bench measures mull's reads. It seeds a large repository (5000 items, 10935 inbox entries, 27403 mull commits, 4161 commits on main) in 45s and times every serve page in-process, over HTTP through this tree's handler, and against a running server (-url), with git calls counted. Baseline on the large seed: load 237ms, every page 460-650ms, revision 1.1s at 10 commits back and 4.8s at 10000, project list 5.4s. On kit, the installed server is 3.5-4x slower than the same handler run in the foreground (fa47). Later mull-data phases are judged by these numbers.

### UI tour

1. In mull/, run: make bench BENCH_FLAGS="-n 3 -url http://127.0.0.1:6855"
2. The first run seeds .dev/bench/large (about 45s); later runs reuse it.
3. See two tables, kit and large: a row per page with in-process, http and live medians, the cold first call, bytes, and git calls.
4. make bench BENCH_SIZE=small for a quick run; BENCH_FLAGS=-json for JSON.

### Checks (run by mull)

- `make done-checks`: pass, 88.0s

## How it was built

### Design notes

Two halves, in internal/bench, with a thin command in scripts/bench and make bench on top.

Seeding (seed.go). A generator lays out events in time: plans created, doc'd, agreed with phases chained by needs, closed; planning-session decisions; items that are captured (some triaged, some dropped) or added, committed, started with a claim, given decisions and sometimes a question, finished with a report and a review entry carrying the decisions, and accepted (settling the review and its decisions). Each item's work lands on main as a commit with a Mull trailer, so the board's git links (and done-not-in-git) behave as in a real repo. Fates are weighted so done plans are ~90% done and open plans carry every stage. Events are sorted by time and streamed to git fast-import as one commit each on refs/heads/mull (and main). Records are encoded with model.Encode with the same bodies mull's put* functions use, and every path is checked against model.ValidDataPath. A seeded PCG makes it deterministic for a given seed and end time (tested).

I rejected seeding through mull's ops (Apply/batch): every write reloads the whole branch, so ~27k commits that way is O(n^2) and takes most of an hour. That is itself a finding for phase 2. Fidelity is covered by tests instead: every stage and plan state appears, no done-not-in-git problems, and the repository accepts a real mull write (format 2, no legacy records).

Measuring (bench.go). For each repository it reads the shape, then picks targets: the whole-branch load, the summary, overview, tree, backlog, unplanned and board, the busiest open and done plans, a phase of the open plan, an item in review, one active and one done, and revision from several distances back. Each target runs once cold, then N times; it reports the median, min and max, and git calls from internal/git's new counter (one atomic add per runner; every git call already goes through there). The same targets go over HTTP through serve.Handler in an httptest server (so git calls still count), and optionally against a live server by matching project paths. For the revision endpoint it marks the last look N commits back in per-machine state, which run points at a temp XDG_CONFIG_HOME so the real registry and last looks are never touched.

make bench runs on this repository's own mull branch (read only) and on .dev/bench/$(BENCH_SIZE), seeding it once. CLAUDE.md now says to measure before and after read-performance changes.

### Decisions made in the session

Accepting the review ratifies these. To decide one differently, before accepting: `mull decide <id> "..."`

- **6f63** The bench seeder writes the mull branch with git fast-import, encoding records with internal/model, instead of running mull's ops
  Why: Each op reloads the whole branch, so ~27k commits through ops take most of an hour; fast-import builds them in 45s. Fidelity is checked by tests: every stage and plan state present, done work found on main, the result writable.
- **dacd** make bench reads this repository's real mull branch (read only) as well as the seeded one, with per-machine state pointed at a temp dir
  Why: The real branch is the baseline the human feels; nothing is written to it, and seen/registry go to a disposable XDG_CONFIG_HOME.
- **df50** internal/git counts the git processes it starts (git.Calls), so the benchmark reports git calls per page
  Why: Every git call already goes through internal/git; an atomic counter there is one line per runner and needs no tracing setup.
- **ebbc** The benchmark is a dev tool (scripts/bench, internal/bench, make bench), not a mull subcommand
  Why: It's for working on mull, not for agents or the human; keeping it out of the CLI keeps mull's surface unchanged.

### Changes by layer

**other**: 6 files, +1112 −1
- M mull/CLAUDE.md (+4 −0)
- M mull/Makefile (+10 −1)
- A mull/internal/bench/bench.go (+354 −0)
- A mull/internal/bench/seed.go (+552 −0)
- M mull/internal/git/git.go (+9 −0)
- A mull/scripts/bench/main.go (+183 −0)

**tests**: 1 files, +128 −0
- A mull/internal/bench/bench_test.go (+128 −0)
