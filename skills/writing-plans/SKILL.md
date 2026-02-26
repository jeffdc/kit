---
name: writing-plans
description: Use when you have a spec or requirements for a multi-step task, before touching code
---

# Writing Plans

## Overview

Write implementation plans with enough detail that an engineer with zero codebase context can execute them. Document which files to touch, what to test, and what the expected behavior is. Scale granularity to complexity.

TDD is the implementation discipline — the plan says *what* to build and test, not *how* to run a test command.

## Where Plans Live

Plans go in the mull matter body. If brainstorming already ran, append the plan after the design section.

```bash
mull append <id> - --replace <<EOF
<existing design content, if any>

## Implementation Plan

<plan content>
EOF
git add .mull/ && git commit -m "Add implementation plan for <topic>"
```

## Plan Structure

Every plan starts with a header, then tasks.

### Header

```markdown
## Implementation Plan

**Goal:** One sentence — what does this build?

**Architecture:** 2-3 sentences about the approach.

**Tech Stack:** Key technologies, libraries, frameworks.
```

### Tasks

Each task is a meaningful unit of work — a feature slice, a component, a behavior. Not a single function call or a single test assertion.

````markdown
### Task N: [Component/Behavior Name]

**Files:**
- Create: `exact/path/to/file.ext`
- Modify: `exact/path/to/existing.ext` (what changes)
- Test: `tests/exact/path/to/test.ext`

**Behavior:**
Describe what this task builds. What should the code do? What are the inputs, outputs, edge cases?

**Testing:**
What behaviors to verify. Name the test cases — the developer uses TDD to implement them.

```elixir
# Code sketches where helpful, not mandatory
def example do
  :ok
end
```

**Notes:**
Anything non-obvious — gotchas, dependencies on prior tasks, relevant docs to check.
````

## Scaling Granularity

| Complexity | Tasks look like | Example |
|-----------|----------------|---------|
| **Simple** | 2-3 tasks, brief descriptions, minimal code sketches | Add a CLI flag with validation |
| **Medium** | 4-8 tasks, behavior descriptions with test cases named, code sketches for non-obvious parts | New command with subcommands |
| **Complex** | 8+ tasks, detailed behavior specs, code sketches, dependency ordering between tasks | New subsystem or major refactor |

**Don't micromanage.** A task like "Add the `purge` command" is fine if the behavior section describes what purge does. You don't need separate steps for "write the test", "run the test", "implement", "run again", "commit".

**Do be specific.** Exact file paths always. Name the test cases. Describe edge cases. Include code sketches when the approach isn't obvious from the description.

## Principles

- **Exact file paths** — every task says which files to create, modify, or test
- **TDD is assumed** — name the test cases, describe the behaviors, trust the developer to red-green-refactor
- **DRY / YAGNI** — don't plan features that weren't in the design
- **Dependency order** — if task 3 depends on task 2, say so
- **Complete enough to execute cold** — someone with no context should be able to pick this up

## After Writing the Plan

Commit the matter, then present options:

**"Plan captured in matter `<id>`. How would you like to proceed?"**

1. **Execute now** — invoke the executing-plans skill to start implementation
2. **Execute later** — plan is saved, come back to it when ready

Don't choose for the user.
