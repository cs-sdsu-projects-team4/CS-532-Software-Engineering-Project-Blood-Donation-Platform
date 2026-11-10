# Backend (Go + Chi)

REST API for the Blood Donation Coordination App.

## Layout

```
cmd/api/            Executable entrypoint (func main)
internal/
  config/           Env-based configuration
  http/             Router, middleware, per-resource handlers
  store/            Database access (pgx now; sqlc-generated code later)
migrations/         Goose SQL migrations (0001_init.sql ...)
queries/            Hand-written SQL for sqlc
sqlc.yaml           sqlc config
Dockerfile          Multi-stage build -> small static image
```

`internal/` is special in Go: packages under it can only be imported by code in
this module, so it can't leak into other projects.

## Run locally

```bash
# 1. Start Postgres + the API with Docker (from ../deploy)
cd ../deploy && cp .env.example .env && docker compose up -d

# 2. Apply migrations
cd ../backend
goose -dir migrations postgres "$DATABASE_URL" up

# 3. Hit the health endpoints
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # {"status":"ok","db":"up"}
```

## Run the API outside Docker

```bash
export DATABASE_URL="postgres://blood:change-me-locally@localhost:5432/blooddb?sslmode=disable"
go run ./cmd/api
```

## Common commands

```bash
go mod tidy        # resolve deps + write go.sum (run once after cloning)
go run ./cmd/api   # run the API
go test ./...      # run all tests
go vet ./...       # static checks
sqlc generate      # regenerate store/ after editing queries/ or migrations/
```
