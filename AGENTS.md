# AGENTS.md

Guidance for any coding agent working in kit, a monorepo of small, independent tools. Each tool lives in its own directory with its own language, dependencies and build system. Tools do not share code.

## Tools

- **canal/**: inter-agent messaging system (Elixir/Phoenix)
- **cd2beets/**: safely rip audio CDs into a beets library (Python)
- **flint/**: quick capture to Obsidian, tagged by project (Go)
- **forage/**: personal book library, a Go server and PWA (Go)
- **mull/**: the record of a program run with agents: plans, phases, items, a backlog, and an inbox (Go)
- **omp/**: personal configuration for the OMP coding agent: the safety guard hook (TypeScript)
- **skills/**: agent skills, reference guides for techniques and workflows (Markdown)
- **watchmen/**: time tracking and invoice generation (Go)

## Working in this repo

- Each tool is self-contained. Build, test and install from within its directory.
- Most tools have their own `CLAUDE.md` or `README.md` with tool-specific instructions. It is not loaded automatically: read it before working in that tool.
- Do not create cross-tool dependencies or shared packages.

## Checks

Before you call work finished, run `make done-checks` at the repo root. It runs vet, tests and fmt-check for flint, forage, mull and watchmen, so main never goes red. For omp, run `make test` in `omp/`.
