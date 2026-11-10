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
| `not_allowed` | not allowed (kind rules; a worker accepting its own work; promoting an idea, committing a captured item or agreeing a plan without `--on-request`; needs on an item, or a phase needing an item) |
| `dependency_cycle` | the needs would form a cycle |
| `scope_required` | say which plan or item: `--plan` or `--item` (or `--phase`, or `--plan none` for unplanned work, for `add`) |
| `tour_required` | UI files changed and no `--tour` was given |
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

- **Plan:** `{slug, title, state ("shaping"|"active"|"done"), created, updated, summary?}`. A new plan is `shaping`; `plan agree` (the human) makes it `active`, and `plan close` makes it `done`. A shaping plan takes phases, items, captures and inbox entries, but none of its items is ready until it is agreed.
- **Phase:** `{id, plan, title, needs?, from?, created, updated, summary?}`. `from` is the inbox entry (e.g. an idea) it was promoted from. `needs` are the phases it waits for: needs exist between phases only. (A need on an item, from before Model v2, still counts until `mull migrate` converts it.)
- **Item:** `{id, plan?, phase?, title, state ("captured"|"committed"|"active"|"in-review"|"done"|"dropped"), kind?, category?, source?, needs?, docs?, claim?, from?, created, updated, notes?}`. `from` is the inbox entry (idea, question or loose end) it was made from.
  - **No `plan`** means unplanned work, or (captured) backlog not tagged to a plan.
  - **Captured items** (`mull bug`, `mull idea`) carry `kind` (`"bug"`|`"idea"`), and `category` and `source` when given; `notes` holds the `--detail`. They keep them once committed.
  - **claim:** `{worktree, branch, at, base?}`
  - **Items have no needs;** items inside a phase are unordered. An item's `needs`, from before Model v2, is kept and shown until `mull migrate` converts it, but no longer affects readiness.
- **ItemView:** Item plus `{ready, blocked_by?}`. Ready means committed, in a phase whose needs are done (or in no phase), in a plan that isn't shaping; `blocked_by` lists the phase's needs that aren't done.
- **PhaseView:** Phase plus `{progress, complete, blocked_by?}`.
- **Progress:** `{total, captured, committed, active, in_review, done, dropped, ready}`. `total` counts committed work (every stage but captured); captured items tagged to the phase or plan are counted only in `captured`.
- **Entry** (inbox): `{id, kind ("decision"|"question"|"review"|"loose-end"|"idea"), text, plan, item?, source?, by, created, settled?, detail?}`. `plan` is `""` for an entry about the whole project or an unplanned item.
  - **settled:** `{action ("accept"|"decide"|"do"|"drop"|"park"), note?, item?, phase?, by, on_request?, at}`. `item` or `phase` is what `do` made.
  - **Loose ends and inbox ideas** (`kind: "loose-end"`, `"idea"`) are retired: captured items replaced them, and nothing files them any more. Existing ones are kept until `mull migrate` converts them: listed only by `inbox --kind loose-end|idea`, counted in the view's `legacy`, never in `needs_you`, and settled as before (an idea's `text` is its title and `detail` its note).
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
| `add`, `release`, `bug`, `idea`, `commit` | ItemView |
| `show` | `{type: "plan"\|"phase"\|"item"\|"entry", <type>: …}`. A plan is shown as in `plan show`, a phase as PhaseView + `{items}`, an item as ItemView, an entry as Entry |
| `list`, `ready` | `{items: [ItemView]}`. `list --plan none` selects items with no plan; `--kind bug\|idea` selects captured kinds |
| `set` | `{type: "plan"\|"phase"\|"item", plan?: Plan, phase?: PhaseView, item?: ItemView}` |
| `need`, `unneed` | `{type: "phase", phase: PhaseView}`. Only a phase has needs, and only on phases; on an item they fail with `not_allowed` |
| `start` | `{item: ItemView, warnings: []}`. Starting a blocked item, or one in a shaping plan, is allowed with a warning |
| `decided`, `ask` | Entry |
| `inbox` | `{entries: [Entry]}`, oldest first: questions, decisions and reviews. Loose ends and ideas only with `--kind loose-end\|idea`. `--plan none` for entries with no plan |
| `loose`, `ideas` | retired: fail with code `retired` |
| `accept` | `{entries: [Entry], items: [ItemView], follow_up: ItemView\|null}` |
| `decide`, `park` | Entry |
| `do` | `{entry: Entry, item?: ItemView, phase?: PhaseView}`: the new item (or phase, with `--as phase`), or the reopened item for a review |
| `drop` | `{entries: [Entry], items: [ItemView]}` |
| `decisions` | `{decisions: [{id, plan, item?, kind, context, decision, by, on_request?, at}]}`, newest first |
| `config`, `config set\|checks\|layer` | Config |
| `view` | `{generated, integration, integration_found, release, ahead_of_release, plans: [Board], unlinked_commits_this_week, unlinked_recent?, unplanned?: Board, legacy?: {loose_ends, ideas}}`. `unplanned` is a Board (slug `""`, title `"Unplanned"`, no `state`) for items with no plan and entries with no plan, present when there are any. `legacy` counts open loose ends and inbox ideas until `mull migrate`. `plans` holds the active plans, then the shaping ones; a done plan only with `--plan`. Board: `{slug, title, state?, progress, phases: [{id, title, progress, complete, blocked_by?}], unfiled: Progress, problems: [{kind, item, text}], in_flight: [{id, title, state, worktree?, branches?, commits, last_activity?}], needs_you: {reviews, decisions, questions, older_than_a_week, parked}}`. Problem kinds: `idle-claim`, `unclaimed-work`, `done-not-in-git`, `failed-done` |
| `context` | `{worktree, branch, protocol: [], item?: …, my_claims: [], plans: [], ready: [], active: [], decisions: []}`. `plans` are the shaping and active plans: `{slug, title, state, phases?}` |
| `check` | `{tree, commit, at, by, passed, results: [{command, exit, duration_ms, tail?}], cached}` |
| `done` | `{report, report_path, item: ItemView, review_entry?, reminder?}` |
| `report` | `{report, text}`. `text` is the markdown rendering |
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
| `phase add` | `title`, `plan`, `summary`, `needs` |
| `add` | `title`, `plan` (optional with `phase`; `none` for unplanned work), `phase`, `notes`, `doc` |
| `set` | `id`, `field`, `value` |
| `need`, `unneed` | `id` (a phase), `needs` (phases) |
| `decided`, `ask` | `text`, `item`, `plan`, `source`, `detail` |
| `bug`, `idea` | `text` (the title), `plan`, `phase` (both optional), `category`, `source`, `detail` (kept as notes) |
| `commit` | `id`, `plan` (`none` for unplanned), `phase`, `on-request` (required: only the human commits) |
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
