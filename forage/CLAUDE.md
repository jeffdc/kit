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
```

Run a single test: `go test -v ./internal/storage -run TestName`

## Architecture

Forage is a personal book management CLI tool. LLMs are the primary consumer via Claude Code skills. All output is JSON to stdout, errors as JSON to stderr.

Data is stored in `~/.forage/` (global, not project-local). Override with `FORAGE_DIR` env var.

### Package Structure

- `cmd/` - CLI commands using Cobra framework
- `cmd/forage-tui/` - Bubble Tea TUI entry point
- `internal/model/` - Data structures (Book)
- `internal/storage/` - File-based persistence layer (~/.forage/ directory)
- `internal/tui/` - Bubble Tea TUI (book list, detail, file watcher)
- `internal/export/` - Static HTML export

### Key Patterns

**Global Store**: Root command initializes a global `store *storage.Store` in `PersistentPreRunE`. All subcommands access this.

**Book Files**: Each book is a markdown file with YAML frontmatter stored at `~/.forage/books/<4-char-hash>-<slug>.md`.

**JSON Output**: All commands output JSON to stdout. Errors go to stderr as `{"error": "message"}`.

**Auto-Init**: Running any command creates the directory structure automatically.

### Data Layout

```
~/.forage/
  books/
    a3f1-the-left-hand-of-darkness.md
    b7c2-blood-meridian.md
  config.yml    # optional
```

## Testing

Always use `t.TempDir()` for test isolation. Never operate on real `~/.forage/` directories in tests.

## Wrap

When wrapping up:
1. `git status` to check changes
2. Commit with appropriate message
3. Push to remote
4. `make install` to update local binary
