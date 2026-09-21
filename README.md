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
- [Docker Compose](https://docs.docker.com/compose/) for local Postgres, Kafka, and the API
- [lefthook](https://github.com/evilmartians/lefthook) applies `golangci-lint fmt` (goimports) on staged `*.go` at pre-commit
- GitHub Actions job `ci` on pull requests and pushes to `main` is the merge-quality gate: `golangci-lint fmt --diff`, `golangci-lint run`, `go build ./...`, and `go test ./...`

## Getting started

Install toolchain with [mise](https://mise.jdx.dev/), copy env vars if needed, then start Compose. The API waits until Postgres is healthy and goose has applied `migrations/`.

```bash
make tools
make up
```

`make up` creates `.env` from `.env.example` when it is missing, then runs the API (Postgres and migrations start as dependencies). It starts no Kafka containers, because the API does not talk to a broker; use `make up-all` for those. Other targets: `make help`, `make up-d`, `make up-all`, `make down`, `make logs`, `make migrate`, `make hooks`.

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

## Messaging

`make up-all` starts a single-node Kafka broker in KRaft mode plus a UI. The publisher and consumer do not produce or consume yet; this is the infrastructure and configuration they will read.

| Service | Image | Role |
| --- | --- | --- |
| `kafka` | `apache/kafka:4.3.1` | Broker and controller in one node, reachable only inside the Compose network at `kafka:9092` |
| `kafka_topics` | `apache/kafka:4.3.1` | One-shot `kafka-topics.sh --create --if-not-exists`, 3 partitions, replication factor 1 |
| `kafka_ui` | `kafbat/kafka-ui:v1.5.0` | Browser UI on [localhost:8080](http://localhost:8080), single cluster `local` defined in `compose.yml` |

No Kafka port is published to the host, so a binary run outside Compose cannot reach the broker; only the UI's HTTP port is exposed. Topic auto-creation is disabled, so an unknown topic name fails instead of silently creating a single-partition topic. The broker's log directory is kept in the `kafka_data` volume, so consumer group offsets survive a restart.

| Env var | Default | Purpose |
| --- | --- | --- |
| `KAFKA_BROKERS` | `kafka:9092` | Comma-separated bootstrap servers |
| `KAFKA_TOPIC` | `order.created` | Topic for outbox events; also drives `kafka_topics` |
| `KAFKA_CONSUMER_GROUP` | `outbox-event-consumer` | Consumer group id |

All three have defaults rather than being required, so the API, which shares one `Config` struct, still boots without a broker. `.env.example` lists them commented out: uncomment one only to override it, because `cleanenv` treats an empty value as a value and not as a missing one.

Inspect the topic without the UI:

```bash
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 --describe --topic order.created
```

## Tests

Integration tests start Postgres with Testcontainers, clone a templated database per case, and hit a real HTTP server.

```bash
make test
```
