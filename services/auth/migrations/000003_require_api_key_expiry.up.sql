UPDATE api_keys
SET expires_at = NOW() + INTERVAL '365 days'
WHERE expires_at IS NULL;

ALTER TABLE api_keys
ALTER COLUMN expires_at SET NOT NULL;
