import { makeCounterProvider, makeHistogramProvider, makeGaugeProvider } from '@willsoto/nestjs-prometheus'

// HTTP metrics
export const httpRequestsTotalProvider = makeCounterProvider({
  name: 'http_requests_total',
  help: 'Total HTTP requests',
  labelNames: ['method', 'path', 'status']
})

export const httpRequestDurationProvider = makeHistogramProvider({
  name: 'http_request_duration_seconds',
  help: 'HTTP request duration in seconds',
  labelNames: ['method', 'path'],
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5]
})
// Webhook-specific metrics
export const webhookDeliveriesProvider = makeCounterProvider({
  name: 'payment_reference_webhook_deliveries_total',
  help: 'Total webhook delivery attempts',
  labelNames: ['status']   // delivered, failed, dead
})

export const webhookDeadLetterProvider = makeGaugeProvider({
  name: 'payment_reference_webhook_dead_letter_total',
  help: 'Current number of events in the dead letter queue'
})

export const webhookRetryQueueProvider = makeGaugeProvider({
  name: 'payment_reference_webhook_retry_queue_total',
  help: 'Current number of webhook deliveries pending retry'
})