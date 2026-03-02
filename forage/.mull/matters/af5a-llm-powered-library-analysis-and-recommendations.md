---
status: raw
tags: [llm, forage]
created: 2026-03-01
updated: 2026-03-01
---

# LLM-powered library analysis and recommendations

Use the forage library as context for an LLM to provide recommendations, find gaps, and analyze reading patterns.

## Ideas discussed

- Feed the library (via forage prime or the SQLite DB directly) as LLM context
- Recommendations based on reading history and preferences
- Gap analysis — what's missing from your collection given your tastes
- forage prime already exists for token-efficient context injection
- The Claude Code skill already teaches the LLM the CLI commands

## Open questions

- Is forage prime sufficient or should the LLM query the DB directly?
- What's the right UX — CLI command, skill-driven conversation, or both?
