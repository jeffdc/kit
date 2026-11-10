# CLAUDE.md

This is a monorepo of small, independent CLI tools ("kit"). Each tool lives in its own directory with its own language, dependencies, and build system. Tools do not share code.

## Tools

- **watchmen/** - Time tracking and invoice generation (Go)
- **canal/** - Inter-agent messaging system (Elixir/Phoenix)
- **mull/** - The record of a program run with agents: plans, phases, items, a backlog, and an inbox of what needs the human (Go)

## Tracking work

kit's work is tracked with mull. Follow AGENTS.md (the mull protocol) for every task: start from `mull context`, claim one item per session, and finish with `mull done`.

## Working in this repo

- Each tool is self-contained. Build, test, and install from within its directory.
- Each tool has its own `CLAUDE.md` with tool-specific instructions. Read it before working in that tool.
- Do not create cross-tool dependencies or shared packages.
