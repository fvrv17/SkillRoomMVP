# Kubernetes Assets

Files:

- `namespace.yaml`
- `backend-configmap.yaml`
- `backend-secret.template.yaml`
- `backend.yaml`
- `runner.yaml`
- `postgres.yaml`
- `redis.yaml`

These manifests mirror the supported MVP runtime: frontend outside this folder, plus backend, runner, PostgreSQL, and Redis.
`runner.yaml` now includes the restricted Docker API proxy sidecar so the public runner container does not mount the host Docker socket directly.
