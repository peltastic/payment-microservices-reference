-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "public";

-- CreateEnum
DO $$
BEGIN
    CREATE TYPE "DeliveryStatus" AS ENUM ('pending', 'delivered', 'failed', 'dead');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- CreateTable
CREATE TABLE IF NOT EXISTS "webhook_endpoints" (
    "id" TEXT NOT NULL,
    "merchant_id" TEXT NOT NULL,
    "url" TEXT NOT NULL,
    "secret" TEXT NOT NULL,
    "events" TEXT[],
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "description" TEXT,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "webhook_endpoints_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE IF NOT EXISTS "webhook_deliveries" (
    "id" TEXT NOT NULL,
    "endpoint_id" TEXT NOT NULL,
    "merchant_id" TEXT NOT NULL,
    "event_id" TEXT NOT NULL,
    "event_type" TEXT NOT NULL,
    "payload" JSONB NOT NULL,
    "status" "DeliveryStatus" NOT NULL DEFAULT 'pending',
    "attempt_count" INTEGER NOT NULL DEFAULT 0,
    "next_attempt_at" TIMESTAMP(3),
    "last_status_code" INTEGER,
    "last_response" TEXT,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "webhook_deliveries_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE IF NOT EXISTS "dead_letter_events" (
    "id" TEXT NOT NULL,
    "delivery_id" TEXT NOT NULL,
    "endpoint_id" TEXT NOT NULL,
    "merchant_id" TEXT NOT NULL,
    "event_type" TEXT NOT NULL,
    "payload" JSONB NOT NULL,
    "failure_log" JSONB NOT NULL DEFAULT '[]',
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "dead_letter_events_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX IF NOT EXISTS "webhook_endpoints_merchant_id_idx" ON "webhook_endpoints"("merchant_id");

-- CreateIndex
CREATE INDEX IF NOT EXISTS "webhook_deliveries_endpoint_id_idx" ON "webhook_deliveries"("endpoint_id");

-- CreateIndex
CREATE INDEX IF NOT EXISTS "webhook_deliveries_status_idx" ON "webhook_deliveries"("status");

-- CreateIndex
CREATE INDEX IF NOT EXISTS "webhook_deliveries_next_attempt_at_idx" ON "webhook_deliveries"("next_attempt_at");

-- CreateIndex
CREATE UNIQUE INDEX IF NOT EXISTS "dead_letter_events_delivery_id_key" ON "dead_letter_events"("delivery_id");

-- CreateIndex
CREATE INDEX IF NOT EXISTS "dead_letter_events_merchant_id_idx" ON "dead_letter_events"("merchant_id");

-- AddForeignKey
DO $$
BEGIN
    ALTER TABLE "webhook_deliveries" ADD CONSTRAINT "webhook_deliveries_endpoint_id_fkey" FOREIGN KEY ("endpoint_id") REFERENCES "webhook_endpoints"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- AddForeignKey
DO $$
BEGIN
    ALTER TABLE "dead_letter_events" ADD CONSTRAINT "dead_letter_events_delivery_id_fkey" FOREIGN KEY ("delivery_id") REFERENCES "webhook_deliveries"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
