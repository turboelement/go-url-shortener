DROP INDEX IF EXISTS idx_urls_is_deleted;
ALTER TABLE urls
DROP COLUMN is_deleted;
