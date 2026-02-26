---
name: brainstorming
description: Use before any creative work - creating features, building components, adding functionality, or modifying behavior. Explores user intent, requirements and design before implementation.
---

# Brainstorming Ideas Into Designs

## Overview

Turn ideas into designs through collaborative dialogue. Understand the project context, ask questions to refine the idea, present the design, capture it in a mull matter.

**Core principle:** No implementation without an approved design. Scale the ceremony to the scope.

<HARD-GATE>
Do NOT invoke any implementation skill, write any code, scaffold any project, or take any implementation action until you have presented a design and the user has approved it.
</HARD-GATE>

## Scaling Ceremony to Scope

Not every feature needs a full design document. Match the depth to the complexity:

| Scope | Design depth | Example |
|-------|-------------|---------|
| **Trivial** | 2-3 sentences in the matter body | Config change, flag addition |
| **Small** | A paragraph per section, skip sections that don't apply | New CLI command, simple refactor |
| **Medium** | Full treatment, each section a few sentences | New subsystem, API redesign |
| **Large** | Full treatment with alternatives analysis | Architecture change, new tool |

Even trivial changes get a design — it's just a short one. The point is to think before building, not to produce paperwork.

## Process

### 1. Explore project context

Check files, docs, recent commits. Understand what exists before proposing what to build.

### 2. Ask clarifying questions

- One question at a time — don't overwhelm
- Prefer multiple choice when possible
- Focus on: purpose, constraints, success criteria
- Keep going until you understand what you're building

### 3. Propose approaches

- Present 2-3 approaches with trade-offs
- Lead with your recommendation and why
- For trivial/small scope, one clear recommendation with a brief alternative is fine

### 4. Present design

- Scale each section to its complexity
- Ask after each section whether it looks right
- Cover what's relevant: architecture, components, data flow, error handling, testing
- Skip sections that don't apply — a config change doesn't need an architecture section
- YAGNI ruthlessly

### 5. Capture in mull matter

Once the user approves the design:

**If a matter already exists** (user provided an ID or one is obvious from context):
```bash
# Append the design to the matter body
mull append <id> - --replace <<EOF
<design content>
EOF
git add .mull/ && git commit -m "Capture design for <topic>"
```

**If no matter exists yet:**
```bash
mull add "<feature name>" --tag design
# then append the design content to the new matter
mull append <id> - --replace <<EOF
<design content>
EOF
git add .mull/ && git commit -m "Add matter with design for <topic>"
```

### 6. What next?

Present the user with options:

**"Design captured in matter `<id>`. What would you like to do?"**

1. **Implement directly** — for small/trivial scope where the design is the plan. Docket the matter (`mull dock <id>`) and start building with TDD.
2. **Write an implementation plan** — for medium/large scope. Docket the matter (`mull dock <id>`) and invoke the writing-plans skill.
3. **Stop here** — design is captured, user will decide when to proceed. Don't docket.

Do NOT choose for the user. Present the options and wait.

## Key Principles

- **One question at a time** — don't overwhelm with multiple questions
- **Multiple choice preferred** — easier to answer than open-ended
- **YAGNI ruthlessly** — remove unnecessary features from all designs
- **Explore alternatives** — always propose approaches before settling
- **Scale to scope** — a config change gets 3 sentences, not 3 pages
- **Incremental validation** — present design, get approval before moving on
- **Mull is the record** — design lives in the matter, not in loose files
