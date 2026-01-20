# Canal Specification

> The conduit between the ocean of thought and the ocean of implementation.

Canal is an inter-agent messaging system that coordinates work between LLM agents. It enables planning agents to dispatch work to coding agents, tracks task completion, and handles escalations when agents get blocked.

## Overview

Canal solves the problem of manually shuttling information between multiple LLM agents. Instead of copying and pasting prompts between sessions, agents communicate through a persistent message queue with well-defined semantics.

### Core Principles

- **Agents are stateless**: Context lives in the message and in the repo, not in agent memory
- **Planners don't know workers**: Planners publish work; workers pull it. No direct assignment.
- **Specs are self-contained**: Everything needed to complete the work is in the spec
- **Exactly-once delivery**: Work is never lost, never duplicated
- **Escalation path**: Worker → Planner → Human

---

## Roles

### Worker

Executes specs using a Ralph loop (autonomous execution until spec is satisfied).

**Responsibilities:**
- Pull tasks from the queue when available
- Execute the spec until acceptance criteria are met
- Heartbeat while working to maintain task claim
- Report completion or escalate blockers
- Clear context between tasks

**Capabilities:**
- Build, review, test, and iterate on code
- Run in YOLO mode (auto-approve tool calls)
- Operate on any project (project-agnostic)

### Planner

Orchestrates work for a single project. Works with the human to create plans, generates specs, tracks progress, and handles escalations.

**Responsibilities:**
- Maintain the overall plan for the project
- Break down the plan into specs
- Submit specs to the work queue
- Track task status (pending, claimed, done, blocked)
- Attempt to resolve worker escalations
- Escalate to human when unable to resolve
- Resubmit tasks after blockers are resolved

**Constraints:**
- One planner per project
- Always running (to receive escalations)
- Cannot assign work to specific workers (only publishes to queue)

### Human

Provides oversight, makes critical decisions, and handles escalations the planner cannot resolve.

**Responsibilities:**
- Work with planner to create and review top-level plans
- Respond to escalations that require human judgment
- Sign off on critical milestones
- Monitor progress across all projects
- Adjust priorities when needed

**Interface needs:**
- Dashboard showing all projects with drill-down
- Notifications for items requiring attention
- Ability to respond to escalations
- Controls for pause/resume, reprioritize, inject tasks, kill tasks

---

## Message Format

### Spec (Planner → Worker)

When a planner publishes work to the queue:

| Field | Required | Description |
|-------|----------|-------------|
| `task_id` | Yes | Unique identifier (`{project}-{sequence}`, e.g., `canal-0042`) |
| `project` | Yes | Project identifier |
| `spec` | Yes | What needs to be done (markdown text) |
| `acceptance_criteria` | Yes | How to verify the work is complete |
| `source_control` | Yes | Git configuration (see below) |
| `origin` | Yes | Who sent this (for escalations) |
| `dependencies` | No | Task IDs that must complete first |
| `priority` | No | Ordering hint (default: FIFO) |
| `constraints` | No | Things to avoid, patterns to follow |

### Source Control Configuration

