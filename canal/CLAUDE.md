# Canal - Project Instructions

## Overview

Canal is an inter-agent messaging system that coordinates work between LLM agents. See `plan/CANAL-SPEC.md` for the full specification.

## Coding Standards

Follow the Elixir/Phoenix coding standards in `CODING_STANDARDS.md`.

Key points:
- Run `mix format` before committing
- All code must pass `mix credo --strict`
- All code must pass `mix dialyzer`
- Treat warnings as errors (`--warnings-as-errors`)
- Public functions require `@doc` and `@spec`
- Use typespecs for all public APIs

## Project Structure

```
canal/
├── plan/                    # Plans and specs
│   ├── CANAL-SPEC.md        # System specification
│   └── specs/               # Task specs (canal-NNNN.md)
├── queue_service/           # Phoenix app (CanalQueue)
├── CODING_STANDARDS.md      # Elixir/Phoenix standards
└── CLAUDE.md                # This file
```

## Conventions

- Task IDs: `{project}-{sequence}` (e.g., `canal-0001`)
- Specs live in `plan/specs/`
- Plans live in `plan/`
