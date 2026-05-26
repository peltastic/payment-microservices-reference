CREATE TABLE idem_keys (
    key VARCHAR(255) PRIMARY KEY,
    merchant_id VARCHAR(26) NOT NULL REFERENCES merchants(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    response_body JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE payments (
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
    CONSTRAINT payments_status_check CHECK (status IN ('pending', 'processing', 'completed', 'cancelled', 'failed')),
    CONSTRAINT payments_merchant_id_idempotency_key_key UNIQUE (merchant_id, idempotency_key)
);
