---
status: raw
tags: [design, architecture]
created: 2026-02-28
updated: 2026-02-28
---

# Design and build forage

## What it is

A personal book management tool with three interfaces: a JSON CLI (for LLM use and scripting), a TUI (for interactive browsing), and a static HTML export (for phone use at bookstores). Follows mull's architecture patterns.

## Tech stack

- **Go** — same as mull, chosen for fast CLI startup, single binary, mature CLI/TUI ecosystem
- **Cobra** — CLI framework, all commands output JSON to stdout
- **Bubble Tea + Bubbles** — TUI
- **Single-file HTML** — exported static view with embedded JS for search/filter

## Data model

Each book is a file in `~/.forage/books/` (YAML frontmatter + markdown body for notes):

```
~/.forage/
  books/
    a3f1-the-left-hand-of-darkness.md
    b7c2-blood-meridian.md
  config.yml
```

```yaml
---
title: The Left Hand of Darkness
author: Ursula K. Le Guin
status: read
tags: [sci-fi, gender, classic]
rating: 5
date_added: 2026-02-28
date_read: 2026-01-15
---

Incredible exploration of gender and politics on a frozen planet.
```

**Statuses:** wishlist → reading → read, with dropped as terminal.

**File naming:** 4-char hex hash + slugified title (same pattern as mull).

## CLI commands (Phase 1)

| Command | Purpose |
|---------|---------|
| `forage add "Title" --author "Name"` | Add a book (defaults to wishlist) |
| `forage list [--status X] [--tag Y]` | List/filter books |
| `forage show <id>` | Full book details |
| `forage search <query>` | Full-text search |
| `forage set <id> key value` | Update fields |
| `forage read <id>` | Mark as read |
| `forage drop <id>` | Mark as dropped |
| `forage remove <id>` | Delete a book |
| `forage prime` | Token-efficient JSON snapshot for LLM context |
| `forage export` | Generate self-contained HTML file |

All commands output JSON. Errors as `{"error": "..."}` to stderr.

## TUI

Bubble Tea app with list view (filterable by status/tags, searchable), detail view, quick-add flow, and file watcher for live updates.

## Static HTML export

`forage export` generates a single .html file containing all non-dropped books as embedded JSON, with vanilla JS for search/filter/sort, mobile-friendly CSS, fully offline.

## LLM integration

- `forage prime` for token-efficient context injection
- Claude Code skill teaches the LLM the CLI commands
- LLM queries the library, adds recommendations to wishlist, etc.

## Phase 2 (future)

- Bookseller integration: `forage price <id>` to query online sellers
- Price tracking data stored alongside books
- Purchase links in the HTML export

## Out of scope (Phase 1)

- No LLM API calls from forage itself
- No bookseller queries
- No sync/cloud features
- No import from Goodreads or other services

## Implementation Plan

**Goal:** Build forage — a personal book library CLI + TUI + static HTML export, following mull's architecture patterns.

**Architecture:** Global data directory at `~/.forage/books/`, one markdown file per book with YAML frontmatter. CLI outputs JSON, TUI uses Bubble Tea, export generates a self-contained HTML file. Simpler than mull — no linking/relationships, no docket, no epics.

**Tech Stack:** Go, Cobra, Bubble Tea/Bubbles/Lipgloss, gopkg.in/yaml.v3, fsnotify.

### Task 1: Project scaffolding and data model

**Files:**
- Create: `go.mod` (module `forage`)
- Create: `main.go`
- Create: `internal/model/book.go`
- Create: `internal/model/book_test.go`
- Create: `Makefile`
- Create: `CLAUDE.md`

**Behavior:**
Initialize the Go module. `main.go` just calls `cmd.Execute()` (same as mull). The `Book` struct defines all fields:

```go
type Book struct {
    ID        string   `yaml:"-" json:"id"`
    Filename  string   `yaml:"-" json:"file"`
    Title     string   `yaml:"title" json:"title"`
    Author    string   `yaml:"author" json:"author"`
    Status    string   `yaml:"status" json:"status"`
    Tags      []string `yaml:"tags,omitempty" json:"tags,omitempty"`
    Rating    int      `yaml:"rating,omitempty" json:"rating,omitempty"`
    DateAdded string   `yaml:"date_added" json:"date_added"`
    DateRead  string   `yaml:"date_read,omitempty" json:"date_read,omitempty"`
    Body      string   `yaml:"-" json:"body,omitempty"`
}
```

Key difference from mull: `Title` and `Author` are stored IN the frontmatter (not extracted from a heading). The body is purely for notes/review. Valid statuses: wishlist, reading, read, dropped.

**Testing:**
- Valid status validation
- Status transition helpers (IsTerminal, etc.)

**Notes:**
Unlike mull which stores title as a `# Heading` in the body, forage stores title and author in frontmatter since they're structured data. The body is just freeform notes.

