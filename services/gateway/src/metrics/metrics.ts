import {
  makeCounterProvider,
  makeHistogramProvider,
  makeGaugeProvider,
} from '@willsoto/nestjs-prometheus';

// HTTP metrics
export const httpRequestsTotalProvider = makeCounterProvider({
  name: 'http_requests_total',
  help: 'Total HTTP requests',
  labelNames: ['method', 'path', 'status'],
});

export const httpRequestDurationProvider = makeHistogramProvider({
  name: 'http_request_duration_seconds',
  help: 'HTTP request duration in seconds',
  labelNames: ['method', 'path'],
  buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5],
});

// Gateway-specific metrics
export const authValidationsTotalProvider = makeCounterProvider({
  name: 'payment_reference_auth_validations_total',
  help: 'Total API key validation attempts',
  labelNames: ['result'], // valid, invalid, revoked
});

export const proxyRequestsTotalProvider = makeCounterProvider({
  name: 'payment_reference_proxy_requests_total',
  help: 'Total requests proxied to downstream services',
  labelNames: ['service', 'status'],
});
