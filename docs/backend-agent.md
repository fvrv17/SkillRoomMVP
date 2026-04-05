# Backend Agent

## Design

The backend is a Go modular monolith. It owns:

- auth
- challenge sessions
- deterministic variant generation
- runner orchestration
- scoring and skill updates
- room state
- rankings
- anti-cheat signals
- HR search and shortlist flows

The runner is a separate service and is the only execution environment.

## Implementation

Core files:

- `cmd/backend/main.go`
- `internal/backend/app.go`
- `internal/backend/models.go`
- `internal/backend/challenges.go`
- `internal/backend/runner_integration.go`
- `internal/backend/evaluator.go`
- `internal/evaluation/evaluation.go`
- `internal/runner/runner.go`
- `cmd/runner-service/main.go`

Important backend behaviors:

- source submissions are packaged into a runner workspace
- hidden and visible tests execute in the runner, not in the API
- score is computed from runner output plus persisted history
- confidence is updated from task count, consistency, and anomaly signals
- room state is derived from actual skill scores
- the room slot is now `chair -> architecture`

## Tests

- `internal/backend/app_test.go`
- `internal/backend/challenges_test.go`
- `internal/backend/ai_test.go`
- `internal/runner/runner_test.go`
- `internal/evaluation/evaluation_test.go`

Run:

```bash
go test ./...
```

## Tradeoffs

- the backend is intentionally monolithic for MVP speed and operational simplicity
- challenge execution is isolated, but challenge calibration still depends on keeping template tests strong
- confidence remains heuristic, but correctness and score are execution-based
