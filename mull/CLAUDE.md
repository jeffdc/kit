# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Build Commands

```bash
make all           # fmt, vet, test, build (default)
make build         # Build binary (outputs ./mull)
make install       # Install to GOBIN
make test          # Run all tests (go test -v ./...)
make fmt           # Format code (gofmt -s -w)
make vet           # Run go vet
```

Run a single test: `go test -v ./internal/storage -run TestName`

## Architecture

Mull is a CLI tool for tracking ideas and features ("matters") for solo projects. LLMs are the primary consumer via Claude Code skills. All output is JSON to stdout, errors as JSON to stderr.

Data is stored in `.mull/` at the repo root, version controlled alongside the project.

### Package Structure

- `cmd/` - CLI commands using Cobra framework
- `internal/model/` - Data structures (Matter, Docket)
- `internal/storage/` - File-based persistence layer (.mull/ directory)

### Key Patterns

**Global Store**: Root command initializes a global `store *storage.Store` in `PersistentPreRunE`. All subcommands access this.

**Matter Files**: Each matter is a markdown file with YAML frontmatter stored at `.mull/matters/<4-char-hash>-<slug>.md`.

**JSON Output**: All commands output JSON to stdout. Errors go to stderr as `{"error": "message"}`.

**Auto-Init**: Running any command in a repo without `.mull/` creates the directory structure automatically.

**Bidirectional Links**: Relationship mutations update both sides atomically with rollback on failure.

### Data Layout

```
.mull/
  matters/
    ab3f-add-an-rss-feed.md
    c7d1-dark-mode.md
  docket.yml
  config.yml    # optional
```

## Testing

Always use `t.TempDir()` for test isolation. Never operate on real `.mull/` directories in tests.

## Wrap

When wrapping up:
1. `git status` to check changes
2. Commit with appropriate message
3. Push to remote
4. `make install` to update local binary
