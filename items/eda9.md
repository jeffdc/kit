---
plan: mull-ui-edit
title: Audit every action in mull serve that copies a command instead of saving, then convert each of Jeff's own steps into a real write
stage: committed
created: "2026-11-12T00:05:13Z"
updated: "2026-11-12T00:05:13Z"
---

Jeff's words, 2026-11-11: Drop in backlog is not an action, just a copy. This is getting frustrating as I had thought we agreed earlier that we convert all actions to real actions, not copying of commands. It needs to start with an audit of all actions that are NOT true actions, and then make sure we convert them all.

Start with the audit: list every button or link that copies (data-cmd, data-copy) with where it is and what it does, and show it to Jeff before converting. A first look found: on a captured item's sheet, Start it now, Commit and Drop (item.js); Agree this plan (plan.js, bug d1c5); Capture a bug or idea in the title block (title.js; overlaps mull-backlog-ui's add-new-items idea ffe0); and the agent prompts in Needs you (Discuss with an agent, Copy all N for an agent; needs.js). The plan doc keeps steps that hand work to an agent (Start, agent prompts) as copies: confirm that with Jeff in the audit rather than assume it. Each conversion adds its op to uiOps and uses writeButton or textWrite (DESIGN.md's Two Kinds of Action Rule).
