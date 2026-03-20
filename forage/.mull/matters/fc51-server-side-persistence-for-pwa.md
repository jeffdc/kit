---
status: raw
tags: [forage, design, pwa]
created: 2026-03-19
updated: 2026-03-19
---

# Server-side persistence for PWA

## Design: Server-side persistence for PWA

### Goal

Replace the static file server on the Sprite with a Go API server so the PWA can persist changes server-side. The PWA becomes the primary interface (phone/laptop), with full offline support — writes go to IndexedDB immediately, sync to server when online.

### Server: Go API on the Sprite

A single Go binary (`forage-server`) that:
- Serves static PWA files from an on-disk directory
- Stores books in SQLite (`/home/sprite/forage.db`)
- REST API:
  - `GET /api/books` — full book list (PWA fetches on load/sync)
  - `POST /api/changes` — apply a batch of create/update/delete operations
  - `GET /api/version` — current data version (timestamp), so PWA knows if it's stale
- Auth: `Authorization: Bearer <shared-secret>` on all POST endpoints. Secret set via env var on Sprite.

### PWA: offline-first with background sync

- On load: try `GET /api/books` to get fresh data. If offline or failed, use IndexedDB (current behavior).
- On save: write to IndexedDB immediately (instant UI), then try `POST /api/changes`. If offline, changes queue in the `changes` IndexedDB store.
- On reconnect / visibility change: auto-flush queued changes to server.
- Replace "Download Changes" button with sync status indicator ("Synced" / "2 pending" / "Offline").

### Deploy flow

- `make deploy` cross-compiles Go server for linux/amd64, uploads binary + PWA assets to Sprite, restarts service.
- Embedded seed data in `index.html` goes away — PWA fetches from API instead.

### CLI integration

- CLI stays local (`~/.forage/forage.db`).
- `make sync FILE=...` still works for importing PWA change files locally.
- Future: `make push` to upload local DB state to server API.

### Auth

- Shared secret stored as env var on Sprite (`FORAGE_API_KEY`).
- PWA stores the key in localStorage after first entry (settings/setup flow).
- POST requests include `Authorization: Bearer <key>` header.

### What stays the same

- Local CLI workflow unchanged
- IndexedDB for offline PWA use
- Bookmarklet works as before (opens PWA, which syncs to server)

## Implementation Plan

**Goal:** Build a Go API server that replaces the static file server on the Sprite, enabling the PWA to persist book changes server-side with offline queuing.

**Architecture:** New `cmd/forage-server/` binary using net/http. Reuses `internal/storage` for SQLite and the existing `applyChanges` logic (moved from `cmd/import.go` to a shared package). PWA's `app.js` gains a sync layer that talks to `/api/*` endpoints when online, falls back to IndexedDB when offline.

**Tech Stack:** Go stdlib net/http (no framework), existing `internal/storage` + `modernc.org/sqlite`, cross-compiled for linux/amd64.

### Task 1: Extract change-application logic to shared package

**Files:**
- Create: `internal/changes/changes.go` — move `changelog`, `changeEntry`, `changeSummary`, and `applyChanges` from `cmd/import.go`
- Modify: `cmd/import.go` — import from `internal/changes` instead of local types
- Test: `internal/changes/changes_test.go` — move relevant tests from `cmd/import_changes_test.go`

**Behavior:**
The `applyChanges` function and its types are currently in `cmd/import.go` and only usable by the CLI. Move them to `internal/changes/` so both the CLI import command and the new server can reuse them. The function already accepts an interface — no signature changes needed.

**Testing:**
- Existing tests from `cmd/import_changes_test.go` move to the new package and still pass.
- `TestImportChanges_Create`, `TestImportChanges_Update`, `TestImportChanges_Delete`, `TestImportChanges_SkipMissing`, `TestImportChanges_MixedOps`, `TestImportChanges_JSONRoundtrip` all pass in new location.

### Task 2: Build the API server

**Files:**
- Create: `cmd/forage-server/main.go` — HTTP server entry point
- Create: `internal/api/api.go` — handler functions for the API routes
- Test: `internal/api/api_test.go`

**Behavior:**
The server binary reads config from env vars:
- `FORAGE_DIR` — path to data directory (default `/home/sprite/forage`)
- `FORAGE_API_KEY` — shared secret for POST auth
- `FORAGE_WWW` — path to static PWA files (default `./www`)
- `PORT` — listen port (default `8080`)

Routes:
- `GET /api/books` — returns JSON array of all non-dropped books (same shape as current `forage list` output). No auth required.
- `GET /api/version` — returns `{"version": "<RFC3339 timestamp>"}`. Timestamp updates whenever a write happens. No auth required.
- `POST /api/changes` — accepts the same changelog JSON format the PWA already produces (`{version, exported, changes: [...]}`). Requires `Authorization: Bearer <key>`. Returns `{"applied": N, "skipped": N, "errors": N}`. On success, bumps the version timestamp.
- `/*` — serves static files from `FORAGE_WWW` directory (PWA assets).

