CREATE TABLE api_keys (
    id VARCHAR(26) PRIMARY KEY,
    merchant_id VARCHAR(26) NOT NULL REFERENCES merchants(id),
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    key_prefix VARCHAR(12) NOT NULL,
    scope VARCHAR(255) NOT NULL DEFAULT 'full',
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE revoked_keys (
    key_hash VARCHAR(64) PRIMARY KEY,
    revoked_at TIMESTAMPTZ DEFAULT NOW(),
    reason VARCHAR(100)
);
