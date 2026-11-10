# mull output reference

Every command prints one JSON value to stdout. Most print an object; `batch` prints an array. `view`, `inbox`, `report` and `guide` also have `--text` for a human rendering. Treat the JSON as the contract: the text form may change.

`field?` means the field is left out when it's empty. A list that's always present is written `[]`.

## Errors

On failure, mull prints nothing to stdout, prints this to stderr, and exits 1:

```json
{"code": "not_found", "error": "no item \"ffff\""}
```

`code` is stable: branch on it, not on the message. Codes:

| code | meaning |
|---|---|
| `usage` | bad command line: unknown command or flag, wrong number of arguments |
| `invalid_argument` | a value is malformed or missing |
| `not_a_repo` | not inside a git working tree |
| `not_initialized` | no `mull` branch (run `mull init`, or `mull sync` on a second machine) |
| `already_initialized` | `mull init` where the branch exists |
| `not_found` | no such plan, phase, item, entry, report or guide |
| `already_exists` | a plan slug or batch ref already used |
| `invalid_state` | the record is in the wrong state (e.g. accepting an item that isn't done) |
| `claimed` | the item is claimed by another worktree |
| `already_settled` | the inbox entry is already settled |
| `not_allowed` | not allowed (kind rules; a worker accepting its own work; promoting an idea, committing a captured item, triaging or agreeing a plan without `--on-request`; needs on an item, or a phase needing an item) |
| `dependency_cycle` | the needs would form a cycle |
| `scope_required` | say which plan or item: `--plan` or `--item` (or `--phase`, or `--plan none` for unplanned work, for `add`) |
| `tour_required` | UI files changed and no `--tour` was given |
| `design_required` | `done` was given no design notes (`--design` missing, empty or "none") |
| `no_checks` | no hard checks configured |
| `dirty_tree` | uncommitted or untracked changes |
| `checks_failed` | the hard checks failed |
| `sync_conflict` | the same records changed here and on the remote |
| `push_refused` | the mull branch holds non-mull paths |
| `remote_error` | the remote couldn't be reached or refused the push |
| `batch_invalid` | one or more batch lines failed; nothing was written |
| `invalid_json` | a batch line isn't a JSON object |
| `busy` | too many concurrent writers; try again |
| `git_error` | git failed unexpectedly |
| `retired` | a retired command or batch op (`loose`, `ideas`); the message names its replacement |
| `internal` | anything else |

A `batch_invalid` error also lists every failed line:

```json
{"code": "batch_invalid", "error": "2 of 40 batch lines failed; nothing was written",
 "lines": [{"line": 7, "op": "add", "ref": "webhook", "code": "not_found", "error": "no phase \"ffff\""}]}
```

## Records

- **Plan:** `{slug, title, state ("shaping"|"active"|"done"), created, updated, summary?, doc?}`. A new plan is `shaping`; `plan agree` (the human) makes it `active`, and `plan close` makes it `done`. A shaping plan takes captures and inbox entries but refuses phases and committed items (`invalid_state`) until it is agreed.
- **Phase:** `{id, plan, title, needs?, from?, created, updated, summary?, doc?}`. `from` is the inbox entry (e.g. an idea) it was promoted from. `needs` are the phases it waits for: needs exist between phases only. (A need on an item, from before Model v2, still counts until `mull migrate` converts it.)
- **Docs:** `doc` is a plan's or phase's doc, in markdown: a plan's picture, done-when and why the order; a phase's design and done-when. Docs hold intent only, never status: status comes from the records. A doc is kept apart from its record, so changing the record (`summary` included) never makes a new version of the doc. `plan doc` and `phase doc` print it, replace it (`-`) and list its versions (`--log`).
- **Item:** `{id, plan?, phase?, title, state ("captured"|"committed"|"active"|"in-review"|"done"|"dropped"), kind?, category?, source?, needs?, docs?, claim?, from?, triaged?, created, updated, notes?}`. `from` is the inbox entry (idea, question or loose end) it was made from.
  - **No `plan`** means unplanned work, or (captured) backlog not tagged to a plan.
  - **Captured items** (`mull bug`, `mull idea`) carry `kind` (`"bug"`|`"idea"`), and `category` and `source` when given; `notes` holds the `--detail`. They keep them once committed.
  - **triaged** is when the item was last triaged with the human (`mull triage`). A captured item with no `triaged` is untriaged.
  - **claim:** `{worktree, branch, at, base?}`
  - **Items have no needs;** items inside a phase are unordered. An item's `needs`, from before Model v2, is kept and shown until `mull migrate` converts it, but no longer affects readiness.
- **ItemView:** Item plus `{ready, blocked_by?}`. Ready means committed, in a phase whose needs are done (or in no phase), in a plan that isn't shaping; `blocked_by` lists the phase's needs that aren't done.
- **PhaseView:** Phase plus `{progress, complete, blocked_by?}`.
- **Progress:** `{total, captured, untriaged, committed, active, in_review, done, dropped, ready}`. `total` counts committed work (every stage but captured); captured items tagged to the phase or plan are counted only in `captured`, and `untriaged` counts those of them not triaged yet.
- **Entry** (inbox): `{id, kind ("decision"|"question"|"review"|"loose-end"|"idea"), text, plan, item?, source?, by, created, settled?, decisions?, detail?}`. `plan` is `""` for an entry about the whole project or an unplanned item.
  - **settled:** `{action ("accept"|"decide"|"do"|"drop"|"park"), note?, item?, phase?, by, on_request?, at}`. `item` or `phase` is what `do` made.
  - **decisions** (reviews only): the IDs of the decision entries the review carries: every unsettled, unparked decision on its item when `mull done` filed it. Accepting the review ratifies those still unsettled (settled note `with review <id>`). Decide one differently first with `decide <id>`; sending the review back leaves them open, and the next `done` carries them again.
  - **Loose ends and inbox ideas** (`kind: "loose-end"`, `"idea"`) are retired: captured items replaced them, and nothing files them any more. Existing ones are kept until `mull migrate` converts them: listed only by `inbox --kind loose-end|idea`, counted in the view's `legacy`, never in `needs_you`, and settled as before (an idea's `text` is its title and `detail` its note).
