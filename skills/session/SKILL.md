---
name: session
description: Use when wrapping up a work session and wanting to capture what happened — what changed, decisions made, open questions — before ending the conversation
---

# Session Log

Capture a work session as a structured log. Invoked explicitly at the end of a session.

## Gather Context

1. `git log --oneline --since="4 hours ago"` (adjust window to session length)
2. Note which mull matters were referenced or advanced in this conversation
3. Review the conversation for decisions, trade-offs, and unresolved questions

## Draft the Session

Write three sections:

```markdown
## What changed
- Concrete things that were built, fixed, or modified (reference commits where useful)

## Decisions
- What was decided and why — especially non-obvious choices or trade-offs

## Open questions
- Unresolved items, things to revisit, blockers
```

**Keep it concise.** Strip anything recoverable from git history or the code itself. Focus on the *why* — decisions, context, and intent that aren't captured elsewhere.

**Skip empty sections.** If there are no open questions, omit the section.

## Present for Review

Show the draft to the user. Wait for approval or edits before saving.

## Save

```bash
mull session save --matter <id1> --matter <id2> - <<'EOF'
<approved session body>
EOF
```

Commit the session file:
```bash
git add .mull/sessions/ && git commit -m "Session log: <brief summary>"
```
