# AI Agent

## Design

OpenAI is optional and never on the critical correctness path.

AI is used for:

- limited hints
- evaluation explanations
- mutation previews for HR/content flows

AI is not used to decide correctness.

## Implementation

Core files:

- `internal/backend/ai.go`
- `internal/backend/app.go`
- `internal/backend/models.go`
- `internal/backend/postgres.go`
- `cmd/backend/main.go`

Fallback behavior:

- if `OPENAI_API_KEY` is unset, deterministic responses remain available

## Tests

- `internal/backend/ai_test.go`

Run:

```bash
go test ./...
```

## Tradeoffs

- deterministic fallback keeps the product usable offline or without API credentials
- AI responses can explain score and mutation logic, but backend scoring still comes from execution data
