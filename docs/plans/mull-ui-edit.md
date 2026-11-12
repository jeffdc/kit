mull serve becomes a place Jeff acts, not only reads. Every step that is Jeff's (approve, decide, answer, drop, park, accept or send back a review, capture a bug or idea, edit an item's text and plan and phase docs, agree and close a plan) can be taken from the page, and is recorded as Jeff's own. Steps that hand work to an agent (Start, the agent prompts) stay copy-to-clipboard, and any ID can be copied with one click to hand to an agent. Triage stays with Jeff and an agent: committing captured items to plans and phases, and building plans from them, isn't done by hand on the page.

Done when Jeff can work through Needs you, the backlog and the plans from the UI without copying a command.

## Foundations (all in the first phase)

- **One write path.** The server exposes the existing op registry (the ops `mull batch` runs) behind one endpoint, `POST /api/projects/{id}/ops`: one commit per request, all or nothing, and only the ops the UI offers. Every edit is the same op the CLI runs; no mull logic is reimplemented in the server.
- **Made by Jeff.** A write from the UI is Jeff's, by name, not a worktree's and not an agent's on request. The model records that, and the ops only Jeff may run (commit, triage, agree…) accept it without `--on-request`. The name is the one set on this machine with `mull name`, and the server refuses writes when it isn't set.
- **No acting on a stale page.** Each write carries the revision the page showed. If a record it touches changed since, the server refuses with a conflict and the page redraws instead of acting.
- **Acting acknowledges.** Records Jeff acts on stop being clouded, up to and including the write itself; everything else stays clouded until Mark revision seen. The per-machine last look keeps the acknowledged records beside its commit.
- **Two kinds of action.** DESIGN.md's rule that nothing in the UI saves anything is replaced by a vocabulary of two kinds: copy actions (as before) and write actions, which look different, show a pending state, confirm in place before anything that can't be undone (mull has no undo), take text in place (reasons, decisions, answers), and show failures plainly.

## Phases, and why this order

1. **Write foundation and approving decisions.** The foundations above, then Approve all and approving one decision. First because approving decisions is Jeff's biggest need, and it exercises every foundation with the simplest edit (no text, no pickers).
2. **Settling the inbox.** Decide differently, answer a question, drop or park with a reason, accept or send back a review. Adds text input and the irreversible confirmation.
3. **The backlog.** Where Jeff writes before an agent is involved: capture a bug or idea quickly, edit an item's title and notes and plan and phase docs in place, drop a captured item, and copy any ID in one click. Every remaining copy action that was Jeff's own step becomes a write. No triage on the page and no plan or phase pickers.
4. **Plans.** Agree a plan, change plan and phase titles and summaries, close a plan.

Out of scope: anything an agent does (start, done, claims), triage (committing captured items and stamping triage, which Jeff does with an agent), configuration, and `mull sync` (UI writes are local commits; pushing stays Jeff's).
