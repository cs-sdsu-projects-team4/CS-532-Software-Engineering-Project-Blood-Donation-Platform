# Backend — Blood Donation Coordination API

Go + Chi REST API over PostgreSQL + PostGIS. Type-safe DB access is generated with
sqlc from explicit SQL; schema changes are Goose migrations.

## Layout

```
cmd/api/            HTTP server entrypoint (main)
internal/
  config/           env configuration loading
  http/             router, middleware, one handler file per resource
  auth/             JWT, Argon2id, RBAC middleware
  store/            sqlc-generated DB access (never hand-edited)
  domain/           entities + blood-compatibility & screening rules
  matching/         deterministic matching pipeline
  notify/           push-notification abstraction (FCM-ready)
  bootstrap/        first-run system_admin seeding
migrations/         Goose .sql migrations
queries/            .sql source for sqlc
```

## Prerequisites

- Go 1.23+
- Docker (for the local Postgres+PostGIS via `../deploy/docker-compose.yml`)
- `sqlc` and `goose` on your PATH (or call them from `~/go/bin`). On machines where
  cgo is broken, install sqlc with `CGO_ENABLED=0 go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`.

## Configuration

Config comes from the environment (12-factor). Copy `../deploy/.env.example` to
`../deploy/.env` and fill it in. Required: `DATABASE_URL`, `JWT_SECRET`. Optional
`BOOTSTRAP_ADMIN_EMAIL`/`BOOTSTRAP_ADMIN_PASSWORD` create the first `system_admin`
on startup (clinics and staff are provisioned by a system_admin, never self-registered).

## Run locally

```bash
# 1. start the database
cd ../deploy && docker compose up -d db

# 2. apply migrations
cd ../backend
goose -dir migrations postgres "$DATABASE_URL" up

# 3. run the API (reads config from the environment)
go run ./cmd/api
```

## Common tasks

```bash
go build ./...                 # compile
go test ./...                  # unit tests (no database needed)
go vet ./... && gofmt -l .     # vet + format check
golangci-lint run              # lint (config in .golangci.yml)

sqlc generate                  # regenerate internal/store after editing queries/ or migrations/
sqlc diff                      # fail if generated code is stale (used in CI)

# integration tests (need a live DB):
DATABASE_URL="postgres://blood:<pw>@localhost:5432/blooddb?sslmode=disable" \
  go test -tags integration ./...
```

After changing `queries/` or `migrations/`, always `sqlc generate` and commit the
regenerated `internal/store/` together with the change.

## API contract

The REST contract lives in [`../docs/openapi.yaml`](../docs/openapi.yaml).
