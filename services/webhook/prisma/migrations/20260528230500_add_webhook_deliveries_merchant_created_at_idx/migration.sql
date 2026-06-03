CREATE INDEX IF NOT EXISTS webhook_deliveries_merchant_id_created_at_idx
ON webhook_deliveries (merchant_id, created_at);