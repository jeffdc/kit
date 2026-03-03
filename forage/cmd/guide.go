package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func guideText() string {
	return `# Forage — Personal Book Library

## Quick Start

  forage add "Dune" --author "Frank Herbert"           # add a book (status: wishlist)
  forage add "Neuromancer" --author "William Gibson" --tag sci-fi --tag classic
  forage list                                           # list all non-dropped books
  forage show a3f2                                      # full details by 4-char hex ID
  forage set a3f2 status reading                        # update a field
  forage read a3f2                                      # shortcut: mark as read + set date
  forage set a3f2 rating 4                              # rate 1-5
  forage search "neuromancer"                           # search title, author, tags, notes
  forage drop a3f2                                      # mark as dropped (hidden by default)
  forage remove a3f2                                    # permanent delete

## Statuses

  wishlist   — want to read (default on add)
  reading    — currently reading
  paused     — on hold
  read       — finished
  dropped    — abandoned (terminal: hidden from list, prime, export)

Statuses are set via "forage set <id> status <value>" or shortcuts:
  forage read <id>    — sets status to "read" and date_read to today
  forage drop <id>    — sets status to "dropped"

## Fields

  Field       Type       Settable via "set"   Notes
  ─────       ────       ──────────────────   ─────
  id          string     no                   4-char hex, auto-generated
  title       string     yes                  required on add
  author      string     yes                  required on add (--author flag)
  status      string     yes                  one of: wishlist, reading, paused, read, dropped
  tags        []string   yes                  comma-separated in "set", --tag flag on "add"
  rating      int        yes                  1-5 (0 = unrated, omitted from output)
  date_added  string     no                   YYYY-MM-DD, auto-set on add
  date_read   string     yes                  YYYY-MM-DD, auto-set by "forage read"
  body        string     no (use add --body)  free-form notes

## Commands

### forage add <title> --author <author> [--status S] [--tag T]... [--rating N] [--body TEXT]
  Add a book. Status defaults to "wishlist".
  Output: {"id": "a3f2", "title": "...", "status": "wishlist"}

### forage list [--status S] [--tag T] [--author A] [--all]
  List books (excludes dropped unless --all). Body field is stripped.
  Output: [{"id": "...", "title": "...", "author": "...", "status": "...", ...}, ...]

### forage show <id>
  Full details of one book, including body.
  Output: {"id": "...", "title": "...", ..., "body": "..."}

### forage search <query>
  Search title, author, tags, and notes. Body stripped from results.
  Output: same shape as "list"

### forage set <id> <key> <value>
  Update a field. Valid keys: title, author, status, rating, tags, date_read.
  For tags, use comma-separated values: forage set a3f2 tags "sci-fi,classic"
  Output: {"id": "a3f2", "title": "...", "status": "..."}

### forage read <id>
  Mark a book as read. Sets status to "read" and date_read to today.
  Output: {"id": "a3f2", "title": "...", "status": "read"}

### forage drop <id>
  Mark a book as dropped (terminal status).
  Output: {"id": "a3f2", "title": "...", "status": "dropped"}

### forage remove <id>
  Permanently delete a book. Cannot be undone.
  Output: {"removed": "a3f2"}

### forage prime
  Token-efficient snapshot for LLM context. Excludes dropped books from
  the list but includes them in counts.
  Output: {"books": [{...minimal...}], "counts": {"reading": 1, "wishlist": 5, ...}}

### forage export [-o path] [--pwa]
  Generate a self-contained HTML file (or PWA directory) of your library.
  Output: {"exported": "forage-library.html", "books": 17}

### forage import <file.csv> [file2.csv ...] [--dry-run]
  Import from Goodreads or StoryGraph CSV exports. Auto-detects format.
  Multiple files are merged with deduplication.
  Output: {"imported": 42, "skipped_existing": 3, "skipped_duplicate": 1}

### forage import <file.json> --changes
  Apply a PWA-exported changelog (create/update/delete operations).
  Output: {"applied": 10, "skipped": 1, "errors": 0}

## Workflows

  Import:    forage import goodreads.csv storygraph.csv
  Browse:    forage list --tag sci-fi | jq '.[].title'
  Curate:    forage set a3f2 tags "sci-fi,classic"
  Track:     forage set a3f2 status reading → forage read a3f2
  Rate:      forage set a3f2 rating 5
  Export:    forage export -o my-books.html

## LLM Usage

  forage prime       — load library context (compact, token-efficient)
  forage guide       — load this reference (commands, fields, statuses)

All commands output JSON to stdout. Errors go to stderr as {"error": "..."}.
IDs are 4-character lowercase hex strings (e.g. "a3f2", "00ff").
`
}

var guideCmd = &cobra.Command{
	Use:   "guide",
	Short: "Print a structured reference for humans and LLMs",
	Long: `Print a comprehensive reference covering all commands, fields, statuses,
workflows, and output shapes. Useful for learning forage or feeding context
to an LLM agent.`,
	Args: cobra.NoArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil // no database needed
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(guideText())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(guideCmd)
}
