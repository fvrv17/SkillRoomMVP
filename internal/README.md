# Internal Packages

Active packages:

- `backend`: main product logic and HTTP handlers
- `evaluation`: shared score and confidence logic
- `platform`: config, ids, auth, and HTTP helpers
- `runner`: runner contract, Docker sandbox execution, HTTP transport

`internal/evaluation` is the source of truth for shared score calculations. The older duplicated service-layer scoring code has been removed.
