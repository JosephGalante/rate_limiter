# Distributed Rate Limiting Service

This repository is being built in reviewable chunks. The current state includes a bootable Go backend scaffold plus the initial Postgres schema, migration workflow, and generated `sqlc` query layer.

## Current status

- Go API scaffolded with `chi`
- Environment-based config loading
- Static admin bearer auth middleware
- Route registry for protected endpoints and request costs
- Postgres schema and migrations for users, API keys, rate limit policies, and blocked-request audit logs
- `sqlc` query definitions and generated repository layer scaffold
- API key admin endpoints for create, list, deactivate, and raw-key hashing
- Policy admin create/list endpoints with deterministic scope validation
- Policy admin update/deactivate endpoints
- Redis projection for active policies on policy writes and startup rebuild
- Effective policy resolution from Redis projection with a precedence-based inspector endpoint
- Redis-backed token bucket engine with optimistic locking and summary counters
- Protected API endpoints enforced with API key auth, policy resolution, Redis buckets, and rate-limit headers
- Docker Compose workflow for API, Postgres, and Redis

## Run the scaffold

```bash
docker compose up --build api postgres redis
```

Apply migrations:

```bash
docker compose run --rm migrate
```

Regenerate `sqlc` code:

```bash
docker compose run --rm sqlc
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Admin ping:

```bash
curl -H 'Authorization: Bearer dev-admin-token' http://localhost:8080/api/admin/ping
```

Create an API key:

```bash
curl -X POST \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"name":"primary"}' \
  http://localhost:8080/api/admin/api-keys
```