Makefile follows mull's pattern: `all`, `build`, `build-tui`, `install`, `test`, `fmt`, `vet`, `clean`.

### Task 2: Storage layer

**Files:**
- Create: `internal/storage/storage.go`
- Create: `internal/storage/storage_test.go`

**Behavior:**
File-based storage at `~/.forage/books/`. The `Store` struct holds the root path. `New()` creates the directory structure via `os.MkdirAll`. Unlike mull which uses `.` (project-local), forage defaults to `~/.forage/` but accepts an override path (for testing and flexibility).

Methods:
- `New(root string) (*Store, error)` — creates dirs, returns store
- `DefaultRoot() string` — returns `~/.forage` (respects `FORAGE_DIR` env var)
- `CreateBook(title, author string, meta map[string]string) (*model.Book, error)` — generates ID (SHA-256 of title+author+timestamp → 4-char hex), writes file
- `GetBook(id string) (*model.Book, error)` — find by ID prefix, parse file
- `ListBooks(filters map[string]string) ([]model.Book, error)` — read all, apply filters (status, tag, author)
- `UpdateBook(id, key, value string) (*model.Book, error)` — get, modify, write back
- `DeleteBook(id string) error` — remove file
- `SearchBooks(query string) ([]model.Book, error)` — case-insensitive substring match on title, author, body, tags

File format:
```yaml
---
title: The Left Hand of Darkness
author: Ursula K. Le Guin
status: wishlist
tags: [sci-fi]
date_added: 2026-02-28
---

Notes go here as markdown body.
```

ID generation: SHA-256(title + author + timestamp)[:2] → 4-char hex, collision retry.
File naming: `{id}-{slugified-title}.md`
Parsing: split on `---` delimiters, unmarshal YAML, body is everything after second `---`.

**Testing:**
- CreateBook generates valid ID and file
- GetBook round-trips correctly
- ListBooks with status/tag/author filters
- UpdateBook modifies fields and renames file if title changes
- DeleteBook removes the file
- SearchBooks matches across title, author, body
- ID collision retry works
- DefaultRoot respects FORAGE_DIR env var

**Notes:**
Tests should use `t.TempDir()` as the store root — never touch the real `~/.forage/`. Use the same YAML Node ordering technique as mull for clean, deterministic frontmatter output.

### Task 3: Root command and CLI skeleton

**Files:**
- Create: `cmd/root.go`
- Create: `cmd/output.go`

**Behavior:**
Package-level `var store *storage.Store`. `PersistentPreRunE` initializes it using `storage.DefaultRoot()`. Same error handling as mull — JSON errors to stderr, `SilenceUsage: true`.

`output.go` defines compact confirmation types:
```go
type bookConfirmation struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Status string `json:"status"`
}
```

And a `stripBodies(books)` helper that zeros out Body fields for list output.

**Testing:**
No direct tests needed — this is wiring. Tested transitively through command tests.

### Task 4: Core CRUD commands (add, show, list, search)

**Files:**
- Create: `cmd/add.go`
- Create: `cmd/show.go`
- Create: `cmd/list.go`
- Create: `cmd/search.go`

**Behavior:**

`forage add "Title" --author "Author"`:
- Positional arg: title (required)
- Flags: `--author` (required), `--status` (default: wishlist), `--tag` (repeatable), `--rating`, `--body`
- Returns: `bookConfirmation{ID, Title, Status}`

`forage show <id>`:
- Positional arg: book ID
- Returns: full Book JSON including body

`forage list`:
- No args
- Flags: `--status`, `--tag`, `--author`, `--all`
- Excludes terminal (dropped) by default unless `--all` or `--status dropped`
- Returns: `[]Book` with bodies stripped. Empty list returns `[]` not null.

`forage search <query>`:
- Positional arg: search query
- Returns: matching books with bodies stripped

**Testing:**
Test via the storage layer — command tests can be integration-style (create store, run command logic, check output). Key cases:
- add creates a book and returns confirmation
- show returns full book with body
- list filters by status/tag/author correctly
- list excludes dropped by default
- search finds matches in title, author, body, tags
- empty results return `[]`

### Task 5: Mutation commands (set, read, drop, remove)

**Files:**
- Create: `cmd/set.go`
- Create: `cmd/read.go`
- Create: `cmd/drop.go`
- Create: `cmd/remove.go`

**Behavior:**

`forage set <id> <key> <value>`:
- Sets a field on a book. Valid keys: title, author, status, rating, tags, date_read.
- For tags: value is comma-separated, replaces existing tags.
- Returns: `bookConfirmation`

`forage read <id>`:
- Shortcut: sets status to "read" and date_read to today.
- Returns: `bookConfirmation`

`forage drop <id>`:
- Sets status to "dropped".
- Returns: `bookConfirmation`

