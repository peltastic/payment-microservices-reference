# Bank Mock Service

Teaching/demo bank provider used by the Payment Microservices Reference stack.

The mock stays in the production-mode demo intentionally. It lets the project
show provider-boundary patterns, retries, idempotency, logging, and observability
without real banking credentials or live money movement.

Run locally from the repository root with `docker compose up --build`.
