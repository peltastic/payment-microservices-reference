CREATE TABLE merchant_balances (
    merchant_id VARCHAR(26) PRIMARY KEY,
    available BIGINT NOT NULL DEFAULT 0,
    pending BIGINT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);