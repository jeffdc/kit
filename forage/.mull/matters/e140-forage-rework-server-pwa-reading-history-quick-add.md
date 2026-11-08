---
status: planned
tags: [forage, design, pwa]
created: 2026-11-08
updated: 2026-11-08
relates: [fc51]
---

# Forage rework: server + PWA, reading history, quick add

# Forage rework plan

Agreed 2026-11-08 after a technical audit and a features/UI review.

## Goals

1. Keep a history of everything read, with the date finished and a rating. Re-reads count.
2. Search the whole library, fast, offline.
3. Add to the wishlist quickly, mostly from iPhone, mostly from a link (website or Bluesky post).
4. At a bookstore: browse the wishlist, check "have I read this?", add to the wishlist.

The app should be fast and task-oriented. No covers.

## Decisions

- **Scope:** server + PWA only. Drop the CLI, TUI, Claude skill, CSV importers, local `~/.forage` DB, and bookmarklet. They can be recreated if needed.
- **Source of truth:** the server DB on the Sprite.
- **Statuses:** keep all six (wishlist, owned, reading, paused, read, dropped). Dropped books stay visible and can be un-dropped.
- **Reads:** a book can have many reads, each with a date finished (day precision). The rating stays on the book. (Default choice; it can move to per-read if wanted.)
- **Link extraction:** deterministic only to start: structured data plus Open Library. No LLM.
- **Share-sheet adding:** via an iOS Shortcut.
- **Booksellers:** keep them all, used sellers first and Amazon last as a reference. Add BookFinder.
- **Dropped:**
  - price scraping (its numbers are wrong)
  - barcode scanning (it needs a network lookup to be useful, and typing a few letters into search is about as fast)
  - covers
  - tags in the UI (the data is kept for later)
  - the bookmarklet

## Known bugs this fixes

- New PWA books can vanish. IDs are 4 hex characters (about a 3% collision chance per add at 2,100 books), and the server silently skips collisions while the client clears its queue anyway.
- The client clears the whole change queue after any successful push, which can drop changes queued in the meantime.
- With a bad API key, local edits are hidden by the next reseed and show as "pending" forever.
- Editing an author doesn't update its sort key.
- There's no read-date field: all 14 books marked read since March 2026 have no date.
- Search only looks inside the selected status, so "have I read this?" fails from the Wishlist view.
- Dropped books can never be un-dropped.
- The production DB has no independent backup.

## Phase 0: Safety (small)

- [x] Take a `make backup` now, before any changes.
- [x] `make deploy` runs a backup first and refuses to deploy if the backup fails.

## Phase 1: Prune and server foundation

The existing API keeps working throughout this phase, because the old PWA on the phone still uses it.

- [x] Delete the CLI commands, root `main.go`, TUI, skill, importers, `internal/openlibrary`, `forage export`, and the `pwa/` export folder. Remove the Cobra and Bubble Tea dependencies.
- [x] Make the server the module's only binary (it stays at `cmd/forage-server`).
- [x] Serve the PWA from the embedded files, not from disk. Deploy becomes "upload one binary".
- [x] Add `make run`: a local server against a temporary or copied DB, for development and browser testing. This replaces the temporary `.claude/launch.json` set up during the review.
- [x] Hardening:
  - server timeouts, request logging, graceful shutdown
  - body-size limits
  - constant-time key check
  - drop CORS `*`
- [x] Require the API key on all `/api` reads. (The current PWA already sends it.)
- [x] SQLite: `busy_timeout`, a single connection, and numbered migrations keyed on `PRAGMA user_version`. WAL was skipped: how the Sprite's filesystem handles SQLite's shared-memory file is unknown, and one connection already prevents busy errors.
- [x] Remove price scraping (`/api/prices`, `pricing.go`), and drop `price_quotes` from API responses.
- [x] Deploy:
  - back up first
  - upload to a temporary path, then swap and restart
  - fail loudly
  - stamp the build version (served at `/api/health`)
- [x] Update CLAUDE.md and the sprite-deploy memory.
- [x] Added: interim PWA data-loss fixes ahead of the Phase 3 rewrite. New IDs are 12 hex characters; pushes run one at a time; only changes that were sent are removed from the queue; the price UI is removed.
- [x] Added: the service worker bypasses the browser's HTTP cache. The old server sent no cache headers, so browsers kept stale `app.js` and the "network-first" worker still got old files. After this deploy the phone needs one or two reopens to pick up the new version.
- [ ] Deploy to production, and check on the phone that pending changes flush and new adds get 12-character IDs.

## Phase 2: Data model and sync API

- [x] Add a `reads` table: id, book_id, date_finished, optional date_started.
  - Migrate the existing `date_read` values into it. Books marked read with no date get a read with an empty date, which History shows under "Undated".
  - A book is "read" when it has at least one read and isn't currently being read again.
- [x] Every row gets `rev`, `updated_at` and `deleted`:
  - `rev` comes from one global counter.
  - `deleted` makes deletes soft (tombstones).
