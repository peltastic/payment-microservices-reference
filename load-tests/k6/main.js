import http from 'k6/http';
import exec from 'k6/execution';
import crypto from 'k6/crypto';
import { check, fail, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const TEST_TYPE = String(__ENV.K6_TEST_TYPE || 'smoke').toLowerCase();
const ENDPOINT_PACK = String(__ENV.K6_ENDPOINT_PACK || 'all').toLowerCase();
const BASE_URL = trimTrailingSlash(__ENV.K6_BASE_URL || 'http://localhost:3000');
const AUTH_BASE_URL = trimTrailingSlash(
  __ENV.K6_AUTH_BASE_URL || 'http://localhost:4001',
);
const INTERNAL_AUTH_SECRET =
  __ENV.K6_INTERNAL_AUTH_SECRET || __ENV.INTERNAL_AUTH_SECRET || '';
const RUN_ID = __ENV.K6_RUN_ID || `gateway-${TEST_TYPE}-${Date.now()}`;
const SCALE_MULTIPLIER = positiveNumber(__ENV.K6_SCALE_MULTIPLIER, 1);
const THINK_TIME_SECONDS = defaultThinkTimeSeconds();
const HTTP_TIMEOUT = __ENV.K6_HTTP_TIMEOUT || '30s';
const SHOULD_BOOTSTRAP =
  String(__ENV.K6_SETUP_BOOTSTRAP || 'true').toLowerCase() !== 'false';

const gatewayUnexpectedResponses = new Counter('gateway_unexpected_responses');
const gatewayBusinessFailureRate = new Rate('gateway_business_failure_rate');
const gatewayFlowDuration = new Trend('gateway_flow_duration', true);

const FLOW_PACKS = {
  all: [
    { weight: 4, run: gatewayHealth },
    { weight: 8, run: authValidateFlow },
    { weight: 18, run: listPaymentsFlow },
    { weight: 20, run: createAndGetPaymentFlow },
    { weight: 14, run: getBalanceFlow },
    { weight: 10, run: verifyBalanceFlow },
    { weight: 8, run: webhookHealthFlow },
    { weight: 8, run: listWebhookEndpointsFlow },
    { weight: 5, run: listWebhookDeliveriesFlow },
    { weight: 5, run: createAndUpdateWebhookEndpointFlow },
  ],
  payments: [
    { weight: 2, run: gatewayHealth },
    { weight: 20, run: listPaymentsFlow },
    { weight: 24, run: createAndGetPaymentFlow },
    { weight: 10, run: authValidateFlow },
  ],
  ledger: [
    { weight: 4, run: gatewayHealth },
    { weight: 18, run: getBalanceFlow },
    { weight: 18, run: verifyBalanceFlow },
    { weight: 10, run: authValidateFlow },
  ],
  webhooks: [
    { weight: 4, run: gatewayHealth },
    { weight: 10, run: webhookHealthFlow },
    { weight: 16, run: listWebhookEndpointsFlow },
    { weight: 10, run: listWebhookDeliveriesFlow },
    { weight: 14, run: createAndUpdateWebhookEndpointFlow },
    { weight: 8, run: authValidateFlow },
  ],
  auth: [
    { weight: 4, run: gatewayHealth },
    { weight: 24, run: authValidateFlow },
  ],
};

export const options = {
  discardResponseBodies: false,
  summaryTrendStats: ['avg', 'min', 'med', 'p(95)', 'p(99)', 'max'],
  scenarios: {
    gateway: scenarioFor(TEST_TYPE),
  },
  thresholds: thresholdsFor(TEST_TYPE),
  tags: {
    suite: 'gateway-load-testing',
    entrypoint: 'gateway',
    endpoint_pack: ENDPOINT_PACK,
    test_type: TEST_TYPE,
    testid: RUN_ID,
  },
};

export function setup() {
  const apiKey = resolveGatewayApiKey();
  const merchantId = validateGatewayKey(apiKey);

  return {
    apiKey,
    merchantId,
  };
}

export default function (state) {
  const flow = pickFlow();
  flow.run(state);

  if (THINK_TIME_SECONDS > 0) {
    sleep(THINK_TIME_SECONDS);
  }
}

function scenarioFor(testType) {
  switch (testType) {
    case 'spike':
      return {
        executor: 'ramping-vus',
        startVUs: 0,
        stages: scaleStages([
          { duration: '45s', target: 15 },
          { duration: '1m', target: 75 },
          { duration: '2m', target: 75 },
          { duration: '45s', target: 10 },
          { duration: '30s', target: 0 },
        ]),
        gracefulRampDown: '30s',
      };
    case 'soak':
      return {
        executor: 'constant-vus',
        vus: integerOverride(__ENV.K6_VUS, scaleCount(18)),
        duration: __ENV.K6_DURATION || '30m',
      };
    case 'stress':
      return {
        executor: 'ramping-vus',
        startVUs: scaleCount(10),
        stages: scaleStages([
          { duration: '2m', target: 30 },
          { duration: '4m', target: 80 },
          { duration: '4m', target: 120 },
          { duration: '2m', target: 140 },
          { duration: '2m', target: 0 },
        ]),
        gracefulRampDown: '45s',
      };
    case 'smoke':
    default:
      return {
        executor: 'constant-vus',
        vus: integerOverride(__ENV.K6_VUS, scaleCount(5)),
        duration: __ENV.K6_DURATION || '3m',
      };
  }
}

function thresholdsFor(testType) {
  const p95LatencyMs = testType === 'smoke' ? 1500 : testType === 'soak' ? 2500 : 3500;
  const p99LatencyMs = testType === 'stress' ? 6000 : 4500;
  const failureRate = testType === 'smoke' ? 0.02 : 0.1;

  return {
    http_req_failed: [`rate<${failureRate}`],
    http_req_duration: [`p(95)<${p95LatencyMs}`, `p(99)<${p99LatencyMs}`],
    checks: [`rate>${1 - failureRate}`],
    gateway_business_failure_rate: [`rate<${failureRate}`],
  };
}

function gatewayHealth() {
  sendGatewayRequest('GET', '/health', '', undefined, {
    endpoint: 'gateway.health',
    flow: 'gateway.health',
    authMode: 'public',
  }, [200]);
}

function authValidateFlow(state) {
  sendGatewayRequest(
    'POST',
    '/v1/auth/internal/validate',
    state.apiKey,
    { api_key: state.apiKey },
    {
      endpoint: 'auth.validate',
      flow: 'auth.validate',
    },
    [200],
  );
}

function listPaymentsFlow(state) {
  sendGatewayRequest(
    'GET',
    '/v1/payments?page=1&limit=20&status=completed',
    state.apiKey,
    undefined,
    {
      endpoint: 'payments.list',
      flow: 'payments.list',
    },
    [200],
  );
}

function createAndGetPaymentFlow(state) {
  const startedAt = Date.now();
  const createTags = metricTags('payments.create', 'payments.create_get');
  const createResponse = requestGateway(
    'POST',
    '/v1/payments',
    state.apiKey,
    paymentPayload(),
    createTags,
    [201, 400],
  );
  const createPassed = check(
    createResponse,
    {
      'status is expected': (res) => [201, 400].indexOf(res.status) >= 0,
    },
    createTags,
  );
  if (!createPassed) {
    gatewayUnexpectedResponses.add(1, createTags);
    gatewayBusinessFailureRate.add(1, createTags);
    return;
  }

  const payment = safeJson(createResponse);

  if (createResponse.status === 400) {
    const declined = check(
      payment,
      {
        'payment decline is expected business outcome': (body) =>
          body && body.error && body.error.code === 'payment_failed',
      },
      createTags,
    );

    gatewayUnexpectedResponses.add(declined ? 0 : 1, createTags);
    gatewayBusinessFailureRate.add(declined ? 0 : 1, createTags);
    return;
  }

  const paymentId = payment.id || payment.ID;
  const flowTags = metricTags('payments.create_get', 'payments.create_get');
  const hasPaymentId = check(
    payment,
    {
      'response contains payment id': (body) => Boolean(body.id || body.ID),
    },
    flowTags,
  );

  if (!hasPaymentId) {
    gatewayUnexpectedResponses.add(1, flowTags);
    gatewayBusinessFailureRate.add(1, flowTags);
    return;
  }

  gatewayBusinessFailureRate.add(0, flowTags);

  sendGatewayRequest(
    'GET',
    `/v1/payments/${paymentId}`,
    state.apiKey,
    undefined,
    {
      endpoint: 'payments.get',
      flow: 'payments.create_get',
    },
    [200],
  );

  gatewayFlowDuration.add(Date.now() - startedAt, flowTags);
}

function getBalanceFlow(state) {
  sendGatewayRequest('GET', '/v1/balance', state.apiKey, undefined, {
    endpoint: 'ledger.balance',
    flow: 'ledger.balance',
  }, [200]);
}

function verifyBalanceFlow(state) {
  sendGatewayRequest(
    'GET',
    '/v1/balance/verify',
    state.apiKey,
    undefined,
    {
      endpoint: 'ledger.verify',
      flow: 'ledger.verify',
    },
    [200],
  );
}

function webhookHealthFlow(state) {
  sendGatewayRequest(
    'GET',
    '/v1/webhooks/health',
    state.apiKey,
    undefined,
    {
      endpoint: 'webhooks.health',
      flow: 'webhooks.health',
    },
    [200],
  );
}

function listWebhookEndpointsFlow(state) {
  sendGatewayRequest(
    'GET',
    '/v1/webhooks/endpoints',
    state.apiKey,
    undefined,
    {
      endpoint: 'webhooks.endpoints.list',
      flow: 'webhooks.endpoints.list',
    },
    [200],
  );
}

function listWebhookDeliveriesFlow(state) {
  sendGatewayRequest(
    'GET',
    '/v1/webhooks/deliveries?page=1&limit=20',
    state.apiKey,
    undefined,
    {
      endpoint: 'webhooks.deliveries.list',
      flow: 'webhooks.deliveries.list',
    },
    [200],
  );
}

function createAndUpdateWebhookEndpointFlow(state) {
  const startedAt = Date.now();
  const created = sendGatewayRequest(
    'POST',
    '/v1/webhooks/endpoints',
    state.apiKey,
    webhookEndpointPayload(),
    {
      endpoint: 'webhooks.endpoints.create',
      flow: 'webhooks.endpoints.create_update',
    },
    [201],
  );
  const endpoint = safeJson(created);
  const endpointId = endpoint.id;
  const flowTags = metricTags(
    'webhooks.endpoints.create_update',
    'webhooks.endpoints.create_update',
  );
  const hasEndpointId = check(
    endpoint,
    {
      'response contains endpoint id': (body) => Boolean(body.id),
    },
    flowTags,
  );

  if (!hasEndpointId) {
    gatewayUnexpectedResponses.add(1, flowTags);
    gatewayBusinessFailureRate.add(1, flowTags);
    return;
  }

  gatewayBusinessFailureRate.add(0, flowTags);

  sendGatewayRequest(
    'PUT',
    `/v1/webhooks/endpoints/${endpointId}`,
    state.apiKey,
    {
      events: ['refund.succeeded'],
      description: `k6 update ${uniqueSuffix()}`,
      isActive: false,
    },
    {
      endpoint: 'webhooks.endpoints.update',
      flow: 'webhooks.endpoints.create_update',
    },
    [200],
  );

  sendGatewayRequest(
    'DELETE',
    `/v1/webhooks/endpoints/${endpointId}`,
    state.apiKey,
    undefined,
    {
      endpoint: 'webhooks.endpoints.delete',
      flow: 'webhooks.endpoints.create_update',
    },
    [204],
  );

  gatewayFlowDuration.add(Date.now() - startedAt, flowTags);
}

function sendGatewayRequest(method, path, apiKey, body, tagConfig, expectedStatuses) {
  const tags = metricTags(tagConfig.endpoint, tagConfig.flow, tagConfig.phase);
  const response = requestGateway(method, path, apiKey, body, tags, expectedStatuses);

  const passed = check(
    response,
    {
      'status is expected': (res) => expectedStatuses.indexOf(res.status) >= 0,
    },
    tags,
  );

  gatewayUnexpectedResponses.add(passed ? 0 : 1, tags);
  gatewayBusinessFailureRate.add(passed ? 0 : 1, tags);

  return response;
}

function requestGateway(method, path, apiKey, body, tags, expectedStatuses) {
  const url = `${BASE_URL}${path}`;
  const payload = body === undefined ? null : JSON.stringify(body);
  const headers = {
    Accept: 'application/json',
  };

  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  if (apiKey) {
    headers.Authorization = `Bearer ${apiKey}`;
  }

  const response = http.request(method, url, payload, {
    headers,
    tags,
    responseCallback: http.expectedStatuses(...expectedStatuses),
    timeout: HTTP_TIMEOUT,
  });

  return response;
}

function resolveGatewayApiKey() {
  const configuredKey = String(__ENV.K6_GATEWAY_API_KEY || '').trim();

  if (configuredKey) {
    return configuredKey;
  }

  if (!SHOULD_BOOTSTRAP) {
    fail('K6_GATEWAY_API_KEY is required when K6_SETUP_BOOTSTRAP=false');
  }

  if (!INTERNAL_AUTH_SECRET) {
    fail(
      'K6_GATEWAY_API_KEY is not set and K6_INTERNAL_AUTH_SECRET/INTERNAL_AUTH_SECRET is empty',
    );
  }

  const requestId = `req_k6_bootstrap_${Date.now()}`;
  const body = {
    merchant_name: 'k6 gateway load test',
    merchant_email: `k6+${Date.now()}@payflow.local`,
    scope: 'full',
    expires_at: '2027-12-31T23:59:59Z',
  };
  const targetUrl = `${AUTH_BASE_URL}/api/v1/auth/keys/`;
  const rawBody = JSON.stringify(body);
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const bodySha256 = crypto.sha256(rawBody, 'hex');
  const canonical = [
    'POST',
    pathWithQuery(targetUrl),
    timestamp,
    requestId,
    '',
    bodySha256,
  ].join('\n');
  const signature = crypto.hmac(
    'sha256',
    INTERNAL_AUTH_SECRET,
    canonical,
    'hex',
  );

  const response = http.post(targetUrl, rawBody, {
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      'x-request-id': requestId,
      'x-internal-timestamp': timestamp,
      'x-internal-body-sha256': bodySha256,
      'x-internal-signature': signature,
    },
    tags: metricTags('auth.bootstrap', 'auth.bootstrap', 'setup'),
    timeout: HTTP_TIMEOUT,
  });

  const passed = check(
    response,
    {
      'bootstrap returned 201': (res) => res.status === 201,
    },
    metricTags('auth.bootstrap', 'auth.bootstrap', 'setup'),
  );

  if (!passed) {
    fail(`Auth bootstrap failed with status ${response.status}`);
  }

  const created = safeJson(response);
  if (!created.api_key) {
    fail('Auth bootstrap response did not include api_key');
  }

  return created.api_key;
}

