ALTER TABLE urls
ADD COLUMN is_deleted BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_urls_is_deleted 
ON urls (is_deleted);
