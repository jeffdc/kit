---
name: user-profile
description: Use when starting a new tool or project and wanting to calibrate agent tone, or when user asks to set up or update their profile
---

# User Profile

Create or update `~/.claude/user-profile.md` — a global file skills cross-reference against the current project's stack to calibrate tone and depth.

**Core principle:** The profile is about the user, not the project. What changes per repo is the intersection of your profile with the project's stack.

## Check for Existing Profile

Read `~/.claude/user-profile.md`. If it exists, show it and ask if they want to update. If not, run the interview.

## Interview

Ask about each area conversationally — don't dump all questions at once:

1. **Experience** — overall level and years
2. **Languages & frameworks** — and proficiency for each:
   - Strong / Comfortable / Some experience / Learning
3. **Databases, infrastructure, deployment tools** — same proficiency scale
4. **Feedback preferences:**
   - Mentor/teaching vs peer vs terse
   - Thorough explanations vs just the answer
5. **Anything else** about how the agent should interact

Write `~/.claude/user-profile.md` when done.

## Profile Format

```markdown
# User Profile

## Experience
- 30+ years professional engineering

## Languages & Frameworks
- Strong: Java, JavaScript, Go, Scala, TypeScript, React, Next.js
- Comfortable: Postgres, SQLite
- Some experience: Elixir, Phoenix
- Learning: Fly.io

## Feedback Preferences
- Be direct, not flattering
- When I'm learning a stack: teach, explain idioms, draw parallels to strong languages
- When I'm strong in the stack: peer review, keep it brief
```

## How Skills Use the Profile

Skills read the profile at startup, then cross-reference against the project's stack (from CLAUDE.md / codebase). The agent determines tone automatically:

| User proficiency in project stack | Tone |
|-----------------------------------|------|
| Strong | Peer — brief, direct |
| Comfortable | Peer with occasional explanation |
| Some experience | Teaching — explain idioms, draw parallels to strong languages |
| Learning / none | Full mentor — explain everything, reference familiar tech |

No profile = no tailoring. Skills work fine without it.
