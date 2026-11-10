# mull (next)

Guidance for any agent working on mull's code. This branch (`mull-next`) is a from-scratch rewrite; the design is `docs/design.md`. Read it before changing behaviour.

## What mull is

The record of a program, kept on a dedicated git branch (`mull`) in the project's repo:
- **plans** (top level, plain-English slugs);
- **phases** (big pieces of a plan; called epics before Model v2);
- **items** (one agent, one session);
- **a backlog** of captured items (bugs and ideas) that aren't work until the human commits them;
- **an inbox** of decisions, questions and reviews that need the human.

Agents and humans use it only through the CLI. Data commands print JSON; errors go to stderr as `{"code", "error"}` (see `internal/guide/json.md`).

## Rules

- **Any LLM, any harness.** No feature may depend on Claude Code, Codex or any other harness. The only interfaces are the CLI and git.
- **Never touch the working tree or the real index** when writing mull data. All writes go through `internal/store` (git plumbing plus a compare-and-swap on `refs/heads/mull`).
- **The only ref mull may write or push is `refs/heads/mull`.** It's a constant (`store.Ref`), and must stay one.
- **Installed as `mull`** with `make install` (cutover done 2026-11-09). The old binary is kept as `mull1` for reading old `.mull/` data.

## Layout

- `cmd/`: the Cobra CLI. Thin: it parses flags and prints, and the logic lives in `internal/mull`.
- `internal/git`: runs the `git` CLI. Every git call goes through here.
- `internal/store`: the `mull` branch. Load a snapshot, and update with compare-and-swap and retry.
- `internal/model`: records and their file format (YAML frontmatter plus a markdown body).
- `internal/mull`: operations: plans, phases, items, needs and readiness, claims, the inbox, the decision log.
- `internal/gittest`: throwaway repos and worktrees for tests, isolated from global git config.

## Build and test

```bash
make build      # ./mull
make test       # go test ./...
make race       # with the race detector
make fmt vet
```

Tests always use temporary repositories (`gittest.NewRepo`). Never run tests against a real repository's `mull` branch.
