# Veritas URL Shortener

A microservice-driven URL shortener built with Go, React, PostgreSQL, Redis, NATS, Docker Compose, and Traefik.

## Architecture

Veritas separates URL creation, redirection, and analytics processing into independent services:

```text
Frontend -> Creator Service -> PostgreSQL

Browser -> Redirector Service -> Redis
                              -> PostgreSQL on cache miss
                              -> NATS -> Analytics Service
```

Traefik acts as the local gateway:

- `/api/*` requests go to the creator service.
- Short-code requests such as `/abc123` go to the redirector service.
- Other requests go to the frontend.

## Services

| Service | Technology | Responsibility |
| --- | --- | --- |
| Frontend | React + TypeScript | Provides the interface for creating short URLs |
| Creator Service | Go | Handles `POST /api/create` and persists short URLs |
| Redirector Service | Go | Resolves short codes and publishes redirect events |
| Analytics Service | Go | Consumes redirect events from NATS |
| Traefik | Reverse proxy | Routes local HTTP traffic to the correct service |

## Local Development

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/)

### Run the Application

Start the stack:

```bash
docker compose up --build
```

The application is available at `http://localhost:8080`.

Docker Compose starts PostgreSQL, Redis, and NATS locally. PostgreSQL applies
`sql/migrations/001_urls.sql` automatically when its data volume is first
created.

## Environment Variables

| Variable | Example | Description |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://user:password@host:5432/veritas?sslmode=disable` | Optional PostgreSQL connection string override |
| `REDIS_URL` | `redis://host:6379` | Optional Redis connection string override |
| `BASE_URL` | `http://localhost:8080` | Base URL used when generating short links |
| `TRUSTED_PROXY_CIDRS` | `172.28.0.2/32` | Comma-separated proxy networks whose forwarded IP headers the redirector trusts |

Docker Compose provides local defaults for PostgreSQL and Redis and configures
the internal NATS URL automatically. Set overrides in `.env` only when using
external services.

## Continuous Integration

The workflow in `.github/workflows/ci.yml` runs on pushes and pull requests targeting `main`. It:

1. Installs, lints, and builds the frontend.
2. Runs tests for every Go module in the workspace.
3. Verifies that each Docker image builds successfully.
