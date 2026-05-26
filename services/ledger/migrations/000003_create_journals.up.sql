CREATE TABLE journals (
    id VARCHAR(26) PRIMARY KEY,
    merchant_id VARCHAR(26) NOT NULL,
    payment_id VARCHAR(26) NOT NULL,
    entry_type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);