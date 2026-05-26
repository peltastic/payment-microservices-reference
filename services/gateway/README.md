# Gateway Service

NestJS API gateway for the Payment Microservices Reference stack.

Responsibilities:

- Validate merchant API keys through the auth service.
- Enforce route-level scopes before proxying requests.
- Add merchant context and signed internal request headers.
- Expose health and metrics for the local stack.

Run locally from the repository root with `docker compose up --build`.
