# DevOps Agent

## Design

The local and deployment stack is:

- Next.js frontend
- Go backend
- runner service
- PostgreSQL
- Redis

The backend and runner are containerized separately because they have different trust and resource profiles.

## Implementation

Core files:

- `deploy/backend.Dockerfile`
- `deploy/frontend.Dockerfile`
- `deploy/runner.Dockerfile`
- `deploy/docker-compose.yml`
- `deploy/backend.env.example`
- `deploy/k8s/backend.yaml`
- `deploy/k8s/runner.yaml`
- `.github/workflows/ci.yml`

Operational features:

- `/livez`, `/readyz`, `/metrics`
- request logging and basic metrics
- graceful shutdown
- Docker Compose for local verification
- Kubernetes-ready manifests for cloud deployment

## Verification

```bash
go test ./...
docker compose -f deploy/docker-compose.yml config
```

## Tradeoffs

- the manifests are intentionally simple and suitable for an MVP
- deeper production hardening such as secrets management and autoscaling policy should be added in the target environment
