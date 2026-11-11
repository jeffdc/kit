# mull

Guidance for any agent working on mull's code. The design is `docs/design.md` (and `docs/serve/` for `mull serve`). Read it before changing behaviour.

## What mull is

The record of a program, kept on a dedicated git branch (`mull`) in the project's repo:
- **plans** (top level, plain-English slugs);
- **phases** (big pieces of a plan; called epics before Model v2);
- **items** (one agent, one session);
- **a backlog** of captured items (bugs and ideas) that aren't work until the human commits them;
- **an inbox** of decisions, questions and reviews that need the human;
- **mull serve,** the human's view: a local web UI of every registered repository.

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
- `internal/registry` and `internal/seen`: per-machine state in `~/.config/mull/` (the repositories on mull, and when the human last looked at each), never on the mull branch.
- `internal/launchagent`: `mull serve --install`, the macOS LaunchAgent.
- `internal/bench` and `scripts/bench`: `make bench`. Seeds large repositories (through git fast-import, as mull's writes would have built them) and times every page mull serve shows, counting git calls.
- `internal/serve`: `mull serve`, the human's web UI (Go server; plain JS modules, CSS and fonts in `web/`, embedded). Its design lives in `docs/serve/`: `direction.md` (the contract), `PRODUCT.md`, `prototype.html`, and `DESIGN.md`, the design system as built. Read `DESIGN.md` before changing the UI; it's there rather than at the project root, where impeccable looks first.

## Build and test

```bash
make build      # ./mull
make test       # go test ./...
make race       # with the race detector
make fmt vet
make dev        # build and (re)start a scratch mull serve on a seeded demo repo: http://localhost:6899/#/demo
make dev-stop   # stop it; make dev-reset starts again from a fresh demo repo
make bench      # time every serve page on this repo's mull branch and a seeded large one (.dev/bench/); see the Makefile for flags
```

To try a UI change, use `make dev`, not a hand-started server: it serves this tree's build on port 6899 from `.dev/` (ignored), with its own config dir so the real registry and last-looked state are never touched, and it stops only the server it started (by its PID file). Don't start, kill or `pkill` servers at the prompt; the human's permissions ask on every kill.

Measure before and after any change meant to make reads faster, with `make bench`, and put the numbers in the done report.

Tests always use temporary repositories (`gittest.NewRepo`). Never run tests against a real repository's `mull` branch.
