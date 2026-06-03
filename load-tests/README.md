# Gateway Load Testing with k6

This repo now has a k6 suite that drives traffic through the gateway and publishes live metrics to Prometheus using the k6 remote-write output. Local Grafana is bound to `127.0.0.1:3001` by default, and the same k6 run can be redirected to Grafana Cloud by swapping the remote-write endpoint and credentials.

## What was added

- `docker-compose.yml`
  - Exposes Prometheus on `localhost:9090`.
  - Binds Grafana to `127.0.0.1:3001`.
  - Enables Prometheus `--web.enable-remote-write-receiver` for k6 metrics.
- `docker-compose.loadtest.yml`
  - Raises the gateway rate limit default to `5000` while the load-test overlay is active, using `LOADTEST_RATE_LIMIT_MAX` so it does not get masked by the base `.env` value.
  - Adds an on-demand `k6` service under the `loadtest` profile.
  - Pins `NO_PROXY` for internal compose service names so k6 traffic does not leak through host-level proxy settings.
- `load-tests/k6/main.js`
  - Covers smoke, spike, soak, and stress profiles.
  - Exercises multiple gateway-backed endpoint groups: auth, payments, ledger, and webhooks.
  - Bootstraps a full-scope API key automatically through the auth service if you do not provide one.
- `infrastructure/grafana/dashboards/k6-gateway-load-testing.json`
  - Provisioned dashboard for request rate, latency, active VUs, and endpoint-level failures.

## Start the stack

1. Create a local env file if you do not already have one.

   ```bash
   cp .env.example .env
   ```

2. Start the stack with the load-test overlay so the gateway rate limit is high enough for meaningful runs.

   ```bash
   docker compose -f docker-compose.yml -f docker-compose.loadtest.yml up -d --build
   ```

3. Open Grafana locally at `http://localhost:3001`.

   - Username: `admin`
   - Password: value of `GRAFANA_PASSWORD` in your `.env` file, default `payment_reference`

4. In Grafana, open the provisioned dashboard named `Gateway Load Testing`.

## Run k6 profiles

The easiest path is to run the bundled k6 container inside the compose network.

### Smoke

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml --profile loadtest run --rm \
  -e K6_TEST_TYPE=smoke \
  -e K6_RUN_ID=gateway-smoke-$(date +%Y%m%d%H%M%S) \
  k6
```

### Spike

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml --profile loadtest run --rm \
  -e K6_TEST_TYPE=spike \
  -e K6_RUN_ID=gateway-spike-$(date +%Y%m%d%H%M%S) \
  k6
```

### Soak

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml --profile loadtest run --rm \
  -e K6_TEST_TYPE=soak \
  -e K6_RUN_ID=gateway-soak-$(date +%Y%m%d%H%M%S) \
  k6
```

### Stress

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml --profile loadtest run --rm \
  -e K6_TEST_TYPE=stress \
  -e K6_RUN_ID=gateway-stress-$(date +%Y%m%d%H%M%S) \
  k6
```

## Toggle the target mix

Use `K6_ENDPOINT_PACK` to narrow the suite without editing the script.

- `all` (default): auth, payments, ledger, and webhooks mixed together.
- `payments`: payment-heavy flow with create, get, and list.
- `ledger`: balance and verify endpoints.
- `webhooks`: endpoint management and delivery listing.
- `auth`: gateway auth validation focus.

Example:

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml --profile loadtest run --rm \
  -e K6_TEST_TYPE=smoke \
  -e K6_ENDPOINT_PACK=payments \
  -e K6_SCALE_MULTIPLIER=1.5 \
  -e K6_RUN_ID=gateway-payments-smoke-$(date +%Y%m%d%H%M%S) \
  k6
```

## Useful overrides

- `K6_GATEWAY_API_KEY`
  - Reuse an existing gateway API key and skip the bootstrap key creation.
- `K6_VUS`
  - Override the VU count for `smoke` and `soak`.
- `K6_DURATION`
  - Override the duration for `smoke` and `soak`.
- `K6_SCALE_MULTIPLIER`
  - Scales the built-in spike/stress/soak presets up or down.
- `K6_THINK_TIME_SECONDS`
  - Override the pause between iterations.
- `RATE_LIMIT_MAX`
  - Base development rate limit for normal runs.
- `LOADTEST_RATE_LIMIT_MAX`
  - Load-test-only gateway rate limit override used by `docker-compose.loadtest.yml`.

Example with a larger gateway limit:

```bash
LOADTEST_RATE_LIMIT_MAX=12000 docker compose -f docker-compose.yml -f docker-compose.loadtest.yml up -d --build
```

## Send results to Grafana Cloud

The bundled k6 service defaults to local Prometheus. To publish the same run to Grafana Cloud instead, override the remote-write settings when you start k6.

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml --profile loadtest run --rm \
  -e K6_TEST_TYPE=stress \
  -e K6_RUN_ID=gateway-cloud-stress-$(date +%Y%m%d%H%M%S) \
  -e K6_PROMETHEUS_RW_SERVER_URL=https://<prometheus-endpoint>/api/prom/push \
  -e K6_PROMETHEUS_RW_USERNAME=<grafana-cloud-instance-id> \
  -e K6_PROMETHEUS_RW_PASSWORD=<grafana-cloud-api-key> \
  k6
```

Notes:

- The local Grafana instance remains available on `localhost:3001` from the host machine only.
- Grafana Cloud will receive the run tagged with the same `testid`, `test_type`, and `endpoint` labels, so you can import the same dashboard JSON there if you want the same layout.

## Endpoint coverage

The default `all` mix currently exercises these gateway routes:

- `GET /health`
- `POST /v1/auth/internal/validate`
- `GET /v1/payments?page=1&limit=20&status=completed`
- `POST /v1/payments`
- `GET /v1/payments/:id`
- `GET /v1/balance`
- `GET /v1/balance/verify`
- `GET /v1/webhooks/health`
- `GET /v1/webhooks/endpoints`
- `GET /v1/webhooks/deliveries?page=1&limit=20`
- `POST /v1/webhooks/endpoints`
- `PUT /v1/webhooks/endpoints/:id`
- `DELETE /v1/webhooks/endpoints/:id`

## Bank mock behavior

The bundled bank mock intentionally declines about 20% of authorization attempts while still returning a valid business response. The k6 suite now treats `POST /v1/payments` responses with `400` and error code `payment_failed` as expected business declines, not load-test failures. Unexpected 4xx/5xx responses still count against the run.

## Webhook load behavior

The mixed gateway suite uses refund-scoped webhook events for CRUD coverage and deletes each test-created endpoint after the update step. That prevents the test from accumulating hundreds of active payment webhooks and accidentally turning each payment event into a large delivery fan-out inside the webhook service.

## If you want to run k6 outside Docker

You can, but the bootstrap flow needs either:

- a reachable auth service URL for `K6_AUTH_BASE_URL`, or
- an already-issued `K6_GATEWAY_API_KEY`.

Example with a local k6 binary and an existing API key:

```bash
K6_BASE_URL=http://localhost:3000 \
K6_GATEWAY_API_KEY=<existing-key> \
K6_PROMETHEUS_RW_SERVER_URL=http://localhost:9090/api/v1/write \
K6_PROMETHEUS_RW_TREND_STATS='p(95),p(99),avg,max' \
k6 run -o experimental-prometheus-rw load-tests/k6/main.js
```
