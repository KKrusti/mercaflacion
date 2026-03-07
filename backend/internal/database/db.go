// Package database manages the SQLite connection and schema migrations.
package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at path, applies PRAGMAs, and
// runs all schema migrations. Pass ":memory:" for an in-memory database.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Single connection prevents WAL write contention.
	db.SetMaxOpenConns(1)

	if err := applyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	// Restrict on-disk DB to owner-only access (0600).
	// Skipped for ":memory:" and on filesystems that do not support chmod.
	if path != ":memory:" {
		if err := os.Chmod(path, 0600); err != nil {
			fmt.Printf("database: could not set permissions on %q: %v\n", path, err)
		}
	}

	return db, nil
}

func applyPragmas(db *sql.DB) error {
	pragmas := `
		PRAGMA foreign_keys  = ON;
		PRAGMA journal_mode  = WAL;
		PRAGMA synchronous   = NORMAL;
		PRAGMA temp_store    = MEMORY;
		PRAGMA cache_size    = -16000;
	`
	if _, err := db.Exec(pragmas); err != nil {
		return fmt.Errorf("apply pragmas: %w", err)
	}
	return nil
}

// migrate creates all required tables. Add new migrations as numbered steps;
// never alter or remove existing ones.
func migrate(db *sql.DB) error {
	m1 := `
		CREATE TABLE IF NOT EXISTS products (
			id       TEXT PRIMARY KEY,
			name     TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS price_records (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			product_id TEXT    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
			date       TEXT    NOT NULL,  -- ISO-8601: YYYY-MM-DD
			price      REAL    NOT NULL,
			store      TEXT    NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_price_records_product_id
			ON price_records(product_id);
	`
	if _, err := db.Exec(m1); err != nil {
		return fmt.Errorf("migrate m1: %w", err)
	}

	// SQLite does not support IF NOT EXISTS for ADD COLUMN, so check first.
	var colCount int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('products') WHERE name='image_url'`,
	).Scan(&colCount)
	if err != nil {
		return fmt.Errorf("migrate m2 check: %w", err)
	}
	if colCount == 0 {
		if _, err := db.Exec(`ALTER TABLE products ADD COLUMN image_url TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate m2 add column: %w", err)
		}
	}

	m3 := `
		CREATE TABLE IF NOT EXISTS processed_files (
			filename    TEXT PRIMARY KEY,
			imported_at TEXT NOT NULL   -- ISO-8601 timestamp
		);
	`
	if _, err := db.Exec(m3); err != nil {
		return fmt.Errorf("migrate m3: %w", err)
	}

	// m4: user accounts for multi-tenant support.
	m4 := `
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL,
			created_at    TEXT    NOT NULL  -- ISO-8601 timestamp
		);
	`
	if _, err := db.Exec(m4); err != nil {
		return fmt.Errorf("migrate m4: %w", err)
	}

	// m5: associate price_records and processed_files with a user.
	// NULL means the record belongs to no specific user (legacy seed data).
	if err := addColumnIfMissing(db, "price_records", "user_id",
		`ALTER TABLE price_records ADD COLUMN user_id INTEGER REFERENCES users(id)`); err != nil {
		return fmt.Errorf("migrate m5 price_records.user_id: %w", err)
	}
	if err := addColumnIfMissing(db, "processed_files", "user_id",
		`ALTER TABLE processed_files ADD COLUMN user_id INTEGER REFERENCES users(id)`); err != nil {
		return fmt.Errorf("migrate m5 processed_files.user_id: %w", err)
	}

	// m6: allow manual image URL pinning so the enricher won't overwrite it.
	if err := addColumnIfMissing(db, "products", "image_url_locked",
		`ALTER TABLE products ADD COLUMN image_url_locked INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("migrate m6 products.image_url_locked: %w", err)
	}

	// m7: optional email address per user account.
	if err := addColumnIfMissing(db, "users", "email",
		`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("migrate m7 users.email: %w", err)
	}

	// m8: household sharing — groups of users who share ticket imports.
	m8 := `
		CREATE TABLE IF NOT EXISTS households (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT    NOT NULL
		);

		CREATE TABLE IF NOT EXISTS household_invitations (
			token        TEXT    PRIMARY KEY,
			household_id INTEGER NOT NULL REFERENCES households(id) ON DELETE CASCADE,
			inviter_id   INTEGER NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
			expires_at   TEXT    NOT NULL
		);
	`
	if _, err := db.Exec(m8); err != nil {
		return fmt.Errorf("migrate m8: %w", err)
	}
	if err := addColumnIfMissing(db, "users", "household_id",
		`ALTER TABLE users ADD COLUMN household_id INTEGER REFERENCES households(id)`); err != nil {
		return fmt.Errorf("migrate m8 users.household_id: %w", err)
	}

	return nil
}

// addColumnIfMissing adds a column to a table only when it does not exist yet.
// SQLite does not support IF NOT EXISTS on ALTER TABLE ADD COLUMN.
func addColumnIfMissing(db *sql.DB, table, column, alterSQL string) error {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, column,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = db.Exec(alterSQL)
	}
	return err
}
