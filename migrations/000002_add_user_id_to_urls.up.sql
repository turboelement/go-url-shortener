ALTER TABLE urls
ADD COLUMN user_id UUID;

CREATE INDEX IF NOT EXISTS idx_urls_user_id 
ON urls (user_id);
