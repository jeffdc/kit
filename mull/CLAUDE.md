# mull

Guidance for any agent working on mull's code. The design is `docs/redesign.md`. Read it before changing behaviour.

## What mull is

The record of a program run with agents, kept on a dedicated git branch (`mull`) in the project's repo: plans, phases and items; the agents' queue of bugs and ideas; and an inbox of what needs the human (questions with a default, reviews of high-risk work). Beside it in the repo: RULES.md, the rulebook. mull runs the work through git (branches, worktrees, landing) and shows the human one surface: `mull status` or `mull serve`.

## Rules

- **Any LLM, any harness.** No feature may depend on Claude Code, Codex or any other harness. The only interfaces are the CLI and git.
- **Never touch the working tree or the real index** when writing mull data. All writes go through `internal/store` (git plumbing plus a compare-and-swap on `refs/heads/mull`).
- **The store writes, and sync pushes, only `refs/heads/mull`.** It's a constant (`store.Ref`), and must stay one.
- **The runner writes local code branches only, never the release branch, and never pushes.** `start` makes an item's branch and worktree (and the target, from the release branch, if it's missing), `done` and `accept` fast-forward the target, and pruning removes merged worktrees and their branches. Changed with Jeff on 2026-11-23 for the redesign (`docs/redesign.md`).
- **Installed as `mull`** with `make install` (cutover done 2026-11-09). The old binary is kept as `mull1` for reading old `.mull/` data.

## Layout

- `cmd/`: the Cobra CLI. Thin: it parses flags and prints; the logic lives in `internal/core`.
- `internal/core`: mull itself: records, plans, phases, items, the inbox, done (checks, rules, risk), the runner (branches, worktrees, landing), the surface, the map and audits, batch and sync.
- `internal/git`: runs the `git` CLI. Every git call goes through here.
- `internal/store`: the `mull` branch: load a snapshot, update with compare-and-swap and retry.
- `internal/gittest`: throwaway repos and worktrees for tests, isolated from global git config.
- `internal/registry`, `internal/seen`, `internal/human`: per-machine state in `~/.config/mull/` (registered repositories, the human's last look, their name), never on the mull branch.
- `internal/launchagent`: `mull serve --install`, the macOS LaunchAgent.
- `internal/ui`: `mull serve`, the human's web UI: a small Go server and plain JS in `web/`, embedded. Read `docs/serve/DESIGN.md` before changing it.
- `internal/guide`: the guides `mull guide` prints.

## Build and test

```bash
make build      # ./mull
make test       # go test ./...
make fmt vet
```

To try a UI change, run `mull serve` with `HOME` and `XDG_CONFIG_HOME` pointing at a scratch directory, so the real registry and last look are never touched. Don't start, kill or `pkill` servers at the prompt.

Tests always use temporary repositories (`gittest.NewRepo`). Never run tests against a real repository's `mull` branch.
