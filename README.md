# Payflow - Payment Microservices Reference Platform

**Reliable payment orchestration, event-driven accounting, and merchant webhooks in one production-style microservices stack.**

Payflow is a reference implementation of a modern payments platform built around clear service boundaries, strong operational controls, and end-to-end observability. It models the core backend concerns behind a PSP or internal payment gateway: merchant authentication, payment creation, provider authorization, immutable ledger updates, signed event distribution, webhook delivery, and recovery workflows.

**Architectural Overview Diagram:** [View the platform architecture in Excalidraw](https://excalidraw.com/#json=smmq0b8SlWely_eZv-2FN,HCvjDUPG6GQi2oykP_nBtQ)

## Table of Contents

1. [Platform Vision](#platform-vision)
2. [What This Codebase Does](#what-this-codebase-does)
3. [Architecture](#architecture)
4. [Core Payment Flow](#core-payment-flow)
5. [Service Breakdown](#service-breakdown)
6. [Key Capabilities](#key-capabilities)
7. [Technology Stack](#technology-stack)
8. [Repository Structure](#repository-structure)
9. [Quick Start](#quick-start)
10. [API Surface](#api-surface)
11. [Observability and Operations](#observability-and-operations)
12. [Testing and CI](#testing-and-ci)
13. [Production Notes](#production-notes)

## Platform Vision

Payflow demonstrates how to structure a payment backend as a set of focused, independently deployable services instead of one large application. The platform is designed to show how merchant-facing APIs, internal trust boundaries, asynchronous eventing, ledger integrity, idempotent payment handling, and webhook fan-out can work together in a realistic system.

This repository is especially useful for teams building:

- Payment gateways and internal commerce platforms
- Event-driven fintech systems
- Merchant API products with scoped credentials
- Accounting-aware payment pipelines
- Webhook-heavy integrations that need retries, signing, and dead-letter handling

## What This Codebase Does

At a high level, the platform accepts authenticated merchant API traffic through a gateway, validates scoped API keys, creates payments with strict idempotency protections, simulates bank authorization, emits signed payment events to Kafka, updates a ledger for successful payments, and pushes merchant-specific webhooks through a retryable delivery pipeline.

The current implementation includes:

- Merchant API key issuance, validation, caching, expiration, and revocation
- Gateway-level auth enforcement, request signing, and route-aware scope checks
- Payment creation with idempotency locks and persisted replayable responses
- Simulated bank authorization with realistic decline behavior
- Signed Kafka event publication for payment lifecycle events
- Double-entry-style journal creation and merchant balance materialization
- Merchant webhook endpoint management with encrypted secrets at rest
- Retryable webhook delivery with HMAC signatures and dead-letter capture
- Recovery jobs for unpublished payment events
- Prometheus metrics, Jaeger tracing, Grafana dashboards, and load testing

## Architecture

The system is composed of six application services plus supporting infrastructure:

### 1. Gateway (`NestJS`)

The single ingress point for merchant traffic.

- Validates bearer API keys against the auth service
- Enforces route-specific scopes such as `payments:write` and `webhooks:read`
- Adds merchant context to downstream requests
- Signs internal service-to-service calls
- Exposes health and Prometheus metrics endpoints

### 2. Auth Service (`Go + Gin`)

The merchant credential authority for the platform.

- Creates merchants and issues API keys
- Stores only hashed API key material
- Supports scoped keys, expiry, revocation, and last-used tracking
- Caches validation results in Redis
- Exposes an internal validation API used by the gateway

### 3. Payment Service (`Go + Gin`)

The payment orchestration engine.

- Accepts merchant payment creation requests
- Enforces idempotency with Redis locks plus durable idempotency records
- Persists payment state in Postgres
- Calls the mock bank provider for authorization
- Publishes signed `payment.succeeded` and `payment.failed` events to Kafka
- Runs a recovery worker that republishes missed events

### 4. Ledger Service (`Go + Gin`)

The accounting and balance subsystem.

- Consumes payment events from Kafka
- Validates event signatures before processing
- Creates debit and credit journal entries for successful payments
- Maintains merchant materialized balances
- Tracks processed events for exactly-once style replay protection
- Exposes balance and verification endpoints

### 5. Webhook Service (`NestJS + BullMQ + Prisma`)

The outbound event delivery plane for merchants.

- Lets merchants register webhook endpoints per event type
- Encrypts webhook secrets at rest
- Verifies Kafka event signatures before fan-out
- Queues webhook deliveries for reliable async dispatch
- Retries failed deliveries with backoff
- Stores dead-letter events for manual replay and inspection
- Rejects unsafe destinations such as private/internal targets

### 6. Bank Mock (`NestJS`)

The demonstration provider boundary.

- Simulates payment authorization responses
- Intentionally declines a portion of requests
- Lets the stack exercise provider success, failure, retries, and observability without real banking dependencies

## Core Payment Flow

1. A merchant sends a request to the gateway with a bearer API key.
2. The gateway validates the key with the auth service and checks the required scope.
3. The payment service receives the request with merchant context and enforces idempotency.
4. The payment service stores the payment in `processing` state and calls the bank mock.
5. The bank response marks the payment as `completed` or `failed`.
6. The payment service publishes a signed Kafka event for the outcome.
7. The ledger service consumes `payment.succeeded` events and creates journal entries plus pending balance updates.
8. The webhook service consumes signed events, finds subscribed endpoints, and schedules outbound deliveries.
9. Failed webhook attempts are retried and eventually dead-lettered if exhausted.
10. A recovery loop republishes any payment event that was persisted but not successfully emitted.

## Service Breakdown

| Service | Port | Primary Responsibility |
|---|---:|---|
| `gateway` | `3000` | Merchant-facing API gateway and auth enforcement |
| `auth` | `4001` | Merchant creation, API key issuance, validation, revocation |
| `payment` | `4002` | Payment lifecycle orchestration and event publication |
| `ledger` | `4003` | Journal entries, balances, and event processing |
| `webhook` | `4004` | Webhook endpoint management and delivery pipeline |
| `bank-mock` | `5001` | Simulated provider authorization API |
| `postgres` | `5432` | Durable application storage |
| `redis` | `6379` | Caching, locks, queues |
| `kafka` | `9092` | Event transport backbone |
| `prometheus` | `9090` | Metrics collection |
| `grafana` | `3001` | Dashboards and monitoring |

## Key Capabilities

### Payments

- Merchant-scoped payment creation and retrieval
- Idempotent `POST /payments` behavior across retries
- Durable response replay via stored idempotency records
- Support for successful and declined authorization flows
- Event publication decoupled from synchronous request handling

### Identity and Access

- Merchant creation with generated API keys
- SHA-256 hashed key storage
- Scope-aware auth for `auth`, `payments`, `ledger`, and `webhooks`
- Key expiration and revocation workflows
- Redis-backed validation caching for lower latency

### Ledger Integrity

- Signed event verification before accounting writes
- Processed-event tracking to prevent duplicate ledger mutations
- Separate journal and materialized balance models
- Balance verification endpoint to compare derived and stored states

### Webhooks

- Merchant endpoint CRUD
- Secret generation and encryption
- Event-type subscription filtering
- HMAC-signed outbound delivery
- Retry queue, dead-letter capture, and manual retry endpoint
- URL safety controls to reduce SSRF risk

### Operations

- OpenTelemetry tracing across services
- Prometheus metrics for request, Kafka, payment, and webhook behavior
- Grafana dashboards for service health, payment flow, Kafka, webhooks, and k6 load tests
- Docker Compose overlays for development, production-style runs, and load testing

## Technology Stack

### Application Services

- **Gateway:** NestJS 11, Axios, Prometheus, OpenTelemetry
- **Auth:** Go 1.25, Gin, GORM, Redis, Postgres
- **Payment:** Go 1.25, Gin, GORM, Redis, Kafka
- **Ledger:** Go 1.25, Gin, GORM, Kafka
- **Webhook:** NestJS 11, Prisma, BullMQ, Redis, KafkaJS
- **Bank Mock:** NestJS 11

### Infrastructure

- **PostgreSQL 15** for durable storage
- **Redis 7** for cache, locking, and queues
- **Kafka** for asynchronous event transport
- **Jaeger** for distributed tracing
- **Prometheus + Grafana** for metrics and dashboards
- **Docker Compose** for local orchestration
- **k6** for smoke, spike, soak, and stress testing

## Repository Structure

```text
payflow/
├── services/
│   ├── gateway/        # NestJS API gateway
│   ├── auth/           # Go auth and key management service
│   ├── payment/        # Go payment orchestration service
│   ├── ledger/         # Go ledger and balance service
│   ├── webhook/        # NestJS webhook management + delivery service
│   └── bank-mock/      # Mock payment provider
├── infrastructure/
│   ├── postgres/       # DB bootstrap SQL
│   ├── prometheus/     # Prometheus config and alerts
│   └── grafana/        # Provisioned dashboards and datasources
├── load-tests/         # k6 scenarios and documentation
├── postman/            # Postman collection for local API exploration
├── docker-compose.yml
├── docker-compose.prod.yml
├── docker-compose.loadtest.yml
└── .github/workflows/  # CI, test, and security automation
```

## Quick Start

### Prerequisites

- Docker and Docker Compose

### 1. Configure local environment

```bash
cp .env.example .env
```

### 2. Start the stack

```bash
docker compose up --build
```

### 3. Verify core services

- Gateway: `http://localhost:3000/health`
- Auth: `http://localhost:4001/health`
- Payment: `http://localhost:4002/health`
- Ledger: `http://localhost:4003/health`
- Webhook: `http://localhost:4004/health`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3001`

### 4. Provision a merchant API key

API key creation is modeled as an administrative capability exposed by the auth service inside the trusted internal boundary. In practice, teams usually automate this through an admin tool, internal service, or the included Postman collection during local testing.

### 5. Create a payment

Call the gateway `POST /v1/payments` endpoint with:

- `Authorization: Bearer <api_key>`
- `Idempotency-Key: <unique_key>`

The payment service will persist the request, authorize against the bank mock, and publish the resulting event stream.

## API Surface

The gateway fronts the platform and exposes merchant-facing routes under `/v1`.

### Auth

- `POST /v1/auth/keys`
- `POST /v1/auth/keys/:id/revoke`

### Payments

- `POST /v1/payments`
- `GET /v1/payments`
- `GET /v1/payments/:id`

### Ledger

- `GET /v1/balance`
- `GET /v1/balance/verify`
- `POST /v1/transactions/payment-succeeded`

### Webhooks

- `POST /v1/webhooks/endpoints`
- `GET /v1/webhooks/endpoints`
- `PUT /v1/webhooks/endpoints/:id`
- `DELETE /v1/webhooks/endpoints/:id`
- `GET /v1/webhooks/deliveries`
- `GET /v1/webhooks/deliveries/dead-letter`
- `POST /v1/webhooks/deliveries/:id/retry`

## Observability and Operations

This repository is built like an operations-aware platform, not just an API demo.

- Every service exposes `/health`
- Prometheus metrics are available across the stack
- Jaeger receives OpenTelemetry traces from both Node.js and Go services
- Grafana dashboards are provisioned for payment flow, service health, Kafka health, webhook health, and k6 load testing
- The payment service includes an event recovery loop for unpublished events
- The webhook service exposes delivery history and dead-letter retry operations

## Testing and CI

The repository includes unit, integration, and end-to-end coverage across services, plus security checks in CI.

- Node services run unit and integration suites with Jest
- Go services run tests with `go test`
- Prisma client generation is part of webhook CI
- `npm audit` and `govulncheck` are part of the GitHub Actions pipeline
- Repository hygiene checks prevent committed local env files
- Load testing is supported through the bundled `k6` setup

Run the full local stack and then use the load-test overlay when you want gateway-level performance validation:

```bash
docker compose -f docker-compose.yml -f docker-compose.loadtest.yml up -d --build
```

## Production Notes

This codebase is a strong reference platform for payment systems, but it is still intentionally demo-friendly in a few places:

- The bank integration is a mock service, not a live acquirer
- Currency handling is currently centered on `NGN`
- The default compose setup is optimized for local development and teaching
- Production secrets, network policy, and infrastructure hardening should be handled outside the repository defaults

For production-style runs, use the stricter environment contract in `docker-compose.prod.yml`, which requires explicit secrets for internal auth, Redis, webhook encryption, and event signing.

## Why This Repo Stands Out

Unlike many payment demos that stop at a single API endpoint, Payflow models the downstream systems that make payment platforms trustworthy in practice:

- Idempotency that survives retries
- Event signatures across service boundaries
- Ledger-aware processing rather than status-only persistence
- Merchant-scoped webhook delivery with retries and dead letters
- Real observability, CI, and load testing built into the repository

It is best understood as a professional reference architecture for backend payments infrastructure.
