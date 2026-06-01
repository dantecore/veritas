# Veritas

Veritas is a microservice-based URL shortener built with Go, React, PostgreSQL,
Redis, NATS, Docker Compose, and Traefik.

Paste a long URL into the web interface, receive a compact link, and let the
redirect path stay fast through Redis caching. Successful redirects are
published as events so analytics work remains separate from the request path.

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/)

### Start the stack

```bash
docker compose up --build
```

Open [http://localhost:8080](http://localhost:8080) to use the web interface.

You can also create a short URL directly from the API:

```bash
curl --request POST http://localhost:8080/api/create \
  --header "Content-Type: application/json" \
  --data '{"original_url":"https://example.com/articles/veritas"}'
```

Example response:

```json
{
  "short_url": "http://localhost:8080/b"
}
```

Open the returned URL to verify the redirect. The short code depends on the
database record ID, so your response may differ from the example.

### Stop the stack

```bash
docker compose down
```

PostgreSQL data is kept in the `postgres-data` Docker volume. To remove local
data and recreate the database from scratch:

```bash
docker compose down --volumes
```

## How It Works

```mermaid
flowchart LR
    Browser["Browser"] --> Traefik["Traefik gateway"]
    Traefik -->|"/api/*"| FrontendAPI["Creator service"]
    Traefik -->|"/{short_code}"| Redirector["Redirector service"]
    Traefik -->|"other routes"| Frontend["React frontend"]
    FrontendAPI --> PostgreSQL[("PostgreSQL")]
    Redirector --> Redis[("Redis cache")]
    Redirector -->|"cache miss"| PostgreSQL
    Redirector -->|"veritas.redirect.success"| NATS["NATS"]
    NATS --> Analytics["Analytics service"]
```

1. The creator service stores the original URL in PostgreSQL.
2. The database ID is encoded as a Base62 short code.
3. The redirector checks Redis first and falls back to PostgreSQL on a cache
   miss.
4. Resolved URLs are cached for one hour.
5. Every successful redirect publishes a Protobuf event to
   `veritas.redirect.success`.
6. The analytics service currently consumes and logs redirect events.

## Services

| Service | Responsibility |
| --- | --- |
| `frontend-service` | Serves the React interface through Nginx |
| `creator-service` | Creates and persists short URLs |
| `redirector-service` | Resolves short codes, caches URLs, and publishes redirect events |
| `analytics-service` | Consumes and logs successful redirect events |
| `postgres` | Stores URL records |
| `redis` | Caches resolved URLs |
| `nats` | Delivers redirect events |
| `traefik` | Routes local HTTP traffic |

## Local Endpoints

| URL | Description |
| --- | --- |
| [http://localhost:8080](http://localhost:8080) | Web interface |
| [http://localhost:8080/api/healthcheck](http://localhost:8080/api/healthcheck) | Creator service healthcheck |
| [http://localhost:8080/healthcheck](http://localhost:8080/healthcheck) | Redirector service healthcheck |
| [http://localhost:8081](http://localhost:8081) | Traefik dashboard |
| [http://localhost:8222](http://localhost:8222) | NATS monitoring endpoint |

## API Reference

### Create a short URL

`POST /api/create`

Request body:

```json
{
  "original_url": "https://example.com/articles/veritas"
}
```

Successful response: `201 Created`

```json
{
  "short_url": "http://localhost:8080/b"
}
```

Invalid JSON or an invalid URL returns `400 Bad Request`.

### Follow a short URL

`GET /{short_code}`

A known short code returns `302 Found` and redirects to the original URL. An
unknown short code returns `404 Not Found`.

## Configuration

Docker Compose provides working defaults for local development. Create a `.env`
file in the repository root only when you need to override them.

| Variable | Default | Used by | Description |
| --- | --- | --- | --- |
| `DATABASE_URL` | `postgres://veritas:veritas@postgres:5432/veritas?sslmode=disable` | Creator, redirector | PostgreSQL connection string |
| `REDIS_URL` | `redis://redis:6379` | Redirector | Redis connection string |
| `BASE_URL` | `http://localhost:8080` | Creator | Prefix returned in generated short URLs |
| `CREATOR_PORT` | `8081` | Creator | Internal HTTP port |
| `REDIRECTOR_PORT` | `8082` | Redirector | Internal HTTP port |
| `TRUSTED_PROXY_CIDRS` | `172.28.0.2/32` | Redirector | Comma-separated proxies allowed to supply forwarded client IP headers |
| `VITE_API_URL` | `/api` in production | Frontend | API base URL embedded into the frontend build |

`NATS_URL` is configured inside `docker-compose.yml` as `nats://nats:4222` for
the services that use it.

## Repository Layout

```text
.
├── frontend/                  # React + TypeScript interface
├── pkg/                       # Shared Go packages and generated code
├── proto/events/v1/           # Redirect event Protobuf contract
├── services/
│   ├── analytics-service/     # NATS redirect-event consumer
│   ├── creator-service/       # Short URL creation API
│   └── redirector-service/    # Redirect API, Redis cache, NATS publisher
├── sql/
│   ├── migrations/            # PostgreSQL schema
│   ├── queries/               # sqlc queries
│   └── sqlc.yaml              # sqlc generation config
├── docker-compose.yml         # Local stack
└── go.work                    # Go workspace
```

## Development

### Run backend tests

The repository contains multiple Go modules. Run tests for each workspace
module:

```bash
for module in $(go list -m -f '{{.Dir}}'); do
  (cd "$module" && go test ./...)
done
```

### Check the frontend

```bash
npm ci --prefix frontend
npm run lint --prefix frontend
npm run build --prefix frontend
```

### Inspect service logs

```bash
docker compose logs --follow creator-service redirector-service analytics-service
```

## Database Initialization

On the first start, PostgreSQL applies `sql/migrations/001_urls.sql` through
`sql/init/001_urls.sh`. Existing `postgres-data` volumes are left untouched on
later starts.

After changing the initial migration during local development, recreate the
volume to apply it to a fresh database:

```bash
docker compose down --volumes
docker compose up --build
```

## Continuous Integration

The GitHub Actions workflow in `.github/workflows/ci.yml` runs on pushes and
pull requests targeting `main`. It:

1. Installs, lints, and builds the frontend.
2. Runs tests for every Go module in the workspace.
3. Builds each Docker image.
