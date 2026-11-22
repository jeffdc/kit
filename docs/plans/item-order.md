# Ordering items within a phase

## The picture

Model v2 left items unordered: only phases need other phases, and `mull ready` hands out ready work with no ranking, so an agent picks whatever comes first. In practice Jeff tells an agent "we need X early" or "Y is going to affect Z", and nothing records it. This plan gives items an order within their group so that what Jeff says about sequence and priority is kept and acted on.

What it solves: agents picking the wrong item next, priority within a phase, and light dependencies between items (do Y before Z) that don't warrant splitting a phase. What it doesn't: hard dependencies still belong between phases, and backlog priority is a separate, coarser thing (in the `mull-backlog-ui` plan).

## The shape (agreed with Jeff, 2026-11-16)

- **A rank per item,** a sparse integer (10, 20, 30…). Lower goes first. Equal ranks mean "no order between these".
- **Its group:** a phase, a plan's items with no phase, or Unplanned. Ranks never compare across groups. The ready list across phases goes by the phase schedule first, then rank.
- **Rank sorts, and it's loud; it never holds work back.** `mull ready`, `mull context`, the board and mull serve list open items in rank order. `mull start` on an item while a lower-ranked item in its group isn't done still succeeds, with a warning naming that item. Readiness is unchanged.
- **Agents give an order, not numbers.** `mull rank <group> <id…>` sets the whole order in one commit and mull writes the numbers. It must list every open item in the group (committed, active, in review) and refuses with the missing IDs otherwise. `mull add` puts a new item at the end of its group, and `--before <id>` / `--after <id>` place it. So an agent never reasons about numbers or unranked work.
- **Unranked items** exist only from before this plan. They sort after ranked ones, oldest first, and the first `mull rank` on their group ranks them. No migration.
- **Who:** agents set the order from their conversation with Jeff, without `--on-request`. The protocol says to reorder only when Jeff asks, the same as adding items. A reorder files no decision; mull serve's change cloud shows it.
- **Done and dropped items** keep their rank and show after open work.

## Likely phases (proposed; added once the plan is agreed)

1. **Rank in the model and CLI.** The field, `mull rank`, `add --before/--after` and appending, `set <id> rank`, the batch op, sorting in `ready`, `context`, `list` and `view`, the warning on `start`, and the docs: `design.md` (the Ordering section), `mull guide json` and `mull guide agents`.
2. **Rank in mull serve.** Phase, plan and Unplanned item tables in rank order, and the Plans column and the ready lists to match.
3. **Reordering from mull serve** (may fold into `mull-ui-edit` instead): move an item up or down, saved as Jeff.

## Done when

An agent told "do X first" runs one `mull rank`, the next `mull ready` lists X first, starting something ranked after X warns, and mull serve shows the same order.

## Settled

- **How mull serve shows rank** (Jeff, 2026-11-18): a position number (1, 2, 3, equal ranks sharing one) on open items in a group's table, with done and dropped after them, unnumbered. Reordering is up and down arrows on each open row.

## Open

- Whether `mull rank` takes a partial list (the listed items first, the rest after them in their current order) as a convenience, or stays strict (ed34). It's strict for now; the question is in the inbox.