| Field | Required | Description |
|-------|----------|-------------|
| `repo_path` | Yes | Local path to repo (fetch if exists, clone if not) |
| `repo_url` | No | Clone source (only needed if path doesn't exist) |
| `base_branch` | Yes | Branch to start from |
| `work_branch` | Yes | Branch naming pattern (e.g., `task/{task_id}`) |
| `pr_required` | Yes | Whether to open a PR when done |
| `pr_target` | No | Where PR merges to (if PR required) |

### Completion Message (Worker → Queue)

When a worker completes a task:

| Field | Description |
|-------|-------------|
| `task_id` | Which task was completed |
| `status` | `done` |

That's it. Completion is binary. The proof is in the repo (code merged).

### Blocked Message (Worker → Queue)

When a worker cannot proceed:

| Field | Description |
|-------|-------------|
| `task_id` | Which task is blocked |
| `status` | `blocked` |
| `blocker_description` | What's preventing progress |
| `attempts_made` | What the worker tried before escalating |
| `decision_needed` | Specific question or decision required |
| `context` | Error messages, file references, relevant details |

---

## Queue Semantics

### Task States

| State | Description |
|-------|-------------|
| `pending` | In queue, available for any worker to claim |
| `claimed` | Worker has picked it up, invisible to other workers |
| `done` | Work complete, code merged, message deleted |
| `blocked` | Needs intervention, escalated to planner |

### Exactly-Once Delivery

1. Worker claims task → task becomes invisible to other workers
2. Visibility timeout starts (system-wide configurable)
3. Worker sends periodic heartbeats to extend the timeout
4. If no heartbeat before timeout → task returns to `pending`
5. Worker marks `done` **only after** code is pushed and merged
6. `done` permanently removes the task from the queue

### Retry Limit

If a task times out N times (configurable), it auto-escalates as blocked with a note that multiple workers have failed on it.

### Queue Structure

- Single shared queue across all projects
- Workers pull FIFO (first-in-first-out)
- Tasks are tagged with project ID for filtering/reporting
- Future enhancement: human can nudge priority

---

## Escalation Flow

```
Worker hits blocker
        │
        ▼
┌───────────────────┐
│  Planner receives │
│  blocked task     │
└───────────────────┘
        │
        ▼
   Can planner
    resolve?
   /         \
  Yes         No
  │            │
  ▼            ▼
Planner     Planner
resubmits   escalates
task        to human
               │
               ▼
        Human provides
        input/decision
               │
               ▼
        Planner receives
        input, resubmits
        task with updates
```

**Key points:**
- Planner always tries to resolve before escalating to human
- Human never interacts with work queue directly (always through planner)
- Planner decides on urgency/importance of escalations
- Only planners submit/resubmit work to the queue

---

## Persistence

### What Lives in the Repo

| Data | Location | Format |
|------|----------|--------|
| Overall plan | `plan/PLAN.md` | Markdown |
| Task specs | `plan/specs/{task-id}.md` | Markdown |

Plans and specs are version-controlled, human-readable, and durable.

### What Lives in the Queue Service

| Data | Purpose |
|------|---------|
| Task queue entries | Track pending/claimed/blocked status |
| Visibility timeouts | Track when claims expire |
| Heartbeat timestamps | Track worker liveness |
| Retry counts | Track task failures |
| Completion records | Audit trail of done tasks |

This operational data needs queue semantics (atomic claims, timeouts) and changes frequently.

---

## Planner State

The planner must maintain state across sessions. It cannot rely on context windows.

**Plan-level state (in repo):**
- Overall goal/vision
- Top-level plan (phases, milestones)
- All specs created

**Task-level state (in queue service):**
- Task ID → status mapping
- Which tasks are blocked and why
- Dependency tracking
- Completion timestamps

When a planner starts up, it reads the plan from the repo and task state from the queue service to understand current status.

---

## Multi-Project

| Aspect | Design |
|--------|--------|
| Workers | Project-agnostic, can work on any task |
| Planners | One per project |
| Queue | Shared across all projects |
| Ordering | FIFO (priority nudging as future enhancement) |
| Human view | All projects at a glance, drill-down per project |

---

## Agent Registration

**Implicit via activity.** No explicit registration step.

- Workers are known when they claim tasks
- Planners are known per project (always running)
- "Active agents" inferred from tasks in progress

For v1, we track "tasks in progress" rather than "active workers."

---

## Authentication

**API keys** for v1.

- Each agent gets a secret API key
- Key included in every request to queue service
- Keys managed outside the system (bootstrap process)

### Bootstrap

Agents need to be configured with:
- Queue service URL
- API key
- Any project-specific configuration

This can be via environment variables, config file, or launch arguments. Details TBD during implementation.

---

## Human Interface

### Visibility (Read)

| View | Description |
|------|-------------|
| All projects | Overview of all projects with status summary |
| Project detail | Plan status, active tasks, blocked items, queue depth |
| Task detail | Spec, status, history, related escalations |
| Blocked items | Everything needing human attention |
| Recently completed | What got done |

### Actions (Write)

| Action | Description |
|--------|-------------|
| Respond to escalation | Provide decision/info planner needs |
| Pause project | Stop work on a project temporarily |
| Resume project | Continue work on a paused project |
| Reprioritize | Change what gets worked on next |
| Inject task | Add urgent work ("drop everything, do this") |
| Kill task | Cancel a task that's no longer needed |

### Notifications

Push notifications for:
- Blocked items requiring human attention
- Critical failures (task failed N times)
- Milestone completions (configurable)

---

## Naming Conventions

| Entity | Convention | Example |
|--------|------------|---------|
| Project ID | lowercase, hyphenated | `canal`, `gas-town`, `my-app` |
| Task ID | `{project}-{sequence}` | `canal-0042` |
| Plan file | `plan/PLAN.md` | - |
| Spec files | `plan/specs/{task-id}.md` | `plan/specs/canal-0042.md` |

---

## Components to Build

### 1. Queue Service

A lightweight HTTP service that manages the task queue.

**Endpoints (draft):**
- `POST /tasks` - Submit a new task
- `POST /tasks/claim` - Claim the next available task
- `POST /tasks/{id}/heartbeat` - Extend visibility timeout
- `POST /tasks/{id}/complete` - Mark task as done
- `POST /tasks/{id}/blocked` - Escalate as blocked
- `GET /tasks` - List tasks (with filters)
- `GET /tasks/{id}` - Get task details
- `GET /projects` - List projects with status summary
- `GET /projects/{id}` - Get project details

**Requirements:**
- Persistent storage (survives restarts)
- Visibility timeout handling
- Heartbeat tracking
- Multi-project support
- API key authentication

**Location:** Part of the kit project.

### 2. Agent SDK/Library

Helpers for agents to interact with Canal.

**For Workers:**
- Claim next task
- Send heartbeat (background, automatic)
- Mark complete
- Escalate as blocked
- Read spec from repo

**For Planners:**
- Submit task (write spec to repo + add to queue)
- Get task status
- Handle escalations
- Update plan state

### 3. Human Dashboard

Web UI or CLI for human oversight.

**Features:**
- View all projects
- Drill down to project/task details
- Respond to escalations
- Pause/resume/reprioritize/inject/kill
- Notifications for items needing attention

### 4. Bootstrap Tooling

Scripts or commands to:
- Set up a new project in Canal
- Generate/manage API keys
- Configure agent environments

---

## Open Questions

Items to resolve during implementation:

1. **Visibility timeout duration** - What's the right default? 30 minutes? 1 hour? Should it be spec-configurable?

2. **Retry limit** - How many timeouts before auto-escalate? 3?

3. **Heartbeat interval** - How often should workers heartbeat? Every 5 minutes?

4. **Dashboard implementation** - Web UI vs CLI vs both?

5. **Planner always running** - How does this work in practice? Long-running process? Webhook-triggered?

6. **Spec format details** - Any structured sections beyond what's listed, or is it freeform markdown?

7. **Notification delivery** - Push notifications how? Email? Slack? Native OS? Configurable?

---

## Future Enhancements

Not in v1, but on the radar:

- Priority nudging by human
- Worker capacity/capability tagging
- Parallel task limits per project
- Cost tracking (if using paid APIs/compute)
- Task time tracking and analytics
- Mobile-friendly dashboard
- Integration with beads or similar planning tools

---

## Glossary

| Term | Definition |
|------|------------|
| **Canal** | This inter-agent messaging and coordination system |
| **Worker** | An agent that executes specs (coding agent) |
| **Planner** | An agent that creates specs and orchestrates a project |
| **Spec** | A self-contained task description with acceptance criteria |
| **Ralph loop** | Autonomous agent execution until spec is satisfied |
| **Visibility timeout** | Time a claimed task stays invisible before returning to queue |
| **Heartbeat** | Signal from worker that it's still working on a task |
| **Escalation** | Routing a blocked task up the chain (Worker → Planner → Human) |
