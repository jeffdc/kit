# kit: proposed settings

- **Branches:** release `main`; integration `next` (unplanned work lands there); `forage-rework` gets its own target, `plan/forage-rework`. `item-order` and `mull-backlog-ui` are mull's own and can close or keep `next`.
- **Hard checks:** `make done-checks` at the root, as today.
- **Setup for a new worktree:** none needed (Go modules cache globally). Forage's PWA may want `npm ci` if it has a package.json: `test -f forage/internal/pwa/package.json && (cd forage/internal/pwa && npm ci) || true`.
- **Risk:** the defaults (RULES.md, ratchets, CI, Makefiles, go.mod files, migrations, auth) plus `mull/internal/store/**` and `mull/internal/mull/sync.go`, the code that writes and pushes the mull branch.
- **Map:** for mull, `go list -deps`-style edges between its internal packages: `cd mull && go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./... | awk '{for(i=2;i<=NF;i++) if ($i ~ /^mull\//) print $1" -> "$i}'`, recorded once as the baseline.
- **Hook:** kit's own pre-push hook already refuses agents' pushes to main, so `mull hooks` leaves it.
- **AGENTS.md:** already carries the new agent guide.

To apply (Jeff's call): commit `RULES.md` at the root with empty ratchet baselines (`mull rules ratchet R1 --grow --on-request`, then R8), then run the batch in `intake.jsonl`.
