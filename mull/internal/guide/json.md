# mull output reference

Every command prints one JSON value to stdout. Most print an object; `batch` prints an array. `view`, `inbox`, `report`, `guide`, `migrate`, `plan doc`, `phase doc`, `plan rules` and `phase rules` also have `--text` for a human rendering. Treat the JSON as the contract: the text form may change.

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
| `not_allowed` | not allowed (kind rules; a worker accepting its own work; promoting an idea, committing a captured item, triaging, creating a plan, agreeing one, migrating or recording the human's own decision (`decided --human`) without `--on-request`; a `mull serve` write with no `mull name` or git `user.name`, or of an op its pages don't offer; needs on an item, or a phase needing an item) |
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
| `conflict` | a write from a `mull serve` page touches records that changed since the page showed them; nothing was written |
| `git_error` | git failed unexpectedly |
| `retired` | a retired command or batch op (`loose`, `ideas`); the message names its replacement |
| `migration_required` | a write to data from before Model v2 (format 1), or to data holding records still in format 1 (merged in by `sync` from an older mull): the human runs `mull migrate --dry-run`, then `mull migrate --on-request`. Reads still work |
| `unsupported_format` | the data is in a newer format than this mull understands: update mull. Reads fail too |
| `port_in_use` | `mull serve`'s port is taken, most likely by another `mull serve`: pass `--port` to use another |
| `launchd_error` | `mull serve --install` couldn't load the LaunchAgent: the message carries launchctl's output |
| `internal` | anything else |

A `batch_invalid` error also lists every failed line:

```json
{"code": "batch_invalid", "error": "2 of 40 batch lines failed; nothing was written",
 "lines": [{"line": 7, "op": "add", "ref": "webhook", "code": "not_found", "error": "no phase \"ffff\""}]}
```

## Records

