# Transactional Outbox

A Go service that creates orders and writes an `order.created` outbox event in the same PostgreSQL transaction. That pairing is the transactional outbox pattern: the business row and the event either both commit or neither does, so a later publisher can emit the event without dual-write races.

## Architecture

Hexagonal layout with Uber Fx for wiring:

| Layer | Path | Role |
| --- | --- | --- |
| Entry points | `cmd/api`, `cmd/outbox-publisher`, `cmd/event-worker` | Process mains |
| Domain | `internal/domain` | Order validation, outbox event types |
| Use case | `internal/usecase` | Create-order orchestration |
| HTTP adapter | `internal/adapters/http` | Routes, JSON, middleware |
| Postgres adapter | `internal/adapters/postgres` | Transactional writes |
| Config / logs | `internal/config`, `internal/observability` | Env config, zerolog |

## Prerequisites

- [mise](https://mise.jdx.dev/) for Go, golangci-lint, lefthook, and goose
- [Go](https://go.dev/) 1.27 (see `go.mod` / `mise.toml`)
- [Docker Compose](https://docs.docker.com/compose/) for local Postgres and the API

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

Validation failures return `422` with a plain-text error (`order must contain at least one item`, `item name must not be empty`, `quantity must be greater than zero`, `unit price must not be negative`). Invalid JSON returns `400`.

Run the API against an existing database without Compose:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/transactional_outbox_development?sslmode=disable'
export SERVER_ADDRESS='localhost:3000'
go run ./cmd/api
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

## Configuration

Loaded from the environment with [cleanenv](https://github.com/ilyakaznacheev/cleanenv). One `Config` struct is shared; each process reads the fields it needs.

| Variable | Description | Default |
| --- | --- | --- |
| `DATABASE_URL` | Postgres URL | required |
| `SERVER_ADDRESS` | HTTP bind address | `localhost:3000` |
| `DB_MAX_CONNS` | Pool max | `5` |
| `DB_MIN_CONNS` | Pool min | `1` |
| `DB_MAX_CONN_LIFETIME` | Pool lifetime | `1m` |
| `READ_TIMEOUT` | HTTP read timeout | `5s` |
| `WRITE_TIMEOUT` | HTTP write timeout | `10s` |
| `IDLE_TIMEOUT` | HTTP idle timeout | `30s` |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown | `20s` |
| `LOG_LEVEL` | Log level | `debug` |

Compose also uses `POSTGRES_*` for the database container and `GOOSE_*` for migrations. See `.env.example`.

## Tests

Integration tests start Postgres with Testcontainers, clone a templated database per case, and hit a real HTTP server.

```bash
make test
```

`TestNewOrder` checks HTTP status, persisted order/items, and an unpublished `order.created` outbox row.

## Development

```bash
make tools
make hooks
make lint
make build
```

[lefthook](https://github.com/evilmartians/lefthook) runs `gofmt`, golangci-lint, `go build ./...`, and `go test -v ./...` on pre-commit.
