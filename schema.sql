-- basket-cost PostgreSQL schema
-- Run once against your Neon database to initialise the schema.
-- All statements are idempotent (IF NOT EXISTS / ON CONFLICT DO NOTHING).
-- The application also calls EnsureSchema() on every cold start, so this
-- file serves as a reference and for manual inspection / migration runs.

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

-- Per-user deduplication: same filename can't be uploaded twice by the same user.
-- COALESCE(user_id, 0) groups all anonymous/seed rows together.
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

-- IPC (Consumer Price Index) rates for Catalonia, 2015-2025.
-- Source: IDESCAT / INE. Rates are annual interannual average as decimal.
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
