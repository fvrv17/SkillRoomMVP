# Data Agent

## Design

- PostgreSQL is the system of record
- Redis handles cache, rate limits, and hot operational state
- room, ranking, telemetry, evaluation, and HR data are persisted centrally

## Implementation

Core files:

- `internal/backend/schema.sql`
- `internal/backend/postgres.go`
- `internal/backend/ops.go`
- `internal/backend/app.go`

Persisted domains:

- users and profiles
- skills and room items
- challenge templates, variants, instances, telemetry, submissions, evaluations
- rankings snapshots
- friendships and chat history
- companies, jobs, shortlists
- AI audit interactions

## Tests

- `internal/backend/ops_test.go`
- `internal/backend/app_test.go`

Run:

```bash
go test ./...
```

## Tradeoffs

- Redis is optional in local development because the backend has an in-memory fallback
- PostgreSQL migrations are embedded in `schema.sql`, which keeps startup simple but is less flexible than a dedicated migration tool
