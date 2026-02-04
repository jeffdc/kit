# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make all           # fmt, vet, test, build (default)
make build         # Build binary (outputs ./watchmen)
make install       # Install to GOBIN
make test          # Run all tests (go test -v ./...)
make test-coverage # Generate coverage report
make fmt           # Format code (gofmt -s -w)
make lint          # Run golangci-lint or fall back to go vet
```

Run a single test: `go test -v ./internal/storage -run TestName`

## Architecture

Watchmen is a time tracking and invoice generation CLI built with Cobra. Data is stored in JSON at `~/.watchmen/data.json`.

### Package Structure

- `cmd/` - CLI commands using Cobra framework
- `internal/model/` - Data structures (Project, Entry, Invoice, Settings, ContactInfo)
- `internal/storage/` - JSON persistence layer with auto-save and migrations
- `internal/invoice/` - Invoice/report generation (text, markdown, PDF)

### Key Patterns

**Global Store**: Root command initializes a global `store *storage.Store` in `PersistentPreRunE`. All subcommands access this for data operations.

**Entry Segments**: Time entries use segments to support pause/resume. An entry can have multiple TimeSegments, each with start/end times.

**One Active Entry**: The system enforces only one running or paused entry at a time.

**Auto-Save**: Every storage operation immediately persists to the JSON file.

**Data Versioning**: Storage includes migration support (v1→v2 converted flat entries to segment-based).

### Data Model

```
Data
├── Version (int)
├── Projects[] - ID, Name, HourlyRate, BillingContact, PurchaseOrder
├── Entries[] - ProjectID, Note, Segments[], Completed
├── Invoices[] - ProjectID, Hours, Rate, Amount, Status (Pending/Paid)
└── Settings - UserContact (for invoice "From" section)
```

### Invoice Generation

The `invoice` command generates invoices in text, markdown, or PDF format. Condensed mode (default) requires `--desc` flag. One-shot mode (`--one-shot`) auto-calculates date range from last invoice and generates PDF + markdown + stakeholder report.

## Testing

**CRITICAL: NEVER use the production data file (`~/.watchmen/data.json`) for testing or development.**

Always use test files when developing or testing features:

```bash
# Use --data flag to specify a test file
./watchmen --data /tmp/test_data.json list
./watchmen --data /tmp/test_data.json start myproject

# Or in Go tests, use the test helpers that create temporary files
# See internal/storage/storage_test.go for examples
```

The production data file contains real work history and should never be modified during development or testing.

## Wrap

When the user asks to "wrap" or "wrap up" work, follow these steps:

1. **Check outstanding changes**: Run `git status` to see all uncommitted changes
2. **Verify scope**: If changes include files not modified during this session, ask the user whether to include them or exclude them from the commit
3. **Commit**: Stage and commit the changes with an appropriate message
4. **Push**: Push to the remote repository
5. **Install**: Run `make install` to update the local watchmen binary
