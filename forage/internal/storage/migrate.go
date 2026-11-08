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
