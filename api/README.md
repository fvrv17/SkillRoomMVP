# API Contracts

This directory contains externally consumed API contracts:

- `openapi.yaml` for the current REST contract
- `events/` for WebSocket event envelopes
- `proto/` only if internal gRPC is introduced later

The OpenAPI document tracks the live monolith routes, including challenge start,
runner preview execution, scored submissions, rankings, and recruiter candidate views.

Public contract changes should be versioned and treated as backward-compatibility decisions.