Auth middleware: check `Authorization: Bearer <key>` against `FORAGE_API_KEY` env var. Return 401 if missing/wrong. Only applies to POST routes.

**Testing:**
- `TestGetBooks` — creates a store with books, hits GET /api/books, verifies JSON response
- `TestGetVersion` — hits GET /api/version, verifies timestamp format
- `TestPostChanges_Auth` — POST without key returns 401, POST with wrong key returns 401, POST with correct key succeeds
- `TestPostChanges_Create` — POST a create change, verify book appears in subsequent GET /api/books
- `TestPostChanges_Update` — POST an update change, verify field changed
- `TestPostChanges_Delete` — POST a delete change, verify book gone
- `TestStaticFiles` — verify GET / serves index.html from www dir

**Notes:**
Depends on Task 1 for the shared `applyChanges` function.

### Task 3: Update PWA to sync with server API

**Files:**
- Modify: `internal/pwa/assets/app.js` — add sync layer
- Modify: `internal/pwa/assets/style.css` — sync status indicator styles
- Modify: `internal/pwa/assets/index.html` — replace download button with sync indicator, remove embedded book data

**Behavior:**

Sync layer in `app.js`:
1. On `DOMContentLoaded` after `db.init()`: call `GET /api/version`. If server version > stored version, fetch `GET /api/books` and reseed IndexedDB. If fetch fails (offline), silently use existing IndexedDB data.
2. After every local write (create/update/delete in IndexedDB): immediately attempt `POST /api/changes` with just that one change. If it succeeds, remove the change from the `changes` store and update stored version. If it fails (offline/error), leave the change queued.
3. On `visibilitychange` (tab becomes visible) and `online` event: flush all queued changes via `POST /api/changes`, then fetch fresh version.
4. Auth: read API key from `localStorage.getItem("forage_api_key")`. If not set, show a setup prompt (small modal or inline) asking for the key.

UI changes:
- Replace "Download Changes" button and sync bar with a status indicator: "Synced" (green dot), "N pending" (yellow dot + count), "Offline" (gray dot). Compact, non-intrusive.
- Remove `{{.Books}}` / `{{.Booksellers}}` / `{{.DataVersion}}` template vars from `index.html` — data comes from API now.
- `index.html` becomes a plain HTML file, no longer a Go template.

**Notes:**
The `POST /api/changes` payload is the same format the PWA already builds for the "Download Changes" feature — just sending it to the server instead of a file download. The `Authorization` header is the only new part.

Booksellers: still need to be served somehow. Simplest: add `GET /api/booksellers` endpoint, or embed them in the `/api/books` response as a separate field.

### Task 4: Update PWA export and deploy pipeline

**Files:**
- Modify: `internal/pwa/pwa.go` — `index.html` is no longer a template, just copy it. Remove `templateData` and template parsing. Export booksellers separately or remove from export.
- Modify: `internal/pwa/assets/index.html` — remove Go template syntax
- Modify: `Makefile` — new `deploy` target that cross-compiles server, uploads binary + PWA assets, restarts Sprite service
- Modify: `cmd/export.go` — PWA export no longer injects book data, just copies static files

**Behavior:**
`make deploy` flow:
1. `GOOS=linux GOARCH=amd64 go build -o /tmp/forage-server ./cmd/forage-server`
2. Upload binary to Sprite: `cat /tmp/forage-server | sprite exec -s forage bash -c "cat > /home/sprite/forage-server && chmod +x /home/sprite/forage-server"`
3. Export PWA static files to `/tmp/forage-pwa/` (no template rendering, just file copy)
4. Upload each PWA file to Sprite's `www/` directory (same as current)
5. Seed the server DB: `forage list | sprite exec -s forage bash -c "cat > /tmp/seed.json"` then hit the API, OR just copy the local DB file up on first deploy
6. Restart the Sprite service to pick up the new binary

First-time setup:
- Copy local `~/.forage/forage.db` to the Sprite as the initial server DB
- Update Sprite service from Python HTTP server to the Go binary
- Set `FORAGE_API_KEY` env var on the Sprite

**Testing:**
- `forage export` still produces a valid PWA directory (without embedded data)
- `make deploy` builds and deploys successfully

**Notes:**
Depends on Tasks 2 and 3. The first deploy is a one-time migration — subsequent deploys just update binary + assets.

### Task 5: Initial server setup and data migration

**Files:**
- No new code files — this is deployment/ops

**Behavior:**
One-time setup on the Sprite:
1. Copy local `~/.forage/forage.db` to Sprite
2. Set `FORAGE_API_KEY` env var (generate a random 32-char hex string)
3. Stop old Python service, start new Go binary service
4. Verify API works: `curl https://forage-4pbc.sprites.app/api/version`
5. Verify PWA loads and syncs

Store the API key locally for CLI use (e.g., in `~/.forage/api_key` or as an env var).

**Notes:**
Document the setup in the sprite-deploy memory file. This task should be done last, after all code is tested locally.

