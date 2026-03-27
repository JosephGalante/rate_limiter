# Distributed Rate Limiting Service

This repository is being built in reviewable chunks. The current chunk scaffolds the Go backend so it can boot in Docker and exposes the initial health, admin, and protected route surfaces.

## Current status

- Go API scaffolded with `chi`
- Environment-based config loading
- Static admin bearer auth middleware
- Route registry for protected endpoints and request costs
- Docker Compose workflow for API, Postgres, and Redis

## Run the scaffold

```bash
docker compose up --build api postgres redis
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Admin ping:

```bash
curl -H 'Authorization: Bearer dev-admin-token' http://localhost:8080/api/admin/ping
```
