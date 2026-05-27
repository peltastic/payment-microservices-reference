-- Local/shared database bootstrap for the Go services.
-- The webhook service applies its Prisma migration from its own entrypoint.

CREATE TABLE IF NOT EXISTS merchants (
  id VARCHAR(26) PRIMARY KEY,
  email VARCHAR(255) UNIQUE NOT NULL,
  name VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
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

CREATE TABLE IF NOT EXISTS revoked_keys (
  key_hash VARCHAR(64) PRIMARY KEY,
  revoked_at TIMESTAMPTZ DEFAULT NOW(),
  reason VARCHAR(100)
);

ALTER TABLE api_keys
ALTER COLUMN scope TYPE VARCHAR(255);

CREATE TABLE IF NOT EXISTS idem_keys (
  key VARCHAR(255) PRIMARY KEY,
  merchant_id VARCHAR(26) NOT NULL REFERENCES merchants(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  response_body JSONB NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS payments (
  id VARCHAR(26) PRIMARY KEY,
  merchant_id VARCHAR(26) NOT NULL,
  amount BIGINT NOT NULL,
  currency VARCHAR(3) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  idempotency_key VARCHAR(255) NOT NULL,
  customer_email VARCHAR(255) NOT NULL,
  customer_name VARCHAR(255) NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  bank_reference VARCHAR(100),
  failed_reason VARCHAR(255),
  event_published BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT payments_status_check CHECK (
    status IN ('pending', 'processing', 'completed', 'cancelled', 'failed')
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS payments_merchant_id_idempotency_key_key
ON payments (merchant_id, idempotency_key);

CREATE TABLE IF NOT EXISTS merchant_balances (
  merchant_id VARCHAR(26) PRIMARY KEY,
  available BIGINT NOT NULL DEFAULT 0,
  pending BIGINT NOT NULL DEFAULT 0,
  currency VARCHAR(3) NOT NULL,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS processed_events (
  event_id VARCHAR(26) UNIQUE NOT NULL PRIMARY KEY,
  merchant_id VARCHAR(26) NOT NULL,
  event_type VARCHAR(50) NOT NULL,
  processed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS journals (
  id VARCHAR(26) PRIMARY KEY,
  merchant_id VARCHAR(26) NOT NULL,
  payment_id VARCHAR(26) NOT NULL,
  entry_type VARCHAR(20) NOT NULL,
  amount BIGINT NOT NULL,
  currency VARCHAR(3) NOT NULL,
  description TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