function validateGatewayKey(apiKey) {
  const response = sendGatewayRequest(
    'POST',
    '/v1/auth/internal/validate',
    apiKey,
    { api_key: apiKey },
    {
      endpoint: 'auth.validate',
      flow: 'auth.validate',
      phase: 'setup',
    },
    [200],
  );

  const body = safeJson(response);
  const merchantId = body.merchant_id || body.MerchantID || String(__ENV.K6_MERCHANT_ID || '').trim();

  if (!merchantId) {
    fail('Gateway validate response did not include merchant_id');
  }

  return merchantId;
}

function pickFlow() {
  const flows = FLOW_PACKS[ENDPOINT_PACK] || FLOW_PACKS.all;
  const totalWeight = flows.reduce((sum, flow) => sum + flow.weight, 0);
  let choice = Math.random() * totalWeight;

  for (const flow of flows) {
    choice -= flow.weight;
    if (choice <= 0) {
      return flow;
    }
  }

  return flows[flows.length - 1];
}

function paymentPayload() {
  const suffix = uniqueSuffix();

  return {
    amount: 5000,
    customer_email: `customer+${suffix}@example.com`,
    customer_name: 'Ada Lovelace',
    metadata: {
      order_id: `ord_${suffix}`,
      test_run: RUN_ID,
    },
    idempotency_key: `idem_${suffix}`,
  };
}

