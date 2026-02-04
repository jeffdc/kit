---
name: wrap
description: Wrap up work session - review commits, summarize work, and stop timer with summary note
---

# Wrap Up Work Session

Follow this workflow to wrap up your current work session:

## 1. Get Timer Context

First, check the watchmen status to determine:
- The project being tracked (use this project name)
- When the timer started (use this to find relevant commits)

## 2. Review Git Commits

Find all git commits made since the timer started. Be precise with the time range - aim for accuracy within 5 minutes to avoid missing commits. Use `git log --since="<timestamp>"` with the timer's start time.

## 3. Generate Summary

Create a concise summary of what was accomplished during this session based on:
- The commit messages and changes
- The work visible in git history
- Keep it brief but informative (1-3 sentences)

$ARGUMENTS

## 4. Stop Timer with Summary

Stop the watchmen timer using the generated summary as the note:
```
watchmen stop --note "your summary here"
```

## Important Notes

- **Do not assume** - if the git history is unclear or you need clarification, ask the user
- **Be accurate** - ensure the time range captures all relevant work
- **Be concise** - the summary should be informative but brief
- The project name comes from watchmen status, not from assumptions
