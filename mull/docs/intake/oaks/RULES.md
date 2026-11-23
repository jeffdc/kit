# RULES.md (draft for oaks)

Jeff's rules for the oaks code. Agents propose changes and never make them on their own; a rule lands with its check in the same commit. A rule with no `check:` is advisory: a hope, not a guard. Checks run from the repository root inside the Nix shell. The hard checks (`make done-checks`) run first; rules marked "hard checks" below are enforced there and not repeated.

## How we work

## R1: Size the work before starting it

- scope: **

Small (a width, a wording, a component tweak, a few files): straight in the integration worktree, checked by `mix precommit`, `scripts/check-ui` and a look at 2 widths on 2 or 3 pages. Medium (a component rolled out across a few pages): one agent, own worktree, Impeccable's detector. Large (a new page, a redesign): the full Impeccable process with its finish review. At most 3 agents at once, on disjoint areas (mull's `config parallel`).

## R2: Nothing reaches main except Jeff's pull request

- scope: **
- check: grep -q 'oaks: runs scripts/githooks/pre-push' "$(git rev-parse --git-path hooks)/pre-push"

The pre-push hook blocks pushes to main or master and force pushes; this check fails if it isn't installed (`make hooks`). Never amend a commit. Jeff ships a milestone with one PR.

## R3: Dev servers run as background tasks and are never killed by hand

- scope: **

Stop your own with your harness's stop tool or `scripts/stop-dev-server <port>`. Never touch 4444 (Jeff's `make dev`) or 4460 (his preview).

## R4: One toolchain: the Nix flake, matched by the Dockerfile and CI

- scope: flake.nix, flake.lock, Dockerfile, .github/**, config/config.exs
- check: scripts/check-toolchain

A Homebrew Elixir formats differently from CI and production.

## R5: Code is formatted, compiles without warnings, and passes Credo

- scope: lib/**, test/**, config/**
- check: mix format --check-formatted && mix credo --strict

Tests and Dialyzer are in the hard checks (`mix precommit`, `mix dialyzer`).

## R6: Decide locally; ask only for a rule conflict or something irreversible

- scope: **

Replaces CLAUDE.md's "stop, present findings, wait for approval" for small and medium work. An ask names the decision, your default, and when the default stands.

## Architecture

## R7: Contexts own their tables; everything else calls their public functions

- scope: lib/oaks/**, lib/oaks_web/**

No module reads another context's tables or schemas directly (gallformers' pattern). Advisory until the Boundary item (first in Sources and facts core) adds Boundary; that change adds `- check: mix compile --warnings-as-errors` here, with today's crossings in Boundary's `dirty_xrefs`. The declared map is Boundary's own spec.

## R8: The web layer reaches data only through contexts

- scope: lib/oaks_web/**
- check: grep -rlE 'Oaks\.Repo|Ecto\.Query' lib/oaks_web
- ratchet: yes

Baseline 3: health_controller (pings the Repo; may stay), analytics_live and species_source_form_live (move their queries into Analytics and Sources).

## R9: Schema changes are migrations; data changes go through the app or the import endpoint, dry run first

- scope: priv/repo/migrations/**, lib/oaks/import.ex, lib/oaks/import/**, lib/oaks_web/controllers/private_api/**
- check: mix test test/oaks/import_test.exs test/oaks_web/controllers/private_api/import_controller_test.exs

No SQL over `fly ssh`. Production's database is the source of truth; local copies are for development. The server refuses an apply without the matching dry-run token; these tests keep it so.

## R10: SQLite only: no ilike, and every LIKE has an ESCAPE

- scope: lib/**
- check: ! grep -rnE '\bilike\(|fragment\(.*LIKE' lib | grep -v ESCAPE

SQLite has no default escape character, so `%` and `_` in a search would match anything.

## R11: Input never becomes an atom, and raw HTML is only sanitized markdown

- scope: lib/**
- check: { grep -rl 'String.to_atom(' lib | sed 's/^/to_atom /'; grep -rl 'raw(' lib/oaks_web | sed 's/^/raw /'; }
- ratchet: yes

Baseline: 7 files using `raw(`, each on `Oaks.Markdown` output or component markup. Atoms are never collected; `raw` bypasses escaping.

## R12: Editing is gated by @current_scope, and every writing handle_event re-checks it

- scope: lib/oaks_web/live/**, lib/oaks_web/controllers/**

Read pages are public; a LiveView event can be sent by anyone with the page open.

## R13: The public API is read-only; the private API needs a scoped agent token

- scope: lib/oaks_web/router.ex, lib/oaks_web/controllers/api/**, lib/oaks_web/controllers/private_api/**, lib/oaks_web/plugs/**

Jeff's materials never appear in the public API.

## R14: Dev-only routes exist only with dev_routes

- scope: lib/oaks_web/router.ex, config/**
- check: mix test test/oaks_web/controllers/dev_login_controller_test.exs test/oaks_web/live/dev_components_live_test.exs

The dev login logs in with no password; a production release must have none.

## R15: Never log PII or secrets

- scope: lib/**, config/**

Emails, passwords, tokens, names. Use structured Logger metadata.

## UI

## R16: Pages use only the component library

- scope: lib/oaks_web/**, assets/**
- check: scripts/check-ui

check-ui fails on page-width wrappers, inline styles, daisyUI cards and badges, hand-rolled scientific names, and app.css growing past its budget; its own allowlist (`scripts/check-ui.allow`) only shrinks. Five page agents each built their own widths, names and dialogs (decision 8f6b).

## R17: Page CSS never restyles a library component

- scope: assets/css/app.css
- check: grep -E '^[^{}/]*\.oc-[a-z0-9-]+[^{]*\{' assets/css/app.css | sed 's/ *{.*//' | sort -u
- ratchet: yes

