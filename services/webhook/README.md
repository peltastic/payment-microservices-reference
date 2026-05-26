Webhook Service
===============

NestJS worker/API service for merchant webhook endpoints.

Responsibilities:

- Manage merchant webhook endpoint registration.
- Encrypt endpoint secrets at rest.
- Verify signed payment events from Kafka.
- Queue, retry, sign, and record webhook deliveries.
- Reject unsafe webhook destinations before delivery.

Run locally from the repository root with `docker compose up --build`.
