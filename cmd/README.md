# Command Entrypoints

Supported binaries:

- `backend`: main Go API
- `runner-service`: isolated execution service
- `runner-docker-proxy`: restricted Docker API sidecar for the runner boundary
- `devstack`: local Go bootstrap for backend + runner

The older split-service entrypoints were removed to keep the repo aligned with the current modular monolith architecture.