Baseline 12 selectors (bug cab5). A component that almost fits is extended in the library.

## R18: A missing component is added to the library first, as its own change

- scope: lib/oaks_web/components/**, CODING_STANDARDS.md

Shown in `/dev/components`; the Components table in CODING_STANDARDS.md changes in the same commit.

## R19: The printed world: one ochre field, marks not colours, rules not boxes, square corners

- scope: lib/oaks_web/**, assets/**

Ochre is the only saturated fill (IUCN stamps excepted, decision 5604); agree, differs and unreviewed are drawn marks; no cards, shadows or tinted panels; no rounded corners. DESIGN.md has the detail.

## R20: Every taxon page uses one template; a section shows when its data exists

- scope: lib/oaks_web/live/**

A taxon Jeff hasn't covered gets the full page plus the not-covered notice, never a stub.

## R21: Jeff's words are verbatim, and only his

- scope: lib/**, priv/repo/**

Agents never write, paraphrase, summarize or merge his prose; they find and assemble it with sources. On the page his words are in curly quotes with a source mark; derived values are in the frame face, tagged. The import doesn't guard his text yet (decision dbe1); the Authoring editor phase adds the check (the import refuses entry text).

## R22: Copy is sentence case, with fixed patterns

- scope: lib/oaks_web/**

"Back to <place>" in lower case; "New <thing>" for create actions; a submit names the act ("Create source").

## Tests and assets

## R23: Tests build their own rows and never read the seeds

- scope: test/**

Fixtures insert through the context's create function, in `setup` or the test body; assert only on rows the test made.

## R24: No Process.sleep in tests

- scope: test/**
- check: ! grep -rn 'Process.sleep' test

Sleeps make tests slow and flaky; synchronize with monitors or messages.

## R25: Two asset bundles; no inline scripts, no @apply

- scope: assets/**, lib/oaks_web/**
- check: ! grep -rn '@apply' assets/css && ! { grep -rnE '<script[ >]' lib/oaks_web | grep -v 'src='; }

Hooks instead of inline scripts (the CSP forbids them); utilities in templates instead of @apply.

## Data

## R26: Every fact is tied to its source, and Jeff's lead

- scope: lib/oaks/**

An Oak Compendium fact leads its slot; other sources stand behind it; disagreeing values raise a conflict for Jeff, never a silent winner.

## R27: iNaturalist is the taxonomic backbone, with no regional assumptions

- scope: lib/oaks/**

Departures from iNat are Jeff's sourced decisions. Traits assume nothing eastern North American or red/white-oak specific.
