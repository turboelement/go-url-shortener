DROP INDEX IF EXISTS idx_urls_user_id;
ALTER TABLE urls
DROP COLUMN user_id;
