# SkillRoom MVP

SkillRoom is a production-oriented MVP for evaluating React skills through real code execution. The system uses:

- a Go modular monolith for product logic and APIs
- a separate runner service for isolated challenge execution
- PostgreSQL for system-of-record data
- Redis for cache, rate limiting, and hot operational state
- Next.js for the browser client

## Current guarantees

- challenge correctness comes from real test execution in the runner
- scoring is computed in `internal/evaluation` from execution output
- quality score combines lint output and task-specific quality checks from real test results
- runtime efficiency is exposed as `execution_cost_score` and is based on execution cost normalized per challenge, not candidate solve speed
- challenge variants are deterministic per user/template/attempt seed
- challenge bank includes 12+ real React and JavaScript templates with visible and hidden tests
- room items reflect stored skill data
- confidence and HR views include explanation data, not just raw numbers
- monetization foundation is wired into the backend via plans, subscriptions, AI usage metering, and developer cosmetic inventory

## Main entrypoints

- `cmd/backend`: main API server
- `cmd/runner-service`: isolated execution service
- `cmd/devstack`: local two-process Go bootstrap

## Main docs

- `docs/architecture.md`
- `docs/backend-agent.md`
- `docs/data-agent.md`
- `docs/ai-agent.md`
- `docs/frontend-agent.md`
- `docs/devops-agent.md`
- `api/openapi.yaml`

## Monetization foundation

The current MVP ships the monetization architecture layer without billing integration:

- default role-based plans for developers and recruiters
- persisted subscriptions and entitlement snapshots
- AI usage metering for developer hints/explanations and HR-side AI actions
- developer cosmetic catalog, ownership, equipped state, and plan-gated room customization

HR candidate unlock flow is active. Billing and paid developer cosmetic purchases are intentionally deferred to later product sprints.

## Local stack

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Open `http://localhost:3000` for the Next.js frontend.
The backend root route only redirects to the frontend when `FRONTEND_REDIRECT_URL` is set.

For a backend-only local boot without PostgreSQL or Redis, set `ALLOW_INSECURE_BOOT=true`. The default startup path now expects the full production-oriented stack.

## Testing

```bash
go test ./...
node --check frontend/lib/client.js
node --check frontend/app/workspace/workspace-client.js
```

Docker-backed end-to-end runner verification is available with:

```bash
go test ./internal/backend -run TestRealRunnerEndToEnd -count=1
```
