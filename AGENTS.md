# AGENTS.md

Guidance for any coding agent working in kit, a monorepo of small, independent tools. Each tool has its own directory, language and build. Read the tool's own CLAUDE.md or README before working in it.

## Checks

Before you call work finished, run `make done-checks` at the repo root. It runs vet, tests and fmt-check for flint, forage, mull and watchmen, so main never goes red.
