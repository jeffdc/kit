# Rules

The rules for kit, Jeff's. Each has a check that `mull done` runs, unless it says it's advisory. A ratcheted check prints its violations, and the known ones live in `.mull/ratchets/<ID>.txt`, which may only shrink.

## R1: Tools never share code

- scope: flint/**, forage/**, mull/**, watchmen/**
- check: for t in flint forage mull watchmen; do others=$(echo flint forage mull watchmen | tr " " "\n" | grep -vx $t | paste -sd"|" -); grep -rlE --include="*.go" "\"($others)/" $t; done; true
- ratchet: yes

Each tool has its own directory, language, dependencies and build. No tool imports another, and there are no shared packages.

## R2: Each tool builds, tests and installs from its own directory

Advisory. The root Makefile only runs each tool's own targets.

## R3: Only mull's sync pushes, and only the mull branch

- scope: mull/**
- check: ! grep -rn --include="*.go" "\"push\"" mull/internal mull/cmd | grep -v _test.go | grep -v internal/core/sync.go

The store writes only `refs/heads/mull` and sync pushes only it. The runner writes local code branches and never pushes.

## R4: The runner never writes the release branch

- scope: mull/internal/core/runner.go, mull/internal/core/done.go, mull/internal/core/inbox.go

Advisory: held by the runner's tests. Work lands on plan branches or `next`; only Jeff ships to main.

## R5: Any LLM, any harness

- scope: mull/**
- check: ! grep -rIl --include="*.go" -e CLAUDECODE -e "claude-code" mull/cmd mull/internal

No mull feature depends on Claude Code, Codex or another harness. The only interfaces are the CLI and git.

## R6: mull's data writes never touch a working tree or the real index

- scope: mull/internal/store/**, mull/internal/core/**

Advisory: every write goes through internal/store (git plumbing and a compare-and-swap on the mull branch).

## R7: Tests never touch a real repository's mull branch

- scope: **/*_test.go

Advisory: tests use temporary repositories (gittest.NewRepo), isolated from global git config.

## R8: Every git call in mull goes through internal/git

- scope: mull/**
- check: grep -rl --include="*.go" "exec.Command(\"git\"" mull | grep -v -e mull/internal/git/ -e mull/internal/gittest/ -e _test.go; true
- ratchet: yes

So every call is counted and its errors read the same way. The bench seed's fast-import is the one known exception.

## R9: No agent pushes to main

Advisory: the pre-push hook refuses it, whatever the prompt says.

## R10: mull serve's UI follows its design system

- scope: mull/internal/ui/web/**

Advisory: read `mull/docs/serve/DESIGN.md` before changing the UI, and use its components.

## R11: Measure before and after making reads faster

- scope: mull/internal/core/**, mull/internal/store/**

Advisory: time `mull status` on a large repository before and after, and put the numbers in the report.

## R12: Prose is never hard-wrapped

- scope: **/*.md

Advisory: each paragraph or list item is one line, in docs, guides, reports and commit message bodies.
