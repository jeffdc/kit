# Triaging the backlog with the human

The backlog is the captured items: bugs and ideas anyone filed with `mull bug` or `mull idea`. They aren't work until the human commits them. Triage is how they get looked at: a conversation you run with the human, using the normal CLI. Once you've gone through an item together, mull stamps it triaged. The view shows only how many are still untriaged.

Everything mull prints is JSON (see `mull guide json`).

## Rules
- **The human decides every outcome.** You read, group and propose. They choose.
- **Writes carry `--on-request`.** Committing, dropping and stamping are the human's, so pass `--on-request` only after the human has said so in the conversation. Retag (`mull set`) only on their word too.
- **Don't do the work during triage.** At most, note what should start now (see below).
- **Don't invent plans or phases.** If something needs a new plan or phase, create it only when the human asks (`mull plan new ... --on-request`, `mull phase add`). A shaping plan can hold tagged captures, but committed work goes into a plan only once the human has agreed it.

## 1. See what's waiting
- `mull view --text`: the BACKLOG line gives the untriaged count and the last triage date, and each board counts its captured items.
- `mull list --untriaged`: every untriaged item. Narrow it with `--plan <slug>`, `--plan none` (tagged to no plan), `--phase <id>` or `--kind bug|idea`.
- `mull list --state captured`: the whole backlog, if the human wants to revisit items triaged before.
- Older loose ends and inbox ideas, from before captured items, aren't in the backlog: `mull inbox --kind loose-end` and `--kind idea` list them, and the view counts them as legacy. If there are any, suggest running `mull migrate --dry-run` with the human first; `mull migrate` turns them into captured items.

## 2. Show the human a short list
Group the items by plan, then by category. For each, give one line: the ID, kind, category, title, source, and the plan or phase it's tagged to. Then add your proposal and a reason, in a few words. Read an item's notes (`mull show <id>`) before proposing anything.

## 3. Settle each item, as the human says
| Outcome | When | Command |
|---|---|---|
| **Commit** | It's work worth doing | `mull commit <id> --on-request`, with `--plan <slug>`, `--phase <id>` or `--plan none` (unplanned) if it moves |
| **Tag** | Not yet work, but it belongs to a plan or phase | `mull set <id> plan <slug>`, `mull set <id> phase <phase-id>` (implies its plan), or `mull set <id> plan none` to untag |
| **Start** | A small fix the human wants done now | the agent that will do it runs `mull start <id>` in its own worktree (start commits it; not in a plan that's still shaping). If that's you, finish triage first: one item per session |
| **Drop** | Not worth keeping, fixed already, or a duplicate | `mull drop <id> --reason "<why>" --on-request` |
| **Leave** | Fine as it is in the backlog | nothing; it's stamped in step 4 |

Tidy as you go, if the human agrees: `mull set <id> title "<clearer title>"` and `mull set <id> category "<category>"`.

## 4. Stamp what you went through
- `mull triage <id...> --on-request` stamps the items still in the backlog that you went through (tagged or left). Committed, started and dropped items have left the backlog and need no stamp; `triage` refuses them.
- `mull triage --all --on-request` stamps the whole backlog, or `--all --plan <slug>` (or `--plan none`) one plan's. Use it only right after listing that backlog with the human: it also stamps anything captured since, unseen.

## Many outcomes at once
Send them as one `mull batch`: all or nothing, one commit. For example:

```
{"op": "commit", "id": "4ce5", "phase": "204f", "on-request": true}
{"op": "set", "id": "ec21", "field": "plan", "value": "taxogall"}
{"op": "set", "id": "ec21", "field": "category", "value": "public pages"}
{"op": "drop", "ids": "546d", "reason": "fixed in 37cb", "on-request": true}
{"op": "triage", "ids": "ec21 9a10", "on-request": true}
```

## Finish
- Tell the human what happened: how many were committed, tagged, dropped and left, and what's still untriaged (`mull view --text`).
- Run `mull sync`.