- **Report** (done report): `{item, n, passed, override?, commit, base?, at, by, summary, tour?, checks (as `check` prints it, without `cached`), changes?: [{layer, files: [{status, path, added, removed}]}], design?, decisions?: [{id, text, detail?}]}`. What was built (`summary`, `tour`, `checks`) and how (`design`, `decisions`, `changes`). `design` is the design notes, present on every report since `done` required them; `decisions` are the decisions the review carries, as they were when `done` ran.
- **Config:** `{integration, release, remote, done_checks: [], layers: {name: [globs]}}`

Times are RFC 3339 in UTC. IDs are 4 hex characters; plans use their slug.

## Commands

| command | prints |
|---|---|
| `init` | `{initialized, branch, config: Config}` |
| `version` | `{version}` |
| `plan new`, `plan agree`, `plan close` | Plan |
| `plan list` | `{plans: [Plan + {progress}]}` |
| `plan show` | Plan + `{progress, phases: [PhaseView], items: [ItemView]}` |
| `phase add` | Phase |
| `phase list` | `{phases: [PhaseView]}` |
| `plan doc`, `phase doc` | `{type: "plan"\|"phase", id, doc}`: `id` is the plan's slug or the phase's ID, and `doc` is `""` when there is none. `<slug\|id> -` replaces the doc from stdin, and empty stdin removes it. `--text` prints the doc as markdown. `--log` prints `{type, id, versions: [{commit, at, doc}]}`, newest first: one version per commit that changed the doc, `doc` `""` where it was removed |
| `add`, `release`, `bug`, `idea`, `commit` | ItemView |
| `triage` | `{items: [ItemView]}`: the captured items stamped `triaged`. Needs `--on-request`; given IDs must be captured items |
| `show` | `{type: "plan"\|"phase"\|"item"\|"entry", <type>: …}`. A plan is shown as in `plan show`, a phase as PhaseView + `{items}`, an item as ItemView, an entry as Entry |
| `list`, `ready` | `{items: [ItemView]}`. `list --plan none` selects items with no plan; `--kind bug\|idea` selects captured kinds; `--untriaged` selects captured items not triaged yet |
| `set` | `{type: "plan"\|"phase"\|"item", plan?: Plan, phase?: PhaseView, item?: ItemView}`. Item fields: `title`, `notes`, `category`, `phase`, `doc`, and for a captured item `plan` (`none` untags it; a new plan leaves the old phase behind). A captured item set to a phase in another plan moves to that plan; `plan` on any other item fails with `invalid_state` |
| `need`, `unneed` | `{type: "phase", phase: PhaseView}`. Only a phase has needs, and only on phases; on an item they fail with `not_allowed` |
| `start` | `{item: ItemView, warnings: []}`. Starting a blocked item is allowed with a warning. A captured item tagged to a shaping plan can't be started (`invalid_state`), since starting commits it |
| `decided`, `ask` | Entry |
| `inbox` | `{entries: [Entry]}`, oldest first: questions, decisions and reviews. Loose ends and ideas only with `--kind loose-end\|idea`. `--plan none` for entries with no plan |
| `loose`, `ideas` | retired: fail with code `retired` |
| `accept` | `{entries: [Entry], items: [ItemView], follow_up: ItemView\|null}`. `entries` includes the decisions an accepted review carried and ratified |
| `decide`, `park` | Entry |
| `do` | `{entry: Entry, item?: ItemView, phase?: PhaseView}`: the new item (or phase, with `--as phase`), or the reopened item for a review |
| `drop` | `{entries: [Entry], items: [ItemView]}` |
| `decisions` | `{decisions: [{id, plan, item?, kind, context, decision, by, on_request?, at}]}`, newest first |
| `config`, `config set\|checks\|layer` | Config |
| `view` | `{generated, integration, integration_found, release, ahead_of_release, plans: [Board], untriaged, last_triage?, unlinked_commits_this_week, unlinked_recent?, unplanned?: Board, legacy?: {loose_ends, ideas}}`. `unplanned` is a Board (slug `""`, title `"Unplanned"`, no `state`) for items with no plan and entries with no plan, present when there are any. `untriaged` counts the whole project's untriaged captured items (with `--plan` too), and `last_triage` is the latest `triaged` stamp on any item; each Board's `progress` counts its own. `legacy` counts open loose ends and inbox ideas until `mull migrate`. `plans` holds the active plans, then the shaping ones; a done plan only with `--plan`. Board: `{slug, title, state?, progress, phases: [{id, title, progress, complete, blocked_by?}], unfiled: Progress, problems: [{kind, item, text}], in_flight: [{id, title, state, worktree?, branches?, commits, last_activity?}], needs_you: {reviews, decisions, questions, older_than_a_week, parked}}`. Problem kinds: `idle-claim`, `unclaimed-work`, `done-not-in-git`, `failed-done` |
| `context` | `{worktree, branch, protocol: [], item?: …, my_claims: [], plans: [], ready: [], active: [], decisions: []}`. `plans` are the shaping and active plans: `{slug, title, state, phases?}`. `item` is the ItemView plus `phase_title?`, `phase_summary?`, `phase_doc?` (its phase's doc), `open_questions?`, `item_decisions?`, `last_report?`, `last_report_passed?` |
| `check` | `{tree, commit, at, by, passed, results: [{command, exit, duration_ms, tail?}], cached}` |
| `done` | `{report: Report, report_path, item: ItemView, review_entry?, reminder?}`. `--design` is required; only one of `--summary`, `--tour` and `--design` can be `-` (stdin) |
| `report` | `{report: Report, text}`. `text` is the markdown rendering |
| `sync` | `{remote, action ("up-to-date"\|"pushed"\|"pulled"\|"merged"\|"cloned"), local, remote_before?, pushed_commits, dry_run?, commands?}` |
| `guide` | `{guide, text}` |
| `batch` | `[{line, op, ref?, id?, result}]`, one per input line in order. `result` is exactly what that op's command prints, and `id` is what `@ref` names |

## Batch

`mull batch` reads JSON Lines on stdin (or from a file), one op per line:

```
{"op": "<op>", "ref": "<optional name>", ...fields}
```

- **Field names are the command's flag names exactly.** Positional arguments use the names below. List fields take an array, or one string of IDs separated by commas or spaces.
- **`ref`** names the record a line creates. A later line can write `"@name"` in any ID field (`id`, `ids`, `plan`, `phase`, `item`, `needs`) to mean that record.
- **All or nothing.** Every line is validated and run in one transaction. If any line fails, nothing is written, and the error lists every failed line. A batch that passes is a single commit.
- **`--dry-run`** runs every line, writes nothing, and prints the results. IDs in a dry run's results are throwaway.

| op | fields |
|---|---|
| `plan new` | `slug`, `title`, `summary` |
| `plan agree` | `slug`, `on-request` (required: only the human agrees a plan) |
| `plan close` | `slug` |
| `plan doc` | `slug`, `doc` (required: the whole new doc; `""` removes it) |
| `phase add` | `title`, `plan`, `summary`, `needs` |
| `phase doc` | `id`, `doc` (required: the whole new doc; `""` removes it) |
| `add` | `title`, `plan` (optional with `phase`; `none` for unplanned work), `phase`, `notes`, `doc` |
| `set` | `id`, `field`, `value` |
| `need`, `unneed` | `id` (a phase), `needs` (phases) |
| `decided`, `ask` | `text`, `item`, `plan`, `source`, `detail` |
| `bug`, `idea` | `text` (the title), `plan`, `phase` (both optional), `category`, `source`, `detail` (kept as notes) |
| `commit` | `id`, `plan` (`none` for unplanned), `phase`, `on-request` (required: only the human commits) |
| `triage` | `ids`, or `all` (`true`) with an optional `plan` (`none` for untagged captures), `on-request` (required: triage is run with the human) |
| `start`, `release` | `id` |
| `accept` | `ids`, `follow-up`, `on-request` |
| `decide` | `id`, `decision`, `on-request` |
| `do` | `id`, `plan`, `phase`, `as` (`item` or `phase`), `reason`, `on-request`. Promoting an idea needs `on-request: true` |
| `loose` | retired: the line fails with code `retired` |
| `drop` | `ids`, `reason`, `on-request` |
| `park` | `id`, `reason`, `on-request` |
| `config set` | `key`, `value` |
| `config checks` | `commands` |
| `config layer` | `name`, `globs` |

`init`, `sync`, `check` and `done` are not batch ops: they reach outside the mull branch.
