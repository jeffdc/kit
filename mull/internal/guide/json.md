# mull output reference

Every command prints JSON on stdout. An error prints `{"code", "error"}` on stderr and exits 1; a failed batch adds `lines: [{line, op, code, error}]`.

## Error codes

| code | means |
|---|---|
| `usage` | unknown command or flag, wrong arguments |
| `invalid_argument` | a value is malformed or missing |
| `not_a_repo`, `not_initialized` | not in a git repository; no mull branch (`mull init`, or `mull sync` on a second machine) |
| `not_found`, `already_exists` | no such record; a plan slug already used |
| `invalid_state` | the record can't do that now (starting done work, landing on the release branch) |
| `claimed` | another worktree claimed the item |
| `already_settled` | the inbox entry is settled |
| `not_allowed` | the human's step taken without `--on-request`, or a page op mull serve doesn't offer |
| `dependency_cycle` | the phase needs would make a cycle |
| `rules_required` | `done` without `--rules` |
| `at_capacity` | as many items are active as `parallel` allows |
| `setup_failed` | a setup command failed in the new worktree, which was removed |
| `behind_target` | the branch lacks its target's tip: `git merge <target>`, then finish again |
| `no_checks`, `dirty_tree`, `checks_failed` | no hard checks; uncommitted changes; the checks (or a rule's) failed |
| `sync_conflict`, `push_refused`, `remote_error` | sync: the same records changed on both sides; non-mull paths; the remote failed |
| `batch_invalid` | a batch line failed; nothing was written |
| `busy`, `git_error`, `unsupported_format`, `port_in_use`, `launchd_error`, `internal` | as they say; `unsupported_format` is data from before format 2 (read it with `mull1`) |

## Records

- **Plan** `{slug, title, state (shaping|active|done), target?, preview?, created, updated, summary?, doc?}`. `target` is where its work lands (`plan/<slug>` by default); none means the integration branch.
- **Phase** `{id, plan, title, needs?, created, updated, summary?, doc?}`. It's complete when its committed work is done or dropped, with at least one done.
- **Item** `{id, plan?, phase?, title, state (captured|committed|active|in-review|done|dropped), kind? (bug|idea), area?, source?, covered_by?, from?, claim?: {worktree, branch, at, base?}, created, updated, notes?}`. Captured items are the queue.
- **Entry** `{id, kind (question|review|decision), text, plan, item?, by, created, settled?: {action, note?, item?, by, human?, on_request?, at}, landed?, default?, due?, detail?}`. A question has the agent's `default` and when it stands (`due`). A review with `landed` (`medium` or `sampled`) is a look at work already landed. A decision is an agent's, noted (`action: note`), or the human's (`decide`).
- **Report** `{item, n, passed, commit, base?, at, by, summary, rules, slowed?, look?, design?, risk, risk_why?, sampled?, checks: {tree, commit, at, passed, results: [{command, exit, duration_ms, tail?}]}, rule_checks?: [{id, passed, count?, new?, fixed?, tail?}], files?: [{status, path, added, removed}], decisions?, covered?, nearby?, new_edges?, text}`. `text` is the report as the human reads it.
- **Config** `{format, integration, release, remote, checks, risk: {high, max_lines, max_files, sample}, worktrees?, setup?, parallel?, map: {edges?, allowed?}}`.

## Commands

| command | prints |
|---|---|
| `context [--item <id>]` | `{protocol, rules: [Rule], item?: Item + {target, preview?, plan_doc?, phase_doc?, queue?, questions?}, ready, active}` |
| `start <id> [--here]` | `{item, warnings, worktree, branch, target, pruned?}` |
| `done <id>` | `{report, item, review_entry?, landed, merged, merge_error?, pruned?, reminder}` |
| `check` | the check run, plus `cached` |
| `report <id> [--n N] [--text]` | Report |
| `land <id>` | `{item, target, commit}` |
| `worktrees` | `{removed: [paths]}` |
| `hooks` | `{path, installed, note?}` |
| `add`, `bug`, `idea`, `set`, `commit`, `release` | Item; `drop` prints the items dropped |
| `list`, `ready` | `{items}` |
| `show <ref>` | `{type: plan, plan: Plan + {phases: [PhaseHill], items}}`, `{type: phase, phase, items}`, `{type: item, item, reports}` or `{type: entry, entry}` |
| `plan new\|agree\|close\|set`, `phase add\|set` | Plan or Phase |
| `ask`, `decided`, `decide` | Entry; `dismiss` prints the entries dropped |
| `accept <ids>` | `{entries, warnings?}` |
| `send-back <id>` | the reopened item, or a landed item's follow-up |
| `inbox` | `{entries}` (open, oldest first) |
| `decisions` | `{decisions: [{id, item?, plan?, asked?, decision, why?, by, human?, at}]}`, newest first |
| `status [--since] [--text]` | Surface: `{since, generated, calls: [{kind (review\|question\|look), entry, item?, plan?, text, why?, default?, due?, lapsed?, created}], rules: [{id, title, then, now, failing?, trend}], advisory, clusters: [{area, count, items}], problems: [{item, title, text}], new_edges, plans: [{slug, title, state, target, preview?, to_ship, phases: [PhaseHill]}], flow: {active, parallel, queue, landed, slowed, rework, median_minutes?}, audit?}` |
| `rules` | `{path, rules: [{id, title, scope?, check?, ratchet?, advisory?, why?}], checked, advisory}`; `rules check` `{passed, results}`; `rules ratchet <id>` `{id, path, before, after}` |
| `map [--record]` | `{configured, edges, allowed?, undeclared, unused, new, gone, recorded_at?}` |
| `audit` | `{text, at?}`; `audit record -` records one |
| `config` and its subcommands | Config |
| `batch` | `[{line, op, id?, result}]` |
| `sync` | `{remote, action, local, remote_before?, pushed_commits, dry_run?}` |

PhaseHill is `{id, title, hill (uphill|downhill|waiting|done), progress: {total, queued, committed, active, in_review, done, dropped}, blocked_by?}`.

## Batch

`mull batch` reads JSON Lines, one op per line, and runs them in one commit, all or nothing; `--dry-run` writes nothing. Field names are the flag names. A line's `ref` names what it creates, and `"@ref"` in a later line's ID field means that record. Ops: `plan new` (slug, title, summary, target, on-request), `plan agree`, `plan close` (slug, on-request), `plan set` (slug, field, value), `phase add` (plan, title, summary, needs), `phase set` (id, field, value), `add`, `bug`, `idea` (title, plan, phase, area, notes or detail, from, source), `set` (id, field, value), `commit` (id, plan, phase), `drop` (ids, reason), `ask` (text, item, plan, default, by, detail), `decided` (text, item, plan, detail, human, on-request), `decide` (id, decision, on-request), `accept` (ids, on-request), `send back` (id, reason, on-request), `drop entry` (ids, reason, on-request), `config set` (key, value, on-request), `config checks`, `config setup` (commands, on-request), `config risk` (add, remove, max-lines, max-files, sample, on-request), `config map` (edges, allowed, on-request).
