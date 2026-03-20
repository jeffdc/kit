# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Build Commands

```bash
make all           # fmt, vet, test, build (default)
make build         # Build binary (outputs ./forage)
make build-tui     # Build TUI binary (outputs ./forage-tui)
make install       # Install to GOBIN
make test          # Run all tests (go test -v ./...)
make fmt           # Format code (gofmt -s -w)
make vet           # Run go vet
make deploy        # Build server, export PWA, deploy to Sprite
make sync FILE=x   # Import PWA changes and redeploy
```

Run a single test: `go test -v ./internal/storage -run TestName`

## Architecture

Forage is a personal book management tool with a CLI, TUI, and PWA web interface. The PWA is the primary interface (phone/laptop, offline-capable). LLMs are also consumers via Claude Code skills. CLI output is JSON to stdout, errors as JSON to stderr.

Local data is stored in `~/.forage/` (global, not project-local). Override with `FORAGE_DIR` env var.

Server data is stored on the Sprite at `/home/sprite/forage/forage.db`.

### Package Structure

- `cmd/` - CLI commands using Cobra framework
- `cmd/forage-server/` - HTTP API server entry point (deployed to Sprite)
- `cmd/forage-tui/` - Bubble Tea TUI entry point
- `internal/api/` - HTTP API handlers (books, changes, booksellers, static files)
- `internal/changes/` - Changelog application logic (shared by CLI import and API server)
- `internal/model/` - Data structures (Book, Bookseller)
- `internal/openlibrary/` - Open Library API client (title, author, page count, ISBN, subjects)
- `internal/storage/` - SQLite persistence layer
- `internal/tui/` - Bubble Tea TUI (book list, detail)
- `internal/pwa/` - PWA static assets (embedded, copied on export)

### Key Patterns

**Global Store**: Root command initializes a global `store *storage.Store` in `PersistentPreRunE`, closed in `PersistentPostRunE`. All subcommands access this.

**SQLite Storage**: All data lives in SQLite via `modernc.org/sqlite` (pure Go, no CGO). Books table keyed by 4-char hex ID. Tags stored comma-separated. New columns are auto-migrated on startup.

**JSON Output**: All CLI commands output JSON to stdout. Errors go to stderr as `{"error": "message"}`.

**Auto-Init**: Running any command creates the database and tables automatically.

**API Server**: `forage-server` binary serves the PWA and provides REST API endpoints. Auth via `Authorization: Bearer <key>` on POST endpoints. PWA syncs to server when online, queues changes in IndexedDB when offline.

### Data Layout

```
~/.forage/
  forage.db    # SQLite database (books, booksellers tables)
```

### Book Fields

id, title, author, status, tags, rating, sort_author, date_added, date_read, body, page_count, first_published, isbn.

## Deployment

PWA and API server are hosted on a Fly.io Sprite at https://forage-4pbc.sprites.app.

`make deploy` cross-compiles `forage-server` for linux/amd64, uploads the binary and PWA assets to the Sprite. The Sprite runs the Go server on port 8080 as a persistent service.

API endpoints: `GET /api/books`, `GET /api/version`, `POST /api/changes` (bearer auth), `GET /api/booksellers`.

Sprite setup details are in the project memory at `sprite-deploy.md`.

## Testing

Always use `t.TempDir()` for test isolation. Never operate on real `~/.forage/` directories in tests. Call `t.Cleanup(func() { s.Close() })` to close the store after each test.

## Wrap

When wrapping up:
1. `git status` to check changes
2. Commit with appropriate message
3. Push to remote
4. `make install` to update local binary
