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
| `not_found` | no such plan, epic, item, entry, report or guide |
| `already_exists` | a plan slug or batch ref already used |
| `invalid_state` | the record is in the wrong state (e.g. accepting an item that isn't done) |
| `claimed` | the item is claimed by another worktree |
| `already_settled` | the inbox entry is already settled |
| `not_allowed` | not allowed (kind rules; a worker accepting its own work; promoting an idea without `--on-request`) |
| `dependency_cycle` | the needs would form a cycle |
| `scope_required` | say which plan or item: `--plan` or `--item` (or `--epic` for `add`) |
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
| `internal` | anything else |

A `batch_invalid` error also lists every failed line:

```json
{"code": "batch_invalid", "error": "2 of 40 batch lines failed; nothing was written",
 "lines": [{"line": 7, "op": "add", "ref": "webhook", "code": "not_found", "error": "no epic \"ffff\""}]}
```

## Records

- **Plan:** `{slug, title, state ("active"|"closed"), created, updated, summary?}`
- **Epic:** `{id, plan, title, needs?, from?, created, updated, summary?}`. `from` is the inbox entry (e.g. an idea) it was promoted from.
- **Item:** `{id, plan, epic?, title, state ("open"|"active"|"done"|"accepted"|"dropped"), needs?, docs?, claim?, from?, created, updated, notes?}`. `from` is the inbox entry (idea, question or loose end) it was made from.
  - **claim:** `{worktree, branch, at, base?}`
- **ItemView:** Item plus `{ready, blocked_by?}`.
- **EpicView:** Epic plus `{progress, complete, blocked_by?}`.
- **Progress:** `{total, open, active, done_unreviewed, accepted, dropped, ready}`
- **Entry** (inbox): `{id, kind ("decision"|"question"|"loose-end"|"review"|"idea"), text, plan, item?, source?, by, created, settled?, detail?}`
  - **settled:** `{action ("accept"|"decide"|"do"|"drop"|"park"), note?, item?, epic?, by, on_request?, at}`. `item` or `epic` is what `do` made.
  - **An idea** is an Entry with `kind: "idea"`. `text` is its title and `detail` its note. `plan` may be `""` (no plan). Ideas aren't work: they're never ready, never in `inbox` (unless `--kind idea`) and never in `needs_you`.
- **Config:** `{integration, release, remote, done_checks: [], layers: {name: [globs]}}`

Times are RFC 3339 in UTC. IDs are 4 hex characters; plans use their slug.

## Commands

| command | prints |
|---|---|
| `init` | `{initialized, branch, config: Config}` |
| `version` | `{version}` |
| `plan new`, `plan close` | Plan |
| `plan list` | `{plans: [Plan + {progress}]}` |
| `plan show` | Plan + `{progress, epics: [EpicView], items: [ItemView]}` |
| `epic add` | Epic |
| `epic list` | `{epics: [EpicView]}` |
| `add`, `release` | ItemView |
| `show` | `{type: "plan"\|"epic"\|"item"\|"entry", <type>: …}`. A plan is shown as in `plan show`, an epic as EpicView + `{items}`, an item as ItemView, an entry as Entry |
| `list`, `ready` | `{items: [ItemView]}` |
| `set` | `{type: "plan"\|"epic"\|"item", plan?: Plan, epic?: EpicView, item?: ItemView}` |
| `need`, `unneed` | `{type: "item"\|"epic", item?: ItemView, epic?: EpicView}` |
| `start` | `{item: ItemView, warnings: []}` |
| `decided`, `ask`, `loose`, `idea` | Entry |
| `ideas` | `{ideas: [Entry]}`, oldest first. Open ideas by default; `--plan none` for those with no plan |
| `inbox` | `{entries: [Entry]}`, oldest first. Ideas are left out unless `--kind idea` |
| `accept` | `{entries: [Entry], items: [ItemView], follow_up: ItemView\|null}` |
| `decide`, `park` | Entry |
| `do` | `{entry: Entry, item?: ItemView, epic?: EpicView}`: the new item (or epic, with `--as epic`), or the reopened item for a review |
| `drop` | `{entries: [Entry], items: [ItemView]}` |
| `decisions` | `{decisions: [{id, plan, item?, kind, context, decision, by, on_request?, at}]}`, newest first |
| `config`, `config set\|checks\|layer` | Config |
| `view` | `{generated, integration, integration_found, release, ahead_of_release, plans: [Board], unlinked_commits_this_week, unlinked_recent?, ideas_without_plan}`. Board: `{slug, title, progress, epics: [{id, title, progress, complete, blocked_by?}], unfiled: Progress, problems: [{kind, item, text}], in_flight: [{id, title, state, worktree?, branches?, commits, last_activity?}], needs_you: {reviews, decisions, questions, loose_ends, older_than_a_week, parked}, ideas}`. `ideas` is a count; `mull ideas` lists them. Problem kinds: `idle-claim`, `unclaimed-work`, `done-not-in-git`, `failed-done` |
| `context` | `{worktree, branch, protocol: [], item?: …, my_claims: [], plans: [], ready: [], active: [], decisions: []}` |
| `check` | `{tree, commit, at, by, passed, results: [{command, exit, duration_ms, tail?}], cached}` |
| `done` | `{report, report_path, item: ItemView, review_entry?, loose_ends_filed, reminder?}` |
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
- **`ref`** names the record a line creates. A later line can write `"@name"` in any ID field (`id`, `ids`, `plan`, `epic`, `item`, `needs`) to mean that record.
- **All or nothing.** Every line is validated and run in one transaction. If any line fails, nothing is written, and the error lists every failed line. A batch that passes is a single commit.
- **`--dry-run`** runs every line, writes nothing, and prints the results. IDs in a dry run's results are throwaway.

| op | fields |
|---|---|
| `plan new` | `slug`, `title`, `summary` |
| `plan close` | `slug` |
| `epic add` | `title`, `plan`, `summary`, `needs` |
| `add` | `title`, `plan` (optional with `epic`), `epic`, `needs`, `notes`, `doc` |
| `set` | `id`, `field`, `value` |
| `need`, `unneed` | `id`, `needs` |
| `decided`, `ask`, `loose` | `text`, `item`, `plan`, `source`, `detail` |
| `idea` | `text` (the title), `plan` (optional), `source`, `detail` (the note) |
| `start`, `release` | `id` |
| `accept` | `ids`, `follow-up`, `on-request` |
| `decide` | `id`, `decision`, `on-request` |
| `do` | `id`, `plan`, `epic`, `as` (`item` or `epic`), `reason`, `on-request`. Promoting an idea needs `on-request: true` |
| `drop` | `ids`, `reason`, `on-request` |
| `park` | `id`, `reason`, `on-request` |
| `config set` | `key`, `value` |
| `config checks` | `commands` |
| `config layer` | `name`, `globs` |

`init`, `sync`, `check` and `done` are not batch ops: they reach outside the mull branch.
