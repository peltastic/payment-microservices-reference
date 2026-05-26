ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_idempotency_key_key;

CREATE UNIQUE INDEX IF NOT EXISTS payments_merchant_id_idempotency_key_key
ON payments (merchant_id, idempotency_key);
