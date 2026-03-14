// Package database manages the PostgreSQL connection and schema setup.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	once    sync.Once
	shared  *sql.DB
	openErr error
)

// Open returns the shared PostgreSQL connection pool, initialising it once per
// process lifetime. The DSN is read from DATABASE_URL (Vercel/Neon standard).
func Open() (*sql.DB, error) {
	once.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			openErr = fmt.Errorf("DATABASE_URL is not set")
			return
		}
		shared, openErr = OpenDSN(dsn)
	})
	return shared, openErr
}

// OpenDSN opens a fresh connection to the given DSN and applies the schema.
// Use this in tests or CLIs that need an independent connection.
func OpenDSN(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Serverless-friendly pool: Neon PgBouncer handles connection multiplexing.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := EnsureSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

// EnsureSchema creates all tables and indexes if they do not exist yet.
// Idempotent — safe to call on every cold start.
func EnsureSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}

// schema is the full PostgreSQL DDL for basket-cost.
// All statements are idempotent (IF NOT EXISTS / ON CONFLICT DO NOTHING).
const schema = `
CREATE TABLE IF NOT EXISTS households (
	id         BIGSERIAL   PRIMARY KEY,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
	id            BIGSERIAL   PRIMARY KEY,
	username      TEXT        NOT NULL UNIQUE,
	email         TEXT        NOT NULL DEFAULT '',
	password_hash TEXT        NOT NULL,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	household_id  BIGINT      REFERENCES households(id)
);

CREATE TABLE IF NOT EXISTS products (
	id               TEXT    PRIMARY KEY,
	name             TEXT    NOT NULL,
	category         TEXT    NOT NULL DEFAULT '',
	image_url        TEXT    NOT NULL DEFAULT '',
	image_url_locked BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS price_records (
	id         BIGSERIAL        PRIMARY KEY,
	product_id TEXT             NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	date       TEXT             NOT NULL,  -- YYYY-MM-DD
	price      DOUBLE PRECISION NOT NULL,
	store      TEXT             NOT NULL DEFAULT '',
	user_id    BIGINT           REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_price_records_product_id ON price_records(product_id);

CREATE TABLE IF NOT EXISTS processed_files (
	filename    TEXT        NOT NULL,
	imported_at TIMESTAMPTZ NOT NULL,
	user_id     BIGINT      REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_processed_files_dedup
	ON processed_files (filename, COALESCE(user_id, 0));

CREATE TABLE IF NOT EXISTS household_invitations (
	token        TEXT        PRIMARY KEY,
	household_id BIGINT      NOT NULL REFERENCES households(id) ON DELETE CASCADE,
	inviter_id   BIGINT      NOT NULL REFERENCES users(id)      ON DELETE CASCADE,
	expires_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS revoked_tokens (
	jti        TEXT        PRIMARY KEY,
	expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_revoked_tokens_expires_at ON revoked_tokens(expires_at);

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET is_admin = TRUE WHERE id = 227;

CREATE TABLE IF NOT EXISTS ipc_rates (
	year INTEGER          PRIMARY KEY,
	rate DOUBLE PRECISION NOT NULL
);

INSERT INTO ipc_rates (year, rate) VALUES
	(2015, -0.003),
	(2016, -0.002),
	(2017,  0.019),
	(2018,  0.017),
	(2019,  0.008),
	(2020, -0.003),
	(2021,  0.031),
	(2022,  0.084),
	(2023,  0.035),
	(2024,  0.028),
	(2025,  0.025)
ON CONFLICT (year) DO NOTHING;
`
