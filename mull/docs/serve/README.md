# mull serve: design

The human's own view of mull: a local web UI across every repo, live as agents write. Worked out with the human on 2026-11-09 and 10, and tracked as the mull plan `mull-serve`.

- [PRODUCT.md](PRODUCT.md): who it's for, what it must answer, and the principles.
- [direction.md](direction.md): the visual direction ("Drawing Set") and its contract.
- [prototype.html](prototype.html): the clickable prototype, with real gallformers and kit records reshaped to the model below. Open it in a browser. States marked "ex" are invented to show a screen.

## The model (Model v2)

```
Project            a repo
├─ Needs you       questions · decisions · reviews (attention only, ranked)
├─ Plan            shaping → active → done; "agreed" is explicit, by the human
│  │  doc          the picture, done-when, why the order; intent only, never status
│  └─ Phase        doc: its design + done-when
│     │  needs     other phases only ("go live" needs X, Y, Z)
│     └─ Item      one session; no needs, no ordering
├─ Unplanned       items committed without a plan
└─ Backlog         captured items: bug | idea, free-text category,
                   optionally tagged to a plan or phase
```

**One item, stages instead of steps:** captured → committed → active → in review → done, or dropped from anywhere. `start` works from captured or committed, so a small bug goes from the backlog to done in one session with no planning step. Review is always required: "done" means reviewed.

**Rules:**
- **No status in prose.** Docs hold intent and reasons; status comes from records and shows beside the doc.
- **Review is what and how.** What was built (summary, tour, checks) and how (design notes, the decisions made in the session, files by layer), settled together.
- **Order only where it matters.** Needs exist between phases only. "Can start now" is derived.
- **Needs you is attention only.** Questions, decisions, reviews. Bugs and ideas go to the backlog and never nag.
- **Triage is a process,** run with an agent. mull stamps a triaged time; the UI shows only the untriaged count.
- **Anyone captures; the human commits.**
- **What changed since you last looked is clouded in red.** No delta triangles or counts.

**Needs you ranking:** blocking work (a question on committed or active work) → waiting for review → questions for the phases that can start now → questions for later phases → questions about the whole plan → decisions to approve, grouped by source.

## Decisions (2026-11-10)

- "Epic" is renamed "phase" everywhere.
- `mull serve` is a Go server inside mull that imports its own packages, listens on `127.0.0.1` at a fixed port, and serves plain JS modules and CSS embedded in the binary (no Node build step). Live updates come over SSE from a watch on each repo's `refs/heads/mull`.
- Repos are found with no configuration: any `mull` command registers its repo in `~/.config/mull/repos`.
- "Last looked" is kept by `mull serve` in `~/.config/mull/`, per machine, never on the mull branch.
- `http://localhost` is fine; no HTTPS needed.
- Read-first: the UI shows the mull command to run for an action until the flow and model are settled; actions in the UI come later.
- Fonts (Archivo, Red Hat Mono) are self-hosted by `mull serve`; the prototype loads them from Google Fonts only because its sandbox allows nothing else.
