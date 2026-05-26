DROP INDEX IF EXISTS payments_merchant_id_idempotency_key_key;

ALTER TABLE payments
ADD CONSTRAINT payments_idempotency_key_key UNIQUE (idempotency_key);
