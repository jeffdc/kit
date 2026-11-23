# Settings (draft for oaks)

Everything here except the map is in `intake.jsonl`. Run from `~/dev/oaks`.

## Branches

| | Today | Proposed |
|---|---|---|
| release | main | main (only Jeff's PR reaches it) |
| integration (unplanned work) | **main** | `next` |
| compendium | main | `plan/compendium`, preview `http://localhost:4460` |
| search (shaping) | main | `plan/search` |
| gallformers-parity | main | unset, so `next` (two small ports) |
| audit-2026-11 | main | closed |

mull makes each branch from main when its first item starts. Jeff points his 4460 preview at `plan/compendium` (`make dev BRANCH=plan/compendium PORT=4460`). Today's `integration: main` is the "roll out piecemeal on main" choice (decision 4468); it goes.

## Worktree setup

`mull start` runs these in each new worktree (`MULL_MAIN` is `~/dev/oaks`). Each is a no-op if the source is missing.

```
mull config setup --on-request \
  'test -d "$MULL_MAIN/deps" && { cp -Rc "$MULL_MAIN/deps" . 2>/dev/null || cp -R "$MULL_MAIN/deps" .; } || true' \
  'test -d "$MULL_MAIN/_build" && { cp -Rc "$MULL_MAIN/_build" . 2>/dev/null || cp -R "$MULL_MAIN/_build" .; } || true' \
  'mkdir -p priv/plts && cp "$MULL_MAIN"/priv/plts/*.plt priv/plts/ 2>/dev/null || true' \
  'test -f "$MULL_MAIN/priv/oaks_test.sqlite" && cp "$MULL_MAIN/priv/oaks_test.sqlite" priv/ || true' \
  'test -f "$MULL_MAIN/oaks.db" && sqlite3 "$MULL_MAIN/oaks.db" ".backup oaks.db" || true' \
  'direnv allow . 2>/dev/null || true'
```

deps and _build are copy-on-write clones on APFS, so compiling starts warm; the PLTs save Dialyzer's first run; the test database is the template `mix test` copies (the hard checks rebuild it anyway); the dev database is a safe copy, as `scripts/dev-branch` makes, for an agent's own server.

## Risk

Added to mull's defaults (which already cover RULES.md, `.mull/**`, `.github/**`, Makefile, mix.exs, mix.lock, package.json, `**/migrations/**`, `**/auth/**`):

- Auth: `lib/oaks/accounts.ex`, `lib/oaks/accounts/**`, `lib/oaks_web/user_auth.ex`, `lib/oaks_web/login_limiter.ex`, `lib/oaks_web/controllers/user_*`, `lib/oaks/release.ex`, `config/runtime.exs`
- Request pipeline and API: `lib/oaks_web/router.ex`, `lib/oaks_web/endpoint.ex`, `lib/oaks_web/plugs/**`
- Import endpoint: `lib/oaks/import.ex`, `lib/oaks/import/**`, `lib/oaks_web/controllers/private_api/**`, `lib/oaks_web/import_dry_run_token.ex`, `lib/oaks_web/body_digest.ex`
- Data and deploy: `priv/repo/**` (migrations and seeds), `Dockerfile`, `docker-entrypoint.sh`, `fly.toml`, `litestream.yml`, `rel/**`, `flake.nix`, `flake.lock`
- Checks: `scripts/check-ui`, `scripts/check-toolchain`, `scripts/githooks/**`. Not `scripts/check-ui.allow`: check-ui makes every improving UI change lower it, so gating it would gate UI work (mull can't yet tell a shrink from a grow).
- **Decision D3:** the component library, `lib/oaks_web/components/**` (AGENTS.md says Jeff reviews every new component). Leave it out and UI is never gated, as the redesign says.

Size and sample stay at the defaults: medium over 400 lines or 15 files; 20% of low-risk work sampled.

## Hard checks

Unchanged: `make done-checks` (test database, check-toolchain, check-ui, `mix precommit`, `mix dialyzer`), run once per change by `mull done`, never again on the fast-forward merge. RULES.md's checks run after it.

## Map (after the Boundary item lands)

Not a batch op. The Boundary item (Sources and facts core, first) adds Boundary to mix.exs as gallformers does, a `use Boundary` in each context with its `deps`, today's crossings in `dirty_xrefs`, and two scripts:

- `scripts/map-allowed`: the declared connections, from Boundary's own spec: `mix boundary.spec | awk '/^[A-Z]/{b=$1} /^  deps: /{sub(/^  deps: /,""); n=split($0,d,", "); for(i=1;i<=n;i++){sub(/ \(compile only\)/,"",d[i]); if(d[i]!="not checked") print b" -> "d[i]}}'`
- `scripts/map-edges`: the connections the code has, from `mix xref graph --format plain`, each file folded to its part (`lib/oaks_web/**` is OaksWeb; `lib/oaks/<x>.ex` and `lib/oaks/<x>/**` are `Oaks.<X>`), printed `from -> to`. Boundary's tasks only print declarations, so the real edges come from xref.

Then:

```
mull config map --edges scripts/map-edges --allowed scripts/map-allowed --on-request
mull map --record --on-request
```

Today's code (by grep), as the first declared map: OaksWeb uses every context; Import, Search and Stats use Species, Sources and Taxonomy; Taxonomy uses Species; Release uses Accounts; every context uses Repo and ChangesetHelpers. Two things to decide when declaring: Species and Sources call each other (a cycle), and three web files use Repo directly (R8).

## Hooks

Skip `mull hooks`: oaks's own pre-push guard (`make hooks`) already blocks main and force pushes, and mull leaves a hook it didn't write alone. R2 checks it's installed.
