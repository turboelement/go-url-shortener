CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS urls (
    id           UUID PRIMARY KEY DEFAULT uuidv7(),
    short_id     VARCHAR(256) NOT NULL UNIQUE,
    original_url VARCHAR(4096) NOT NULL UNIQUE,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_original_url_unique 
ON urls (original_url);