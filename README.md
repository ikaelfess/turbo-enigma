# Transactional Outbox

A Go service that creates orders and writes an `order.created` outbox event in the same PostgreSQL transaction. That pairing is the transactional outbox pattern: the business row and the event either both commit or neither does, so a later publisher can emit the event without dual-write races.

## Architecture

Hexagonal layout with Uber Fx for wiring:

| Layer | Path | Role |
| --- | --- | --- |
| Entry points | `cmd/api`, `cmd/outbox-event-publisher`, `cmd/outbox-event-consumer` | Process mains |
| Domain | `internal/domain` | Order validation, outbox event types |
| Use case | `internal/usecase` | Create-order orchestration |
| HTTP adapter | `internal/adapters/http` | Routes, JSON, middleware |
| Postgres adapter | `internal/adapters/postgres` | Transactional writes |
| Config / logs | `internal/config`, `internal/observability` | Env config, zerolog |

## Prerequisites

- [mise](https://mise.jdx.dev/) for Go, golangci-lint, lefthook, and goose
- [Go](https://go.dev/) 1.27 (see `go.mod` / `mise.toml`)
- [Docker Compose](https://docs.docker.com/compose/) for local Postgres and the API
- [lefthook](https://github.com/evilmartians/lefthook) runs `gofmt`, golangci-lint, `go build ./...`, and `go test -v ./...` on pre-commit.

## Getting started

Install toolchain with [mise](https://mise.jdx.dev/), copy env vars if needed, then start Compose. The API waits until Postgres is healthy and goose has applied `migrations/`.

```bash
make tools
make up
```

`make up` creates `.env` from `.env.example` when it is missing, then runs the API (Postgres and migrations start as dependencies). Other targets: `make help`, `make up-d`, `make up-all`, `make down`, `make logs`, `make migrate`, `make hooks`.

The API listens on `http://localhost:3000`. Health:

```bash
curl -sS http://localhost:3000/health
```

Create an order:

```bash
curl -sS -X POST http://localhost:3000/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "items": [
      {"item_name": "Wireless Mouse", "quantity": 1, "unit_price_cents": 100},
      {"item_name": "USB-C Cable", "quantity": 2, "unit_price_cents": 400}
    ]
  }'
```

Successful response (`201`):

```json
{"id":"<uuid>","total_cents":900}
```

## HTTP API

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health` | Liveness (`OK`) |
| `POST` | `/orders` | Create order and outbox event |

`POST /orders` body:

| Field | Type | Rules |
| --- | --- | --- |
| `items` | array | At least one item |
| `items[].item_name` | string | Non-empty |
| `items[].quantity` | int | Greater than zero |
| `items[].unit_price_cents` | int64 | Non-negative |

## Database

Goose migrations live in `migrations/`. Compose runs them via `go tool goose up` in the `postgres_migrations` service.

| Table | Purpose |
| --- | --- |
| `orders` | Order header (`id` UUIDv7, `total_cents`) |
| `order_items` | Line items keyed by `order_id` |
| `outbox_events` | Event row: `aggregate_id`, `event_type`, JSON `payload`, `published_at` |

Local goose (same env as Compose):

```bash
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING='postgres://postgres:postgres@localhost:5432/transactional_outbox_development?sslmode=disable'
export GOOSE_MIGRATION_DIR=./migrations
export GOOSE_TABLE=goose_migrations
go tool goose up
```

## Tests

Integration tests start Postgres with Testcontainers, clone a templated database per case, and hit a real HTTP server.

```bash
make test
```
