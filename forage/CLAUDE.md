# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## What Forage is

A personal book library: a Go server (`forage-server`) that serves a PWA and its API. The PWA is the only interface; it's used mostly from an iPhone and works offline. There is no CLI, TUI or Claude skill (removed November 2026; recreate from git history if ever needed).

The current rework plan lives in mull matter `e140` (`.mull/matters/e140-*.md`).

## Build Commands

```bash
make               # fmt, vet, test, build (default)
make build         # Build ./forage-server
make test          # Run all tests
make run           # Local server on :8091, API key "dev", PWA served from disk
make dev-data      # Copy the newest forage-backup-*.db into .dev/ for `make run`
make backup        # Download a verified copy of the production database
make deploy        # Test, back up, upload, swap, restart, verify
```

Run a single test: `go test -v ./internal/storage -run TestName`

For browser testing, `.claude/launch.json` defines `forage-local` (runs `make run`). In the browser, set `localStorage.forage_api_key = "dev"` once.

## Package Structure

- `cmd/forage-server/` - Entry point: config from env, HTTP server with timeouts and graceful shutdown
- `internal/api/` - HTTP handlers, auth, request logging, static file serving
- `internal/changes/` - Legacy: applies the original PWA's change log (`POST /api/changes`) onto the current schema
- `internal/model/` - Book, Read, Op and Bookseller types, statuses, author sort keys
- `internal/resolve/` - Turns a shared URL, ISBN, Bluesky post or text into candidate books (Open Library) and marks library matches
- `internal/storage/` - SQLite store, versioned migrations (`migrate.go`), op application (`ops.go`)
- `internal/pwa/` - Embedded PWA files (`assets/`), served by the server

## Key Patterns

**SQLite**: `modernc.org/sqlite` (pure Go, no CGO). One connection (`SetMaxOpenConns(1)`) with `busy_timeout`. WAL is deliberately not used because the Sprite's filesystem behavior with SQLite shared memory is unknown.

**Migrations**: `internal/storage/migrate.go` holds an ordered list. `PRAGMA user_version` records how many have run. Append new migrations; never edit or reorder old ones. The server refuses to start on a database newer than it knows.

**Auth**: every `/api` route except `/api/health` requires `Authorization: Bearer <FORAGE_API_KEY>`.

**PWA**: plain JS, no build step, no framework. IndexedDB holds books and a queue of pending changes. The service worker is network-first and bypasses the HTTP cache so deploys show up. Bump `CACHE_NAME` in `sw.js` when changing app files.

**Book IDs**: new IDs are 12 hex characters (`storage.NewID`, `genId` in the PWA). Older books keep 4-character IDs. Reads use the same ID format.

**Reads**: read history lives in the `reads` table (one row per reading, `date_finished` may be empty for unknown). A book's `date_read` in API responses is derived from its latest read. `books.date_read` is a legacy column, unused since migration 2.

**Revisions and sync**: every write runs in `Store.write`, which bumps a global revision (`meta.rev`) and stamps the changed rows. Deletes are soft (tombstones). `GET /api/sync?since=REV` returns changes after REV. `meta.epoch` identifies the database: after restoring a backup, change it (`UPDATE meta SET value = value + 1 WHERE key = 'epoch'`) so clients start over with a full sync.

**Ops**: clients send `POST /api/ops` batches; each op has a client `op_id`, and `applied_ops` makes replays harmless. A "rejected" result is permanent (bad input, unknown ID, ID taken): the client should drop the op and tell the user, not retry. A 5xx means retry the whole batch.

**Resolver**: deterministic, no LLM. It tries, in order, an ISBN in the URL, Open Library's own IDs, the page's JSON-LD/Open Graph data, and the URL slug. Bluesky posts are read through the public API and their link card and links are followed. Page fetches use a client that refuses internal addresses (checked on the dialed IP, including redirects). Loose searches are filtered for relevance.

## Server environment

- `FORAGE_API_KEY` (required)
- `FORAGE_DIR` (default `/home/sprite/forage`)
- `PORT` (default 8080)
- `FORAGE_TZ`: IANA zone used for "today" (default: system zone, UTC on the Sprite)
- `FORAGE_DEV_WWW`: development only; serve PWA files from disk instead of the embedded copy

## API

All routes except `/api/health` need `Authorization: Bearer <key>`.

- `GET /api/health`: build version (public)
- `GET /api/sync?since=REV`: books and reads changed after REV (or a full snapshot), plus `rev` and `epoch`
- `POST /api/ops`: `{"ops": [...]}` → `{"results": [...]}`, one ok/rejected result per op (see `model.Op`)
- `POST /api/books`: create one book with a server ID (for the iOS Shortcut)
- `POST /api/resolve`: `{"input": "<url, isbn or text>"}` → candidate books with library matches
- `GET /api/booksellers`: seller search URL templates
- Legacy, used by the current PWA until its rewrite: `GET /api/books` (non-dropped books), `GET /api/version`, `POST /api/changes`. Through `/api/changes`, marking a book read records a read dated today, and creating a book as read records an undated read.

## Deployment

Hosted on a Fly.io Sprite at https://forage-4pbc.sprites.app. The server runs as a Sprite service via `/home/sprite/serve.sh`, which holds the API key. The PWA is embedded in the binary, so a deploy uploads one file.

`make deploy` runs tests and then takes a verified backup. It uploads the new binary beside the old one, swaps it in, restarts the service, and waits for `/api/health` to report the new build. If any step fails, the old server keeps running. More Sprite details are in the project memory at `sprite-deploy.md`.

## Testing

Always use `t.TempDir()` for test isolation. Call `t.Cleanup(func() { s.Close() })` to close the store after each test.

## Wrap

When wrapping up:
1. `git status` to check changes
2. Commit with an appropriate message (stage files by name)
3. Push the branch to the remote (never main)
4. Deploy only when asked
