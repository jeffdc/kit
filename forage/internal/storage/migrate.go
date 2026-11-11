package storage

import (
	"database/sql"
	"fmt"

	"forage/internal/model"
)

// migrations are applied in order; the database's PRAGMA user_version records
// how many have run. Append new migrations — never edit or reorder old ones.
var migrations = []func(*sql.Tx) error{
	migrateBaseline,
	migrateReadsAndRevisions,
	migrateOwned,
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if version > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than this binary supports (%d)", version, len(migrations))
	}

	for i := version; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := migrations[i](tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		// PRAGMA does not accept bound parameters.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: setting version: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d: commit: %w", i+1, err)
		}
	}
	return nil
}

// migrateBaseline brings any pre-versioned database (or an empty one) to the
// schema as of November 2026. Older databases may lack the later columns, and
// may carry an unused price_quotes column, which is left in place.
func migrateBaseline(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id              TEXT PRIMARY KEY,
			title           TEXT NOT NULL,
			author          TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'wishlist',
			tags            TEXT DEFAULT '',
			rating          INTEGER DEFAULT 0,
			date_added      TEXT NOT NULL,
			date_read       TEXT DEFAULT '',
			body            TEXT DEFAULT '',
			sort_author     TEXT DEFAULT '',
			page_count      INTEGER DEFAULT 0,
			first_published INTEGER DEFAULT 0,
			isbn            TEXT DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS booksellers (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url  TEXT NOT NULL
		);
	`); err != nil {
		return err
	}

	for _, col := range []struct{ name, def string }{
		{"sort_author", "TEXT DEFAULT ''"},
		{"page_count", "INTEGER DEFAULT 0"},
		{"first_published", "INTEGER DEFAULT 0"},
		{"isbn", "TEXT DEFAULT ''"},
	} {
		var n int
		if err := tx.QueryRow("SELECT COUNT(*) FROM pragma_table_info('books') WHERE name = ?", col.name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := tx.Exec("ALTER TABLE books ADD COLUMN " + col.name + " " + col.def); err != nil {
				return fmt.Errorf("adding %s: %w", col.name, err)
			}
		}
	}

	// Backfill author sort keys.
	rows, err := tx.Query("SELECT id, author FROM books WHERE sort_author = '' AND author != ''")
	if err != nil {
		return err
	}
	type entry struct{ id, author string }
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.author); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := tx.Exec("UPDATE books SET sort_author = ? WHERE id = ?", model.AuthorSortKey(e.author), e.id); err != nil {
			return err
		}
	}
	return nil
}

// migrateReadsAndRevisions adds:
//   - a reads table, seeded from books.date_read (books marked read without a
//     date get one undated read). books.date_read is unused afterwards.
//   - rev / updated_at / deleted on books and reads, for incremental sync
//   - meta (the global rev counter and the sync epoch)
//   - applied_ops, which makes client ops idempotent
func migrateReadsAndRevisions(tx *sql.Tx) error {
	now := nowStamp()
	stmts := []string{
		`ALTER TABLE books ADD COLUMN rev INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE books ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE books ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE reads (
			id            TEXT PRIMARY KEY,
			book_id       TEXT NOT NULL REFERENCES books(id),
			date_finished TEXT NOT NULL DEFAULT '',
			date_started  TEXT NOT NULL DEFAULT '',
			rev           INTEGER NOT NULL DEFAULT 0,
			updated_at    TEXT NOT NULL DEFAULT '',
			deleted       INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX reads_book_id ON reads(book_id)`,
		`CREATE INDEX reads_rev ON reads(rev)`,
		`CREATE INDEX books_rev ON books(rev)`,
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value INTEGER NOT NULL)`,
		`CREATE TABLE applied_ops (
			op_id      TEXT PRIMARY KEY,
			result     TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}

	rows, err := tx.Query(`SELECT id, date_read FROM books WHERE status = 'read' OR date_read != ''`)
	if err != nil {
		return err
	}
	type seed struct{ bookID, date string }
	var seeds []seed
	for rows.Next() {
		var s seed
		if err := rows.Scan(&s.bookID, &s.date); err != nil {
			rows.Close()
			return err
		}
		seeds = append(seeds, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, s := range seeds {
		id, err := NewID()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO reads (id, book_id, date_finished, rev, updated_at) VALUES (?, ?, ?, 1, ?)`,
			id, s.bookID, s.date, now,
		); err != nil {
			return err
		}
	}

	epoch, err := randomEpoch()
	if err != nil {
		return err
	}
	for _, s := range []string{
		`UPDATE books SET rev = 1, updated_at = '` + now + `'`,
		`INSERT INTO meta (key, value) VALUES ('rev', 1)`,
		`INSERT INTO meta (key, value) VALUES ('epoch', ` + fmt.Sprint(epoch) + `)`,
	} {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// migrateOwned makes ownership its own column, apart from status. The
// "owned" status meant "on the shelf, not started", so it becomes "to_read"
// with owned set. Those books get a new revision so clients pick them up.
func migrateOwned(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE books ADD COLUMN owned INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM books WHERE status = 'owned'`).Scan(&n); err != nil || n == 0 {
		return err
	}
	var rev int64
	if err := tx.QueryRow("UPDATE meta SET value = value + 1 WHERE key = 'rev' RETURNING value").Scan(&rev); err != nil {
		return fmt.Errorf("bumping rev: %w", err)
	}
	_, err := tx.Exec(`UPDATE books SET status = 'to_read', owned = 1, rev = ?, updated_at = ? WHERE status = 'owned'`, rev, nowStamp())
	return err
}
