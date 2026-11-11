---
kind: decision
text: A group of one decision has a single Approve on its header and no row button; larger groups have Approve all plus a per-decision Approve shown on hover or focus (always on touch screens)
plan: mull-ui-edit
item: 92eb
by: /Users/jeff/dev/kit
created: "2026-11-11T04:03:47Z"
settled:
  action: accept
  note: with review 1b79
  by: /Users/jeff/dev/kit
  on_request: true
  at: "2026-11-11T04:08:30Z"
---

Two buttons doing the same thing on one decision was redundant, and a box on every row turned a group into a column of buttons. Opacity, not visibility, keeps hidden row buttons in the tab order.
