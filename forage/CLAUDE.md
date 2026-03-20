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
make deploy        # Export PWA and deploy to Sprite
```

Run a single test: `go test -v ./internal/storage -run TestName`

## Architecture

Forage is a personal book management CLI tool. LLMs are the primary consumer via Claude Code skills. All output is JSON to stdout, errors as JSON to stderr.

Data is stored in `~/.forage/` (global, not project-local). Override with `FORAGE_DIR` env var.

### Package Structure

- `cmd/` - CLI commands using Cobra framework
- `cmd/forage-tui/` - Bubble Tea TUI entry point
- `internal/model/` - Data structures (Book, Bookseller)
- `internal/storage/` - SQLite persistence layer (~/.forage/forage.db)
- `internal/tui/` - Bubble Tea TUI (book list, detail)
- `internal/pwa/` - PWA export (HTML, JS, service worker)

### Key Patterns

**Global Store**: Root command initializes a global `store *storage.Store` in `PersistentPreRunE`, closed in `PersistentPostRunE`. All subcommands access this.

**SQLite Storage**: All data lives in `~/.forage/forage.db` via `modernc.org/sqlite` (pure Go, no CGO). Books table keyed by 4-char hex ID. Tags stored comma-separated.

**JSON Output**: All commands output JSON to stdout. Errors go to stderr as `{"error": "message"}`.

**Auto-Init**: Running any command creates the database and tables automatically.

### Data Layout

```
~/.forage/
  forage.db    # SQLite database (books, booksellers tables)
```

## Deployment

PWA is hosted on a Fly.io Sprite at https://forage-4pbc.sprites.app. Config in `.sprite`.

`make deploy` exports the PWA and copies files to the sprite. The sprite runs a Python HTTP server on port 8080 as a persistent service (survives sleep/wake).

Sprite setup details are in the project memory at `sprite-deploy.md`.

## Testing

Always use `t.TempDir()` for test isolation. Never operate on real `~/.forage/` directories in tests. Call `t.Cleanup(func() { s.Close() })` to close the store after each test.

## Wrap

When wrapping up:
1. `git status` to check changes
2. Commit with appropriate message
3. Push to remote
4. `make install` to update local binary
