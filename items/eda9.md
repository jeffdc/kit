---
plan: mull-ui-edit
phase: "7195"
title: Audit every action in mull serve that copies a command instead of saving, then convert each of Jeff's own steps into a real write
stage: in-review
claim:
  worktree: /Users/jeff/dev/kit/.claude/worktrees/agent-adf0fdbb087d1a707
  branch: eda9-real-writes
  at: "2026-11-12T00:36:57Z"
  base: 8274523c54be658ec9d48f55d15f4f2d1a5c5dd4
created: "2026-11-12T00:05:13Z"
updated: "2026-11-12T00:54:25Z"
---

Jeff's words, 2026-11-11: Drop in backlog is not an action, just a copy. This is getting frustrating as I had thought we agreed earlier that we convert all actions to real actions, not copying of commands. It needs to start with an audit of all actions that are NOT true actions, and then make sure we convert them all.

Start with the audit: list every button or link that copies (data-cmd, data-copy) with where it is and what it does, and show it to Jeff before converting. A first look found: on a captured item's sheet, Start it now, Commit and Drop (item.js); Agree this plan (plan.js, bug d1c5); Capture a bug or idea in the title block (title.js; overlaps mull-backlog-ui's add-new-items idea ffe0); and the agent prompts in Needs you (Discuss with an agent, Copy all N for an agent; needs.js). The plan doc keeps steps that hand work to an agent (Start, agent prompts) as copies: confirm that with Jeff in the audit rather than assume it. Each conversion adds its op to uiOps and uses writeButton or textWrite (DESIGN.md's Two Kinds of Action Rule).

Jeff's answers to the audit, 2026-11-11: convert all of them in this item, except these, which stay copies because they hand work to an agent: Start it now, Discuss with an agent, Copy all N for an agent. Remove the captured item sheet's Commit to a plan button entirely: committing and triage always happen with an agent, and Jeff doesn't build plans by hand, so no plan/phase picker and no Mark triaged button. So: Drop on a captured item (with a reason), Agree this plan (bug d1c5), and Capture a bug or idea in the title block (a form: title, detail, bug or idea; overlaps ffe0) become real writes. Update the command reference (model.js) to match. The backlog is the main place Jeff quickly adds a bug or idea before an agent is involved, so Capture must be quick.