function webhookEndpointPayload() {
  const suffix = uniqueSuffix();

  return {
    url: `https://example.com/payment-reference/webhook/${suffix}`,
    events: ['refund.succeeded', 'refund.failed'],
    description: `k6 endpoint ${suffix}`,
  };
}

function uniqueSuffix() {
  return [
    RUN_ID,
    exec.vu.idInTest,
    exec.scenario.iterationInTest,
    Date.now(),
    Math.floor(Math.random() * 1000000),
  ].join('-');
}

function metricTags(endpoint, flow, phase) {
  return {
    endpoint,
    flow,
    phase: phase || 'load',
    name: endpoint,
  };
}

function safeJson(response) {
  if (!response || !response.body) {
    return {};
  }

  try {
    return response.json();
  } catch (error) {
    return {};
  }
}

function pathWithQuery(targetUrl) {
  const stripped = String(targetUrl).replace(/^https?:\/\/[^/]+/i, '');
  return stripped.startsWith('/') ? stripped : `/${stripped}`;
}

function trimTrailingSlash(value) {
  return String(value).replace(/\/+$/, '');
}

function positiveNumber(value, fallback) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function integerOverride(value, fallback) {
  if (value === undefined || value === null || String(value).trim() === '') {
    return fallback;
  }

  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function scaleCount(value) {
  return Math.max(1, Math.round(value * SCALE_MULTIPLIER));
}

function scaleStages(stages) {
  return stages.map((stage) => ({
    duration: stage.duration,
    target: stage.target === 0 ? 0 : scaleCount(stage.target),
  }));
}

function defaultThinkTimeSeconds() {
  if (__ENV.K6_THINK_TIME_SECONDS !== undefined) {
    return positiveNumber(__ENV.K6_THINK_TIME_SECONDS, 0.2);
  }

  switch (TEST_TYPE) {
    case 'spike':
    case 'stress':
      return 0.05;
    case 'soak':
      return 0.5;
    case 'smoke':
    default:
      return 0.25;
  }
}