- [x] Book updates change multiple fields in one transaction, and recompute `sort_author` whenever the author changes.
- [x] New IDs are 12 hex characters, generated on the client so offline adds work. A collision is rejected explicitly, never skipped silently. Existing 4-character IDs are kept.
- [x] `GET /api/sync?since=REV` returns changed books and reads (including tombstones) plus the new revision.
- [x] `POST /api/ops` takes ops that each carry a client op ID, and returns a result per op. The client deletes only the ops that succeeded.
- [x] `POST /api/books` is a simple create, for the Shortcut.
- [x] `POST /api/resolve` takes a URL, ISBN or text and returns book candidates, each marked if it's already in the library ("read 2019 ★★★★", "on wishlist").
  - **ISBN:** Open Library edition → work → title and authors.
  - **URL:** fetch the page (with an SSRF guard, size and time limits). Pull JSON-LD Book data, og tags, and ISBNs from known URL patterns (Amazon, Bookshop, Goodreads, AbeBooks, etc.). Then search Open Library.
  - **Bluesky post URL:** fetch the post through Bluesky's public API. If it has a link card, resolve that link; otherwise return the post text.
  - **Text:** Open Library search.
  - Library matching normalizes titles and authors (accents, punctuation, a leading "The", subtitles).
- [x] Tests for migrations, sync semantics, and the resolver extractors (using saved fixture pages).
- [x] Added: legacy `/api/changes` maps onto reads. Marking a book read records a read dated today; creating a book as read records an undated read. This fixes missing read dates even before the PWA rewrite.
- [x] Added: optional `FORAGE_TZ` so "today" is local time, not UTC.

Notes for Phase 3:
- 1,345 of 1,889 reads are undated (old imports), so History needs a good "Undated" section.
- The client should drop and surface "rejected" ops, retry the whole batch on a 5xx, and do a full resync when `epoch` changes.
- The Sprite's proxy already compresses responses (Brotli), so the full sync (~740 KB raw) is fine.
- The resolver handles Goodreads, Amazon, Bookshop, ThriftBooks, Wikipedia, Penguin Random House and Open Library links, as well as Bluesky posts with link cards. Posts without links return their text for manual entry.
- [ ] Deploy to production.

## Phase 3: PWA rewrite

Plain JS as native ES modules, with no build step and no framework.

- [ ] Data layer: IndexedDB plus an op queue with per-op acknowledgments and delta pulls. The sync state machine is written as pure functions, tested with `node:test` and no dependencies.
- [ ] Upgrade path: on first load, the new PWA pushes any changes still queued by the old version through the old `/api/changes`, and only then switches to the new storage format. Nothing queued on the phone is lost.
- [ ] Home: a search box that always searches the whole library, with results showing their status, read date and stars. Below it, tabs: **Wishlist · Reading · History**. Controls sit within thumb reach.
- [ ] Wishlist: compact rows sorted by author's last name, which matches bookstore shelving. Optional grouping by letter. Owned books flagged.
- [ ] Reading: reading and paused.
- [ ] History: grouped by year with counts, showing date finished and stars. Re-reads appear under each year they were read.
- [ ] Book detail sheet:
  - actions: Start reading / Start re-read, Finished (date defaults to today and can be changed; stars inline), Got it, Pause, Drop/Undo
  - a list of reads, each editable
  - seller links
  - Edit as a secondary action
- [ ] Add screen: one smart input (URL, ISBN, or title/author) → `/api/resolve` → pick a candidate → added to the wishlist, with a duplicate warning. Offline: manual entry, queued.
- [ ] Settings: API key, sync status and a manual sync, build version, a dropped-books view.
- [ ] Sync indicator: a small dot in the header.
- [ ] Service worker: precache the app shell, with the cache name stamped from the build version.
- [ ] Verify at phone size in the browser pane (speed on the full library, offline behavior). Then you check it on the real iPhone, including the update flow (tab close/reopen).

## Phase 4: Quick add from the iOS share sheet

Home-screen web apps on iOS keep their own storage, separate from Safari, so a Shortcut that opens a link would land in Safari, not the app. Instead, the Shortcut talks to the API directly:

- [ ] Shortcut flow: share a URL or text → POST `/api/resolve` → **Choose from List** ("Title — Author", plus "already read 2019" if it matches) → POST `/api/books` with status wishlist → notification saying it was added.
- [ ] If nothing resolves, save it to an **Inbox** (a small server table plus an app view) (stored on the server, shown in the app) to fix by hand later.
- [ ] Replace the bookmarklet page (`add.html`) with Shortcut setup instructions.

## Phase 5: Cleanup

- [ ] After the new PWA is on the phone and its old queue has flushed, remove `/api/books`, `/api/changes`, `/api/version` and the `changes` package.
- [ ] Fill in finish dates for the 14 books read since March, using the new UI.
- [ ] Close the obsolete mull matters (0aab sync to CLI, 7a39 StoryGraph import) and mark fc51 as superseded.

## Parallel: automated off-Sprite backups

- [ ] Implement `BACKUP_PLAN.md`: a daily GitHub Actions job that copies the DB to a bucket on another provider (R2, B2, or similar). This needs you to choose a provider and create the bucket and secrets.

## Not doing (for now)

Barcode scanning, covers, price lookup, tags in the UI, LLM link extraction, CLI/TUI, notes in the main UI (the field stays under Edit).
