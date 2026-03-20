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