`forage remove <id>`:
- Deletes the book file entirely.
- Returns: `{"removed": "<id>"}`

**Testing:**
- set updates the correct field
- set rejects invalid keys
- set renames file when title changes
- read sets status and date_read
- drop sets status to dropped
- remove deletes the file
- operations on nonexistent IDs return errors

### Task 6: Prime command (LLM context)

**Files:**
- Create: `cmd/prime.go`

**Behavior:**

`forage prime`:
- Outputs a compact JSON snapshot of the library for LLM consumption
- Excludes dropped books
- Excludes body/notes (just title, author, status, tags, rating)
- Includes summary counts by status

```go
type primeBook struct {
    ID     string   `json:"id"`
    Title  string   `json:"title"`
    Author string   `json:"author"`
    Status string   `json:"status"`
    Tags   []string `json:"tags,omitempty"`
    Rating int      `json:"rating,omitempty"`
}
type primeOutput struct {
    Books  []primeBook    `json:"books"`
    Counts map[string]int `json:"counts"`
}
```

**Testing:**
- Excludes dropped books
- Excludes bodies
- Counts are accurate
- Empty library returns empty books array and zero counts

### Task 7: Export command (static HTML)

**Files:**
- Create: `cmd/export.go`
- Create: `internal/export/html.go`
- Create: `internal/export/html_test.go`

**Behavior:**

`forage export`:
- Flag: `--output` / `-o` (default: `forage-library.html`)
- Generates a single self-contained HTML file
- Embeds all non-dropped books as a JSON blob in a `<script>` tag
- Includes inline CSS (mobile-responsive) and vanilla JS for:
  - Text search (filters title, author, tags, notes)
  - Filter by status (wishlist / reading / read / all)
  - Sort by title, author, rating, date added
- No external dependencies — works fully offline
- Returns: `{"exported": "<path>", "books": <count>}`

`internal/export/html.go`:
- `Generate(books []model.Book, w io.Writer) error`
- Uses Go `html/template` to render the HTML with embedded data
- Template is a string constant in the Go source (no external template files)

**Testing:**
- Generated HTML is valid (contains expected elements)
- Book data is correctly embedded as JSON
- Dropped books are excluded
- Empty library produces valid HTML with "no books" message

**Notes:**
Keep the HTML/CSS/JS minimal but functional. Mobile-first CSS. The JS search should be instant (filtering a client-side array). No frameworks.

### Task 8: TUI

**Files:**
- Create: `cmd/forage-tui/main.go`
- Create: `internal/tui/app.go`
- Create: `internal/tui/list.go`
- Create: `internal/tui/detail.go`
- Create: `internal/tui/keys.go`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/watcher.go`

**Behavior:**
Standalone binary (`forage-tui`). Same architecture as mull-tui:
- `main.go`: creates store, launches `tea.NewProgram(tui.NewApp(store), tea.WithAltScreen())`
- `app.go`: App model with views (list, detail), filter state, cursor, search input
- `list.go`: renders book list, filterable by status and searchable
- `detail.go`: renders full book details including notes
- `keys.go`: keybindings (j/k navigate, enter for detail, / for search, q to quit, tab to cycle status filter)
- `styles.go`: lipgloss styles
- `watcher.go`: fsnotify watches `~/.forage/books/` for changes, sends reload msg

Views:
- **List view**: shows books in a table/list format with title, author, status, rating. Filterable by status, searchable.
- **Detail view**: full book info including notes body. Scrollable viewport.

Inline mutations (like mull's TUI):
- `S` to cycle status
- `D` to mark as read (done reading)
- `X` to drop

**Testing:**
TUI is tested manually. The `watcher.go` can have a basic test for file event detection.

**Notes:**
Depends on tasks 1-3 being complete. Follow mull-tui patterns closely — same Elm architecture, same watcher pattern.

### Task 9: Claude Code skill

**Files:**
- Create: `skills/SKILL.md`

**Behavior:**
A Claude Code skill that teaches the LLM how to use forage CLI commands. Covers:
- How to query the library (`forage prime`, `forage list`, `forage search`, `forage show`)
- How to add books (`forage add`)
- How to update books (`forage set`, `forage read`, `forage drop`)
- Workflow: when a user asks for book recommendations, load the library with `forage prime`, understand their reading history and preferences, make recommendations, and offer to add them to the wishlist

**Notes:**
This is just a markdown file — no code to test. Model it on mull's skill structure.

### Task ordering

Tasks 1 → 2 → 3 must be sequential (each depends on the previous). After task 3, tasks 4-7 can be done in any order (all use the same storage/CLI foundation). Task 8 (TUI) depends on tasks 1-3. Task 9 (skill) can be done anytime.

Recommended order: 1, 2, 3, 4, 5, 6, 7, 8, 9.