- **Plan:** `{slug, title, state ("shaping"|"active"|"done"), created, updated, summary?, target?, doc?, rules?}`. A new plan is `shaping`; `plan agree` (the human) makes it `active`, and `plan close` makes it `done`. `target` is the branch its work lands on (`plan set <slug> target <branch>`); absent, it lands on config's `integration`. A shaping plan takes captures and inbox entries but refuses phases and committed items (`invalid_state`) until it is agreed.
- **Phase:** `{id, plan, title, needs?, from?, created, updated, summary?, doc?, rules?}`. `from` is the inbox entry (e.g. an idea) it was promoted from. `needs` are the phases it waits for: needs exist between phases only. (A need on an item, from before Model v2, still counts until `mull migrate` makes it a need on that item's phase.)
- **Docs:** `doc` is a plan's or phase's doc, in markdown: a plan's picture, done-when and why the order; a phase's design and done-when. Docs hold intent only, never status: status comes from the records. A doc is kept apart from its record, so changing the record (`summary` included) never makes a new version of the doc. `plan doc` and `phase doc` print it, replace it (`-`) and list its versions (`--log`).
- **Rules:** `rules` are a plan's or phase's standing rules for the agents working in it, in markdown: the human's, like docs (how to set up a worktree, what to use, what not to touch, skills to load). `context --item` gives an item's plan's rules, then its phase's, in full. They're kept apart from the record and the doc, each in its own file, and `plan rules` and `phase rules` print, replace (`-`) and list them (`--log`) as `plan doc` does a doc.
- **Item:** `{id, plan?, phase?, rank?, title, state ("captured"|"committed"|"active"|"in-review"|"done"|"dropped"), kind?, category?, source?, needs?, docs?, claim?, from?, triaged?, created, updated, notes?}`. `from` is the inbox entry (idea, question or loose end) it was made from, or the done item it follows up (`add --follow-up`, `accept --follow-up`). `rank` is its order in its group, lower first (see Rank).
  - **No `plan`** means unplanned work, or (captured) backlog not tagged to a plan.
  - **Captured items** (`mull bug`, `mull idea`) carry `kind` (`"bug"`|`"idea"`), and `category` and `source` when given; `notes` holds the `--detail`. They keep them once committed.
  - **triaged** is when the item was last triaged with the human (`mull triage`). A captured item with no `triaged` is untriaged.
  - **claim:** `{worktree, branch, at, base?}`
  - **Items have no needs.** An item's `needs`, from before Model v2, is kept and shown until `mull migrate` drops it, but no longer affects readiness.
- **Rank:** the order of open work (committed, active, in review) within its group: a phase, a plan's items with no phase, or Unplanned. It sorts and never affects readiness. Agents never write the numbers: `mull rank` sets a group's order, and mull writes ranks a step of 10 apart. Committed work joins its group last (`add`, `commit`, `start` of a captured item, `set <id> phase` to another group), or just `--before`/`--after` an open item in the group; when there's no gap there, the group's open work is renumbered in order. Captured items have no rank, and done and dropped items keep theirs. An item without a rank (from before ranks) sorts after ranked ones, oldest first, until its group is next ranked or joined. **Work order** is how `list`, `ready`, `context`, `view` (`in_flight`), `show` and `plan show` list items: plan by plan (by slug, Unplanned last); within a plan, its phases in schedule order (by depth of needs not yet done, then age; done phases last), then its items with no phase; within a group, open work in rank order, then the rest (captured, done, dropped) by rank, unranked last.
- **Target:** `{branch, from ("plan"|"integration"), found, ahead_of_release}`: where a plan's work lands, and where seen in git is measured. `from` is `"plan"` when the plan sets its own `target` and `"integration"` when it falls back to config's (as unplanned work always does). `found` is `false` when the branch doesn't exist; `ahead_of_release` counts its commits the release branch doesn't have (0 when it is the release branch).
- **ItemView:** Item plus `{ready, blocked_by?}`. Ready means committed, in a phase whose needs are done (or in no phase), in a plan that isn't shaping; `blocked_by` lists the phase's needs that aren't done.
- **PhaseView:** Phase plus `{progress, complete, blocked_by?}`.
- **Progress:** `{total, captured, untriaged, committed, active, in_review, done, dropped, ready}`. `total` counts committed work (every stage but captured); captured items tagged to the phase or plan are counted only in `captured`, and `untriaged` counts those of them not triaged yet.
- **Entry** (inbox): `{id, kind ("decision"|"question"|"review"|"loose-end"|"idea"), text, plan, item?, source?, by, created, settled?, decisions?, detail?}`. `plan` is `""` for an entry about the whole project or an unplanned item.
  - **settled:** `{action ("accept"|"decide"|"do"|"drop"|"park"), note?, item?, phase?, by, human?, on_request?, at}`. `item` or `phase` is what `do` made. `by` is the worktree that settled it; `human` is `true` when the human settled it themselves (from `mull serve`), and then `by` is their name (`mull name` on that machine, or else the repository's git `user.name`).
  - **decisions** (reviews only): the IDs of the decision entries the review carries: every unsettled, unparked decision on its item when `mull done` filed it. Accepting the review ratifies those still unsettled (settled note `with review <id>`). Decide one differently first with `decide <id>`; sending the review back leaves them open, and the next `done` carries them again.
  - **Loose ends and inbox ideas** (`kind: "loose-end"`, `"idea"`) are retired: captured items replaced them, and nothing files them any more. Existing ones are kept until `mull migrate` turns them into captured ideas: listed only by `inbox --kind loose-end|idea`, counted in the view's `legacy`, never in `needs_you`, and settled as before (an idea's `text` is its title and `detail` its note).
- **Report** (done report): `{item, n, passed, override?, commit, base?, at, by, summary, tour?, checks (as `check` prints it, without `cached`), changes?: [{layer, files: [{status, path, added, removed}]}], design?, decisions?: [{id, text, detail?}]}`. What was built (`summary`, `tour`, `checks`) and how (`design`, `decisions`, `changes`). `design` is the design notes, present on every report since `done` required them; `decisions` are the decisions the review carries, as they were when `done` ran.
- **Config:** `{format, integration, release, remote, done_checks: [], layers: {name: [globs]}}`. `format` is the data's storage format: `2` (Model v2) once migrated, and in every repository `mull init` made since; `1` for data from before Model v2, which mull reads but won't write until `mull migrate`.

Times are RFC 3339 in UTC. IDs are 4 hex characters; plans use their slug.

## Commands

| command | prints |
|---|---|
| `init` | `{initialized, branch, config: Config}` |
| `version` | `{version}` |
| `name` | `{name, file}`: the name this machine's human goes by (`""` when unset) and the file it's kept in. `name <name>` sets it, and `name ""` clears it. It's per machine, never on the mull branch. |
| `plan new`, `plan agree`, `plan close` | Plan |
| `plan set` | As `set`, for a plan only (`not_found` for anything else): `<slug> title\|summary\|target <value>`. `target` takes a branch name (a leading `refs/heads/` is dropped; `""` clears it); one git refuses, or `mull`, fails with `invalid_argument`. The branch needn't exist yet |
| `plan list` | `{plans: [Plan + {progress}]}` |
| `plan show` | Plan + `{progress, phases: [PhaseView], items: [ItemView]}` |
| `phase add` | Phase |
| `phase delete` | `{phase, untagged, unneeded}`: `phase` is the deleted Phase as it was, `untagged` the captured items moved up to its plan and `unneeded` the phases whose need on it was removed. Only a phase with no committed work in any stage (committed, active, in review, done or dropped) can be deleted: otherwise `invalid_state`, naming the items. Only the human deletes a phase (`not_allowed` without `--on-request`). Its record, doc and rules are removed from the mull branch, whose history still has them |
| `phase list` | `{phases: [PhaseView]}` |
| `plan rules`, `phase rules` | `{type: "plan"\|"phase", id, rules}`, as `plan doc` and `phase doc` with `rules` for `doc`: `""` when there are none, `-` replaces them from stdin (empty removes them), `--text` prints them as markdown, and `--log` prints `{type, id, versions: [{commit, at, rules}]}` |
| `plan doc`, `phase doc` | `{type: "plan"\|"phase", id, doc}`: `id` is the plan's slug or the phase's ID, and `doc` is `""` when there is none. `<slug\|id> -` replaces the doc from stdin, and empty stdin removes it. `--text` prints the doc as markdown. `--log` prints `{type, id, versions: [{commit, at, doc}]}`, newest first: one version per commit that changed the doc, `doc` `""` where it was removed |
| `add`, `release`, `commit` | ItemView |
| `bug`, `idea` | ItemView + `{similar: [{id, title}]}`: the captured item, and up to five open captured or committed items (most similar first, `[]` when none) whose titles overlap its own a lot in their words, rarer words counting for more. A hint that it may be a near-duplicate, never a block: the capture is written either way |
| `triage` | `{items: [ItemView]}`: the captured items stamped `triaged`. Needs `--on-request`; given IDs must be captured items |
| `show` | `{type: "plan"\|"phase"\|"item"\|"entry", <type>: …}`. A plan is shown as in `plan show`, a phase as PhaseView + `{items}`, an item as ItemView, an entry as Entry |
| `list`, `ready` | `{items: [ItemView]}`, in work order (see Rank). `list --plan none` selects items with no plan; `--kind bug\|idea` selects captured kinds; `--untriaged` selects captured items not triaged yet |
| `set` | `{type: "plan"\|"phase"\|"item", plan?: Plan, phase?: PhaseView, item?: ItemView}`. Plan fields: `title`, `summary`, `target` (see `plan set`). Phase fields: `title`, `summary`. Item fields: `title`, `notes`, `category`, `phase`, `doc`, and for a captured item `plan` (`none` untags it; a new plan leaves the old phase behind) and `kind` (`bug` or `idea`). A captured item set to a phase in another plan moves to that plan; `plan` or `kind` on any other item fails with `invalid_state` |
| `need`, `unneed` | `{type: "phase", phase: PhaseView}`. Only a phase has needs, and only on phases; on an item they fail with `not_allowed` |
| `rank` | `{plan?, phase?, items: [{id, title, state, rank}]}`: the group (neither for Unplanned) and its open work in the new order. `mull rank <phase\|plan\|none> <ids...>` names the group by phase ID, plan slug (its items with no phase) or `none` (Unplanned), and must list every open item in it, first to last: a missing one, or one that isn't open work in the group, fails with `invalid_argument` naming it |
| `start` | `{item: ItemView, warnings: []}`. Starting a blocked item is allowed with a warning, and so is starting committed work while open work ranked before it in its group isn't done: the warning names those items, with their stage. Equal ranks and unranked items have no order, so they don't warn, and nor does a captured item started now (it joins its group last, unranked by anyone). A captured item tagged to a shaping plan can't be started (`invalid_state`), since starting commits it |
| `decided`, `ask` | Entry. `decided --human --on-request` (the human's own decision) returns it already settled: `settled` is `{action: "decide", note: <the text>, by: <worktree>, on_request: true, at}`, so it's in `decisions` at once and never in `inbox`. `--on-request` without `--human` is `invalid_argument` |
| `inbox` | `{entries: [Entry]}`, oldest first: questions, decisions and reviews. Loose ends and ideas only with `--kind loose-end\|idea`. `--plan none` for entries with no plan |
| `loose`, `ideas` | retired: fail with code `retired` |
| `accept` | `{entries: [Entry], items: [ItemView], follow_up: ItemView\|null}`. `entries` includes the decisions an accepted review carried and ratified |
| `decide`, `park` | Entry |
| `do` | `{entry: Entry, item?: ItemView, phase?: PhaseView}`: the new item (or phase, with `--as phase`), or the reopened item for a review |
| `drop` | `{entries: [Entry], items: [ItemView]}` |
| `decisions` | `{decisions: [{id, plan, item?, kind, context, decision, by, human?, on_request?, at}]}`, newest first; `by` and `human` as in an entry's `settled` |
| `config`, `config set\|checks\|layer` | Config |
| `view` | `{generated, integration, integration_found, release, ahead_of_release, plans: [Board], untriaged, last_triage?, unlinked_commits_this_week, unlinked_recent?, unplanned?: Board, legacy?: {loose_ends, ideas}, format, migration_required?}`. `format` is the data's storage format (as in Config); `migration_required` is `true` while writes are refused until `mull migrate`. `unplanned` is a Board (slug `""`, title `"Unplanned"`, no `state`) for items with no plan and entries with no plan, present when there are any. `untriaged` counts the whole project's untriaged captured items (with `--plan` too), and `last_triage` is the latest `triaged` stamp on any item; each Board's `progress` counts its own. `legacy` counts open loose ends and inbox ideas until `mull migrate`. `plans` holds the active plans, then the shaping ones; a done plan only with `--plan`. Board: `{slug, title, state?, target: Target, progress, phases: [{id, title, progress, complete, blocked_by?}], unfiled: Progress, problems: [{kind, item, text}], in_flight: [{id, title, state, worktree?, branches?, commits, last_activity?}], needs_you: {reviews, decisions, questions, older_than_a_week, parked}}`. Problem kinds: `idle-claim`, `unclaimed-work`, `done-not-in-git` (nothing on its plan's target: in review for three days, or done (accepted) at once), `failed-done`, `work-after-accept` (commits carrying a done item's trailer, on any branch, written after it was accepted: shown while the latest is under a week old and no follow-up of it has been added since). The top-level `integration`, `integration_found` and `ahead_of_release` are config's integration branch; each Board's `target` is its plan's (integration for `unplanned`) |
| `context` | `{worktree, branch, protocol: [], item?: …, my_claims: [], plans: [], ready: [], active: [], decisions: []}`. While writes wait for `mull migrate`, `protocol` starts with a line saying so. `plans` are the shaping and active plans: `{slug, title, state, target: Target, phases?}`. `ready` (at most 15) and `active` are in work order (see Rank), so the ready list goes by phase schedule, then rank. `item` is the ItemView plus `phase_title?`, `phase_summary?`, `phase_doc?` (its phase's doc), `plan_rules?` and `phase_rules?` (its plan's and its phase's rules, in full; when either is there, `protocol` ends with a line saying to follow them, the plan's then the phase's), `open_questions?`, `item_decisions?`, `last_report?`, `last_report_passed?` |
| `check` | `{tree, commit, at, by, passed, results: [{command, exit, duration_ms, tail?}], cached}` |
| `done` | `{report: Report, report_path, item: ItemView, review_entry?, reminder?}`. `--design` is required; only one of `--summary`, `--tour` and `--design` can be `-` (stdin) |
| `report` | `{report: Report, text}`. `text` is the markdown rendering |
| `migrate` | `{dry_run, format_before, format, changed, commit?, counts: {plans, phases, items, entries, docs, captured, needs}, records: [{type, id, path, from?, changes: []}], captured: [{item, kind, title, plan?, entry, entry_kind, was}], needs: [{type, id, need, action, to?, reason}], warnings: []}`. See "Migrating to Model v2" below. `records` are the existing records it changes (`type` `"config"`\|`"plan"`\|`"phase"`\|`"item"`\|`"entry"`; `path` where it's stored after, `from` where it was when it moves), with each change as a line of text; `captured` are the loose ends and inbox ideas it makes captured items (`kind` `"idea"`, `was` `"open"`\|`"parked"`); `needs` are the needs it converts (`action` `"converted"`, `to` the phase needed now) or drops (`"dropped"`). `changed` is `false` when there is nothing to do; `commit` is the mull branch after a real run that wrote. `--text` renders it for the human |
| `sync` | `{remote, action ("up-to-date"\|"pushed"\|"pulled"\|"merged"\|"cloned"), local, remote_before?, pushed_commits, dry_run?, commands?}` |
| `guide` | `{guide, text}` |
| `serve` | `{url, registry, repos: []}` once it is listening, then serves until interrupted: `registry` is the registry file and `repos` the paths in it. With `--install` (macOS): `{label, plist, log, url, program: [], warnings: []}` once the LaunchAgent is written and loaded (`program` is the command it runs; a warning says the port is already in use). With `--uninstall`: `{label, plist, removed}`. See "mull serve" below |
| `batch` | `[{line, op, ref?, id?, result}]`, one per input line in order. `result` is exactly what that op's command prints, and `id` is what `@ref` names |

## mull serve

`mull serve` is the human's view: a web UI of every repository on mull, on `127.0.0.1` only (port 6855 unless `--port`). It writes mull data only as the human, through `POST /api/projects/{id}/ops` and only the ops its pages offer; otherwise the only thing it writes is its own last-looked state. On macOS, `mull serve --install` runs it at login as a LaunchAgent (`~/Library/LaunchAgents/mull.serve.plist`, logging to `~/Library/Logs/mull-serve.log`), restarted if it stops, with the PATH and XDG_CONFIG_HOME it was installed with; run it again after upgrading mull, and `--uninstall` to remove it.

Repositories are found with no configuration. After every command, mull adds the repository it ran in to the registry, if it has a mull branch: `~/.config/mull/repos` (or `$XDG_CONFIG_HOME/mull/repos`), one main-worktree path per line, kept per machine and never on the mull branch. Lines can be removed by hand; blank lines and lines starting with `#` are skipped.

What changed since the human last looked is clouded in the UI. The last look is kept per machine, never on the mull branch: `~/.config/mull/seen.json` (beside the registry), `{repos: {<path>: {commit, at, acked?: [{type, id, commit}]}}}`, the mull branch commit last marked seen and when, and the records the human has acted on from the page since, each with the commit that acted; replaced atomically on every write. Marking seen empties `acked`. The first time `mull serve` reads a repository (or when its last look names a commit the repository doesn't have), the tip becomes the baseline, so only changes from then on are clouded. A revision is `{commit, number, at}`: `number` counts the commits on the mull branch up to `commit`, so it grows with every write, and `at` is its commit time. A Change is one record that differs between the last look and the tip, its records compared as read (so a change of storage format alone, as `mull migrate` makes, is no change): `{type ("plan"|"plan_doc"|"plan_rules"|"phase"|"phase_doc"|"phase_rules"|"item"|"entry"), id, title, plan?, phase?, kind?, change ("added"|"changed"|"removed"), state?, fields?: [{field, from?, to?}], at}`. A doc is its own record, and so are rules (`id` is its plan's slug or phase's ID, `title` its plan's or phase's). `state` is the record's state now: an item's stage, a plan's state, an entry's settle action or `"open"`. `fields` lists what changed on a changed record, with `from` and `to` for short fields (`state`, `title`, `plan`, `phase`, `claim` (the worktree), `settled`, `needs` (comma-separated)…) but not for text (`notes`, `summary`, `text`, `detail`). `at` is the newest commit since the last look that touched it; changes are listed newest first. A record in `acked` is left out (what the human did isn't news to them) unless it changed again after the commit that acted; then its whole change since the last look is listed. In `GET /api/projects`, each readable project also has `revision: {number, changes, seen_at}`: the tip's number, how many records changed since the last look, and when that was.

Writes (`POST`) must be JSON (`Content-Type: application/json`, else 415 `not_allowed`) and must not come from another site (an `Origin` other than the server's own, or a `Sec-Fetch-Site` other than `same-origin` or `none`: 403 `not_allowed`), so a web page elsewhere can't post a form to it. `curl` with the JSON header is fine. A write's status follows its code: 400 `invalid_argument` or `batch_invalid`, 403 `not_allowed`, 404 `not_found`, 409 `conflict`, 503 `busy`.

Its JSON API answers only requests addressed to `localhost` or `127.0.0.1` (others get 403 `not_allowed`); errors are the same `{code, error}` objects, and an unknown endpoint is 404 `not_found`.

| endpoint | returns |
|---|---|
| `GET /api/projects` | `{projects: [{id, name, path, summary?: Summary, error?: Error}]}`, sorted by `id`. `name` is the main worktree's directory name and `id` is the name made unique (a later repository with the same name gets `-2`, `-3`). A repository that can't be read (moved or deleted: `not_a_repo`; no mull branch: `not_initialized`; a newer format: `unsupported_format`) has `error` instead of `summary`. Summary: `{integration, integration_found, release, ahead_of_release, plans: [{slug, title, state}], needs_you: {reviews, decisions, questions, older_than_a_week, parked}, blocking, agents, untriaged, commit, last_change, format, migration_required?}`: `plans` are the active plans, then the shaping ones; `blocking` counts Needs you's first tier (below); `agents` counts active items; `commit` is the mull branch's tip and `last_change` its commit time |
| `GET /api/build` | `{build, version}`: `version` is mull's, and `build` (12 hex) names this server's build, a hash of the version and the UI it embeds. The page reloads itself when, after the event stream reconnects, the build differs from the one it loaded with |
| `GET /api/events` | A `text/event-stream` of live updates. It opens with a comment, then sends `event: project` with `data: {"id"}` for each project whose mull branch moved (a write from any of its worktrees, a sync, packed refs) or that was added to or removed from the registry (under its old ID and its new one, when IDs shift); the page refetches that project. Bursts are gathered for 150ms, and repeats of a project collapse into one event. An idle stream gets a `: keepalive` comment every 25s. The stream ends when `mull serve` stops, so reconnect (the browser's `EventSource` does) and refetch everything then, since events sent while away are not replayed |
| `GET /api/projects/{id}` | The project overview: `{summary: Summary, counts: Counts, needs_you: NeedsYou, plans: [PlanStatus], agents: [Agent]}`. `plans` are every plan not done, active first; `agents` is every active item, unplanned ones too, in work order (see Rank) |
| `GET /api/projects/{id}/tree` | The sheet index: `{plans: [{slug, title, state, blocking, needs_you, own: Attention, phases: [PhaseRow + {needs_you, items: [TreeItem]}], unfiled: [TreeItem]}], counts: Counts, blocking, needs_you, unplanned: Attention, own: Attention, archive: [{slug, title, closed}], archived_unplanned: [{id, title, state, finished}]}`. `archive` is the closed plans, most recently closed first; `archived_unplanned` is the unplanned work in the Archive (done or dropped, with no entry on it open or parked), most recently finished first (`closed` and `finished` are the record's last change). Plans not done, active first; phases in schedule order; `unfiled` is the plan's committed work with no phase; items are committed work (not captured), in rank order. `TreeItem`: `{id, title, state, ready?, needs_you?, blocking?}`; `Attention`: `{needs_you, blocking}`. `needs_you` counts open entries (parked ones aside), each once, on the row that shows it: on its item when the tree lists the item; else on its plan's `own` (about the plan, or on a captured item); on `unplanned` for unplanned work; else the project's `own`. A row's count includes everything below it, so the tree's `needs_you` is its plans', plus `unplanned`, plus `own`, and equals the overview's open count. `blocking` counts the first tier the same way |
| `GET /api/projects/{id}/plans/{slug}` | `{plan: Plan (with summary, doc and rules), status: PlanStatus, schedule: [{key, title, step, phases: [PhaseRow]}], needs_you: NeedsYou, next: [PhaseRow + {items: [ItemRow]}], unfiled: [ItemRow], doc_refs: [DocRef], backlog, untriaged}`. `schedule` is the done phases (`key` `"done"`, `step` -1), then a step per depth of needs not yet done: `"now"` (step 0, can start now), `"then"`, and `"go_live"` for the step holding the go-live phase. `next` is what's next: the phases that can start now, with their active, in-review and committed items in rank order. `unfiled` is committed work in the plan with no phase; `backlog` and `untriaged` count the captured items tagged to it |
| `GET /api/projects/{id}/phases/{phase}` | `{phase: Phase (with summary, doc and rules), plan_title, plan_state, status: PhaseRow, needed_by: [PhaseRef], items: [ItemRow], captured: [ItemRow], needs_you: NeedsYou, doc_refs: [DocRef]}`. `items` is its committed work in rank order; `captured` the backlog tagged to it; `needs_you` the open entries on its items |
| `GET /api/projects/{id}/items/{item}` | `{item: ItemView (with notes), plan_title?, phase_title?, phase_state?, git: {target, landed_commits, last_landed?, merged_at?, unmerged_commits, last_activity?, branches?, seen_in_git}, needs_you: NeedsYou, reports: [{n, passed, at}], archived?, review?: {entry?: Entry, report: Report, decisions: [{id, text, detail?, settled?: Settlement}]}}`. `review` is the review packet: the latest done report with what has become of each decision it carries (`settled` absent while open), and `entry` the review entry filed for that report (absent when the latest attempt failed its checks and wasn't filed). No report, no `review`. `archived` is `true` when the item is in the Archive: in a closed plan, or finished unplanned work |
| `GET /api/projects/{id}/unplanned` | `{items: [ItemRow], needs_you: NeedsYou, archived}`: committed items with no plan, in rank order, and the open entries on them. Finished unplanned work (done or dropped, with no entry on it open or parked) is archived and left out, and `archived` counts it; with `?done=all` it's listed too and `archived` is 0 |
| `GET /api/projects/{id}/backlog` | `{items: [ItemRow], bugs, ideas, untriaged, last_triage?, categories: [], plans: [{slug, title, state}]}`: captured items, untriaged first, then by category, then oldest first. `categories` and `plans` are what the items use, for filters. `?plan=<slug>` is one plan's backlog, `?plan=none` the items tagged to no plan |
| `GET /api/search?q=` | Find: `{query, status, project?, hits: [{project, type ("plan"|"phase"|"item"|"entry"), id, title, plan?, item?, kind?, state?, rank, archived?, finished?}], open, finished}`, records whose ID starts with `q` or whose title has every word of it, best first (`rank` 0 the ID, 1 starts the ID, 2 starts words of the title, 3 inside it; newest first among equals), at most 30. `id` is a plan's slug or the record's ID; `item` an entry's item; `state` an item's stage, a plan's state, a phase's (`done`, `now`, `waits`), an entry's settle action or `"open"`. `?status=` picks the records: `open` (the default: live work, meaning items not done or dropped, plans and phases not done, entries open or parked), `finished` (the rest: done and dropped items, done phases and plans, entries settled other than by parking, and everything archived), or `all`; another value is 400 `invalid_argument`. `archived` marks a hit in the Archive (a closed plan or a record in one, finished unplanned work or an entry on it), and `finished` one that `status=open` leaves out. `open` and `finished` count every match in each status whichever was asked for, the limit aside. `?project=<id>` searches that project only (an unknown one is 404 `not_found`); without it, every project, leaving out any that can't be read |
| `GET /api/projects/{id}/revision` | The revision schedule: `{id, tip: Revision, since: Revision, baseline?, changes: [Change], seen_at}`. `since` is the revision last looked at, marked seen at `seen_at`; `baseline` is `true` when this read made the tip the baseline (nothing is new yet). An unknown `id` is 404 `not_found`; a project that can't be read is 500 with its code (`not_a_repo`, `not_initialized`, `unsupported_format`) |
| `POST /api/projects/{id}/seen` | Marks a revision seen: body `{commit?}`, the mull branch commit the page showed (the tip if left out or the body is empty), which must be the tip or before it (else 400 `invalid_argument`). Changes after it stay new. Returns what `GET …/revision` returns from then on |
| `POST /api/projects/{id}/ops` | Writes mull data as the human: body `{commit, ops: [{op, ...fields}]}`, where `commit` is the mull branch commit the page showed and each op is a `mull batch` line (see Batch below). The ops run as one batch: all of them in one commit, or none. Only the ops the UI offers are taken (now `accept`, `decide`, `do`, `drop`, `park`, `bug`, `idea`, `set` (an item's `title` and `notes` only), `plan agree`, `plan close`, `plan doc`, `phase doc`, `plan rules`, `phase rules` and `rank`, which moves an item up or down by sending its group's whole new order; others are 403 `not_allowed`), and `on-request` is refused (400 `invalid_argument`): the write is the human's own, settled by their name (`mull name`, or else the repository's git `user.name`; neither set, 403 `not_allowed`) with `human: true`, and steps only the human takes need no `on-request`. If a record the ops name or write changed after `commit`, nothing is written: 409 `conflict`, and the page should redraw. Failed ops are 400 `batch_invalid` with `lines` numbered from 1 in `ops` order. Returns `{results: [{line, op, ref?, id?, result}], commit, acknowledged: [{type, id}], ack_error?}`: each op's result as `mull batch` prints it, the commit written (`""` if nothing changed), and the records that commit changed, now in the last look's `acked` so they aren't clouded. `ack_error` (an Error) says why they couldn't be recorded: the write still stands, and they stay clouded |

An unknown project, plan, phase or item is 404 `not_found`; a registered project that can't be read is 500 with its code (`not_a_repo`, `not_initialized`, `unsupported_format`). The shapes:

- **Needs you** is ranked by what an answer unblocks. `NeedsYou`: `{open, parked, tiers: [{key, rank, title, hint, count, entries?: [NeedRow], groups?: [DecisionGroup]}]}`, only the tiers that have entries, in rank order; `rank` stays fixed (1 to 6) when others are empty. The tiers: `blocking` (a question on active work, or on committed work that's ready), `review` (waiting for review), `now` (a question on an item in a phase whose needs are done), `later` (a question on an item in a phase still waiting), `plan` (a question about a whole plan or the project, or on an item in no phase that isn't ready), `decide` (decisions to approve). Entries are oldest first; parked entries are only counted. `NeedRow`: `{id, kind, text, detail?, plan?, plan_title?, phase?, phase_title?, item?, item_title?, item_state?, item_ready?, source?, created, decisions?}`; `phase` is the item's. The decide tier has `groups` instead of `entries`: `DecisionGroup` `{key, title, item?, plan?, review?, entries: [NeedRow]}`, grouped by source: the item they were made in (`key` `item:<id>`), else their source text (`source:<text>`), else their plan (`plan:<slug>`), else `project`. `review` is the open review carrying them: accepting it ratifies them.
- **Now and next** come from phases and needs only. `PhaseRow`: `{id, title, summary?, state ("done"|"now"|"waits"), go_live?, progress: Progress, questions, blocking, needs: [], waits_for: [PhaseRef]}`. A phase is `done` once complete, `now` when every phase it needs is done (and its plan isn't shaping), and `waits` otherwise; `waits_for` are its needs not done. `PhaseRef`: `{id, title, state}`. **Go live** is the phase a plan ends with: the one phase deepest in its needs, when that is more than one step deep; a plan whose phases need nothing, or that ends in several phases at once, has none.
- `PlanStatus`: `{slug, title, state, summary?, target: Target, progress: Progress, phases, built, questions, blocking, now: [PhaseRow], next: [PhaseRow], go_live: PhaseRow|null, agents: [Agent]}`. `built` counts done phases; `now` the phases that can start now; `next` the waiting phases one step away (a phase they need can start now), go live excepted; `questions` and `blocking` count the plan's open questions; `agents` are its active items in work order (by phase schedule, then rank).
- `Agent`: `{id, title, plan?, phase?, phase_title?, claim?: Claim, commits, branches?, last_activity?}`: an active item, with its linked commits (landed and not).
- `ItemRow`: `{id, title, state, ready?, kind?, category?, source?, plan?, phase?, phase_title?, claim?, untriaged?, triaged?, questions, position?, created, updated}`: an item without its notes; `questions` counts its open questions. Lists are in rank order (see Rank): open work first by rank, then done and dropped, unranked last, oldest first; the backlog has its own order. `position` is an open ranked item's place in its group's order, from 1, one more than the open items ranked before it (so equal ranks share one); finished and unranked items have none.
- `Counts`: `{unplanned, backlog, untriaged, last_triage?}`: the project's tab counts.
- `DocRef`: `{id, type ("phase"|"item"), title, state, progress?, questions}`: a phase or item the doc (or the rules) names by ID, in order of first mention, with its status (a phase's `done`/`now`/`waits`, an item's stage): docs hold intent, and status shows beside them.

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
| `plan new` | `slug`, `title`, `summary`, `on-request` (required: only the human creates a plan) |
| `plan agree` | `slug`, `on-request` (required: only the human agrees a plan) |
| `plan close` | `slug` |
| `plan set` | `slug`, `field`, `value` |
| `plan doc` | `slug`, `doc` (required: the whole new doc; `""` removes it) |
| `plan rules` | `slug`, `rules` (required: the whole new rules; `""` removes them) |
| `phase add` | `title`, `plan`, `summary`, `needs` |
| `phase doc` | `id`, `doc` (required: the whole new doc; `""` removes it) |
| `phase rules` | `id`, `rules` (required: the whole new rules; `""` removes them) |
| `phase delete` | `id`, `on-request` |
| `add` | `title`, `plan` (optional with `phase`; `none` for unplanned work), `phase`, `follow-up` (a done item, else `invalid_state`: the new item takes its plan and phase, which `plan` and `phase` must agree with, and `from` is set to it), `notes`, `doc`, `before` or `after` (an open item in its group to rank it next to; default last) |
| `rank` | `group` (a phase ID, plan slug or `none`), `ids` (every open item in it, first to last) |
| `set` | `id`, `field`, `value` |
| `need`, `unneed` | `id` (a phase), `needs` (phases) |
| `decided` | `text`, `item`, `plan`, `source`, `detail`, `human` (`true`: the human's own decision, settled at once), `on-request` (required with `human`, and only with it) |
| `ask` | `text`, `item`, `plan`, `source`, `detail` |
| `bug`, `idea` | `title` (as for `add`), `plan`, `phase` (both optional), `category`, `source`, `detail` (kept as notes) |
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

`init`, `sync`, `check` and `done` are not batch ops: they reach outside the mull branch. Nor is `migrate`, which reaches across every record.

## Migrating to Model v2

Data from before Model v2 is **format 1** (`config` shows `format: 1`). mull reads it correctly, but refuses every write with `migration_required` until it is migrated. `mull migrate --dry-run` (add `--text` to show the human) prints exactly what would change and writes nothing. The real run is the human's call: `mull migrate --on-request`, which fails with `not_allowed` without the flag. It writes one commit, and run again it changes nothing. It:

- **rewrites the format:** phases move from `epics/` to `phases/` (their docs from `docs/epics/` to `docs/phases/`, keeping their `--log` history), items store their stage and phase under Model v2 names, a done plan is stored as `done`, and config records `format: 2`;
- **captures loose ends and inbox ideas:** each open or parked one becomes a captured item of kind `idea` (title, `detail` as notes, `source`, its plan, `from` the entry), and the entry is settled `do`, pointing at it. Re-kind the bugs among them with `mull set <id> kind bug`. Settled ones stay as they are;
- **converts needs:** a phase's need on an item becomes a need on that item's phase, or is dropped when the item is done or dropped, has no phase, is in the same phase, or the need would make a cycle. Items' needs are dropped, and an unfinished item keeps them in its notes;
- **on the first migration only:** retires the standing `general` plan (it becomes `done`, its items unplanned and its entries project-wide; a `general` plan with phases stays as it is, with a warning), and gives each plan and phase with a `summary` and no doc a doc stub holding the summary (the summary stays).

Records in format 1 that reach migrated data later, merged in by `sync` from a machine still on an older mull, block writes again with `migration_required`; `mull migrate` converts them.
