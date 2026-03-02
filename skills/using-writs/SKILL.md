---
name: using-writs
description: Use when starting any conversation - establishes how to find and use skills, requiring Skill tool invocation before ANY response including clarifying questions
---

## Hard Constraints

These are not suggestions. They override your defaults. Violating any of them is a skill failure.

**1. Scan the skill list before your first response.**
When you receive a user message, read the available skills list in your system context. For each skill, ask: "Could this apply?" If yes or even maybe — invoke it with the Skill tool. Do this BEFORE responding, including before asking clarifying questions.

**2. State which skills you considered.**
Before your first response to a new task, say which skills you checked and which you're using. Example: "Considered: brainstorming, TDD, systematic-debugging. Using: TDD for this bugfix." This is your echo gate. If you didn't do this, you skipped the skill check.

**3. Invoke, don't remember.**
Never rely on memory of what a skill says. Skills change. Invoke the Skill tool to load the current version every time.

**4. Skills are not optional when they match.**
If a skill's description matches what you're doing, you must invoke it. This is not a judgment call. "It seems like overkill" is not a valid reason to skip.

### Why these exist

Claude processes the skill list, understands it, and then responds without invoking anything. The scan-and-state requirement (constraints 1-2) forces a visible, verifiable pause. If the user doesn't see "Considered: X, Y, Z" at the start of a response, they know the skill check was skipped.

---

<EXTREMELY-IMPORTANT>
If you think there is even a 1% chance a skill might apply to what you are doing, you ABSOLUTELY MUST invoke the skill.

IF A SKILL APPLIES TO YOUR TASK, YOU DO NOT HAVE A CHOICE. YOU MUST USE IT.

This is not negotiable. This is not optional. You cannot rationalize your way out of this.
</EXTREMELY-IMPORTANT>

## How to Access Skills

**In Claude Code:** Use the `Skill` tool. When you invoke a skill, its content is loaded and presented to you — follow it directly. Never use the Read tool on skill files.

**In other environments:** Check your platform's documentation for how skills are loaded.

## The Procedure

**On every user message:**

1. Read the skill list in your system context
2. For each skill, check: does the description match this task?
3. Invoke all matching skills with the Skill tool
4. State what you considered: "Considered: [list]. Using: [list]."
5. Follow invoked skills exactly
6. If a skill has a checklist, create a TaskCreate item per checklist entry
7. Then respond to the user

**On EnterPlanMode:** Check if brainstorming has been done. If not, invoke brainstorming first.

## Skill Priority

When multiple skills could apply, use this order:

1. **Process skills first** (brainstorming, debugging) - these determine HOW to approach the task
2. **Implementation skills second** - these guide execution

"Let's build X" → brainstorming first, then implementation skills.
"Fix this bug" → systematic-debugging first, then domain-specific skills.

## Skill Types

**Rigid** (TDD, debugging): Follow exactly. Don't adapt away discipline.

**Flexible** (patterns): Adapt principles to context.

The skill itself tells you which.

## Red Flags

These thoughts mean STOP — you're rationalizing:

| Thought | Reality |
|---------|---------|
| "This is just a simple question" | Questions are tasks. Check for skills. |
| "I need more context first" | Skill check comes BEFORE clarifying questions. |
| "Let me explore the codebase first" | Skills tell you HOW to explore. Check first. |
| "I can check git/files quickly" | Files lack conversation context. Check for skills. |
| "Let me gather information first" | Skills tell you HOW to gather information. |
| "This doesn't need a formal skill" | If a skill exists, use it. |
| "I remember this skill" | Skills evolve. Read current version. |
| "This doesn't count as a task" | Action = task. Check for skills. |
| "The skill is overkill" | Simple things become complex. Use it. |
| "I'll just do this one thing first" | Check BEFORE doing anything. |
| "This feels productive" | Undisciplined action wastes time. Skills prevent this. |
| "I know what that means" | Knowing the concept ≠ using the skill. Invoke it. |

## User Instructions

Instructions say WHAT, not HOW. "Add X" or "Fix Y" doesn't mean skip workflows.
