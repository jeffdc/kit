# Canal

> The conduit between the ocean of thought and the ocean of implementation.

Canal is an inter-agent messaging system that coordinates work between LLM agents. It enables planning agents to dispatch work to coding agents, tracks task completion, and handles escalations when agents get blocked.

## The Problem

When working with multiple LLM agents - some for planning, some for coding, some for debugging - you end up manually shuttling information between them. Copy-paste prompts, relay status updates, track what's done and what's blocked. You become the message bus.

Canal fixes this. Agents communicate through a persistent queue with well-defined semantics:

- **Planners** create specs and publish work
- **Workers** pull tasks, execute them, and report completion
- **Humans** get notified only when decisions are needed

## How It Works

```
┌──────────┐     specs      ┌─────────────┐     claim/done     ┌──────────┐
│ Planner  │ ──────────────▶│    Queue    │◀───────────────────│  Worker  │
│  Agent   │                │   Service   │                    │  Agent   │
└──────────┘                └─────────────┘                    └──────────┘
      │                           │                                  │
      │        escalations        │         blocked                  │
      └───────────────────────────┼──────────────────────────────────┘
                                  │
                                  ▼
                            ┌──────────┐
                            │  Human   │
                            │(you)     │
                            └──────────┘
```

- **Specs** are self-contained task descriptions with acceptance criteria
- **Workers** run autonomously (Ralph loops) until the spec is satisfied
- **Exactly-once delivery** ensures work is never lost or duplicated
- **Escalation path** bubbles blockers up: Worker → Planner → Human

## Building Canal with Canal

Here's the fun part: **once we bootstrap the minimum viable queue service, we'll use Canal to build the rest of Canal.**

The bootstrap is a vertical spike - just enough to:
1. Submit a task to the queue
2. Have a worker claim it
3. Do the work
4. Mark it done

After that, every new feature becomes a spec in the queue. Workers (Claude agents in YOLO mode) pick up tasks and build them. The planner tracks progress. We dogfood from day one.

It's turtles all the way down.

## Status

**Current phase:** Bootstrapping the queue service.

See `plan/CANAL-SPEC.md` for the full specification.

## Project Structure

```
canal/
├── plan/                    # Plans and specs
│   ├── CANAL-SPEC.md        # System specification
│   └── specs/               # Task specs (canal-NNNN.md)
├── queue_service/           # Phoenix app (CanalQueue) [coming soon]
├── CODING_STANDARDS.md      # Elixir/Phoenix coding standards
├── CLAUDE.md                # Instructions for LLM agents
└── README.md                # You are here
```

## License

TBD
