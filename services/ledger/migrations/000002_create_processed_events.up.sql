CREATE TABLE processed_events (
    event_id VARCHAR(26) UNIQUE NOT NULL PRIMARY KEY,
    merchant_id VARCHAR(26) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);