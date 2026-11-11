mull serve becomes a place the human acts, not only reads. Every step that is the human's (approve, decide, answer, drop, park, accept or send back a review, commit and triage the backlog, agree and close a plan) can be taken from the page, and is recorded as the human's own. Steps that hand work to an agent (Start, the agent prompts) stay copy-to-clipboard. Editing plan and phase docs is left for later.

Done when the human can work through Needs you, the backlog and their plans from the UI without copying a command.

## Foundations (all in the first phase)

- **One write path.** The server exposes the existing op registry (the ops `mull batch` runs) behind one endpoint, `POST /api/projects/{id}/ops`: one commit per request, all or nothing, and only the ops the UI offers. Every edit is the same op the CLI runs; no mull logic is reimplemented in the server.
- **Made by the human.** A write from the UI is the human's, by name, not a worktree's and not an agent's on request. The model gains a way to record that, and the human-only ops (commit, triage, agree…) accept it without `--on-request`. The name is the repository's git user.name, and the server refuses writes when it isn't set.
- **No acting on a stale page.** Each write carries the revision the page showed. If a record it touches changed since, the server refuses with a conflict and the page redraws instead of acting.
- **Acting acknowledges.** Records the human acts on stop being clouded, up to and including the write itself; everything else stays clouded until Mark revision seen. The per-machine last look gains the acknowledged records beside its commit.
- **Two kinds of action.** DESIGN.md's rule that nothing in the UI saves anything is replaced by a vocabulary of two kinds: copy actions (as today) and write actions, which look different, show a pending state, confirm in place before anything that can't be undone (mull has no undo), take text in place (reasons, decisions, answers), and show failures plainly.

## Phases, and why this order

1. **Write foundation and approving decisions.** The foundations above, then Approve all and approving one decision. First because approving decisions is the human's biggest need today, and it exercises every foundation with the simplest edit (no text, no pickers).
2. **Settling the inbox.** Decide differently, answer a question, drop or park with a reason, accept or send back a review. Adds text input and the irreversible confirmation.
3. **The backlog.** Commit a captured item to a plan or phase, drop it, stamp triage. Adds pickers.
4. **Plans.** Agree a plan, change plan and phase titles and summaries, close a plan.

Out of scope: anything an agent does (start, done, claims), configuration, doc editing, and `mull sync` (UI writes are local commits; pushing stays the human's).
