# Transactional Outbox

This service takes an order over HTTP and stores the order together with an `order.created` event in one Postgres transaction. The request only has to commit to the database: a failed commit leaves nothing to publish, and if the process that talks to Kafka dies, the event is still in the table for the next run. That event row is the order itself — id, total, and line items — and a short claim lease keeps two publishers from sending the same row at the same time.

- **One commit.** `orders`, `order_items`, and `outbox_events` are inserted in a single transaction.
- **Publish later.** [River](https://riverqueue.com/) runs the publisher every **1 minute**, and once when that process starts. Each run claims up to **100** unpublished rows.
- **One row, one publisher.** Claims use `FOR UPDATE SKIP LOCKED` and hold the row for **2 minutes**. The queue allows **5** workers.
- **At least once.** `published_at` is set only after Kafka acks the record (all in-sync replicas). A crash before that update sends the order again once the lease expires.
- **Kafka record.** Topic `order.created`, **3** partitions, replication factor **1**. The key is the order id. Headers are `event_id` and `event_type`.
- **Local stack.** Go **1.27**, Postgres **18.4**, Kafka (Confluent local **8.3.2**). API on port **3000**. Kafka UI on port **8080**.

```bash
curl -sS -X POST http://localhost:3000/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"item_name":"Wireless Mouse","quantity":1,"unit_price_cents":100},{"item_name":"USB-C Cable","quantity":2,"unit_price_cents":400}]}'
# 201
# {"id":"<uuid>","total_cents":900}
```

The response returns only after that commit. The same transaction has already inserted an `outbox_events` row with `event_type` `order.created`. With the publisher running, that row is sent on the next run, at most a minute later.

---

## Contents

- [Getting started](#getting-started)
- [See the event](#see-the-event)
- [The write](#the-write)
- [Publishing](#publishing)
- [HTTP API](#http-api)
- [Configuration](#configuration)
- [Layout](#layout)
- [Develop](#develop)

## Getting started

Install [mise](https://mise.jdx.dev/) and [Docker Compose](https://docs.docker.com/compose/), then:

```bash
make tools
make up-all
```

`make tools` installs Go 1.27.0, goose, golangci-lint 2.13.2, and lefthook from `mise.toml`.

`make up-all` copies `.env.example` to `.env` when `.env` is missing, runs goose migrations, and starts Postgres, Kafka, the `order.created` topic, Kafka UI, the API, the outbox publisher, and the outbox consumer.

| URL | What |
| --- | --- |
| http://localhost:3000 | API |
| http://localhost:8080 | Kafka UI, cluster `local` |

```bash
curl -sS http://localhost:3000/health
# OK
```

Other targets: `make up` (API, Postgres, and migrations only), `make up-d` (that set in the background), `make down`, `make logs`, `make migrate`, `make help`.

Named volumes survive `make down`. `docker compose down -v` removes them. Do that when Postgres was created from an older `outbox_events` table that has no `claimed_until` column.

## See the event

Create an order:

```bash
curl -sS -X POST http://localhost:3000/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"item_name":"Wireless Mouse","quantity":1,"unit_price_cents":100},{"item_name":"USB-C Cable","quantity":2,"unit_price_cents":400}]}'
```

The outbox row is there immediately. `published_at` stays null until the publisher runs:

```bash
docker compose exec postgres psql -U postgres -d transactional_outbox_development \
  -c "SELECT event_type, published_at FROM outbox_events ORDER BY created_at DESC LIMIT 5;"
```

Wait for the next publisher run (at most one minute), then open topic `order.created` in the [Kafka UI](http://localhost:8080). The record value is the order JSON. Confirm the topic itself with:

```bash
docker compose exec kafka kafka-topics \
  --bootstrap-server localhost:9092 --describe --topic order.created
```

Kafka listens only on the Compose network. The UI's HTTP port is the one published on the host.

## The write

`POST /orders` checks the line items, sums `quantity * unit_price_cents`, and then [`OrderStore.Create`](internal/adapters/postgres/order_store.go) does three inserts in one transaction:

1. `orders` — UUIDv7 `id`, `total_cents`
2. `order_items` — one row per line
3. `outbox_events` — `event_type` `order.created`, JSON payload `{id, total_cents, items}`

The handler writes `201` after that commit. Goose applies `migrations/` in the `postgres_migrations` container before the API starts.

## Publishing

[`PublishBatch`](internal/usecase/outbox_event.go) runs inside a River job ([`periodic_jobs.go`](internal/adapters/river/periodic_jobs.go)):

1. Lock unpublished rows whose `claimed_until` is null or already past, oldest first, with `FOR UPDATE SKIP LOCKED`.
2. Set `claimed_until` to now plus the claim TTL (default **2m**).
3. Produce each record synchronously to Kafka.
4. Set `published_at` on the records Kafka accepted.
5. On the first produce error, stop the batch, mark the earlier successes, and return the error. The failed row stays unpublished until the lease expires.

The Kafka record:

| Field | Value |
| --- | --- |
| Topic | `KAFKA_TOPIC`, default `order.created` |
| Key | Order id (`aggregate_id`) |
| Headers | `event_id`, `event_type` (`order.created`) |
| Value | `{"id","total_cents","items":[{"id","item_name","quantity","unit_price_cents"}]}` |

Compose creates that topic with 3 partitions and replication factor 1, and turns broker auto-creation off. The broker log stays in the `kafka_data` volume, so consumer offsets survive a restart.

## Consuming

`cmd/outbox-event-consumer` joins group `KAFKA_CONSUMER_GROUP` on `KAFKA_TOPIC` and decodes each record back into an outbox event (`event_id` and `event_type` headers, order id key, JSON value). [`Handle`](internal/usecase/consumed_event.go) then runs `INSERT ... ON CONFLICT (event_id) DO NOTHING` into `consumed_events`, so a redelivered record never adds a second row.

- **Commit after the write.** Auto-commit is off. An offset is committed only once its write succeeded.
- **Retry in place.** A failed write is retried with backoff (100ms doubling to 5s). The partition does not advance past it.
- **Skip the undecodable.** A record with a missing header, a bad key, or bad JSON can never succeed. It is logged at error level and skipped.
- **Graceful stop.** On SIGTERM the consumer stops polling, finishes the in-flight record, commits, and leaves the group.

Check a consumed event:

```bash
docker compose exec postgres psql -U postgres -d transactional_outbox_development \
  -c "SELECT event_id, aggregate_id, event_type, consumed_at FROM consumed_events ORDER BY consumed_at DESC LIMIT 5;"
```

## HTTP API

| Method | Path | Result |
| --- | --- | --- |
| `GET` | `/health` | `200` and the body `OK` |
| `POST` | `/orders` | `201` and `{"id","total_cents"}` |

`POST /orders` body:

| Field | Rule |
| --- | --- |
| `items` | At least one item |
| `items[].item_name` | Non-empty string |
| `items[].quantity` | Integer greater than zero |
| `items[].unit_price_cents` | Integer, zero or greater |

Invalid JSON returns `400` and `invalid request body`. Any error from creating the order returns `422` and the error text (`order must contain at least one item`, `item name must not be empty`, `quantity must be greater than zero`, `unit price must not be negative`).

## Configuration

One [`Config`](internal/config/config.go) struct serves every process. Each process reads the fields it uses. Compose passes these through from `.env`.

| Variable | Default | Role |
| --- | --- | --- |
| `DATABASE_URL` | required | Postgres URL |
| `SERVER_ADDRESS` | `localhost:3000` | API listen address. `.env.example` sets `0.0.0.0:3000` so the published port works |
| `OUTBOX_BATCH_SIZE` | `100` | Rows claimed per job |
| `OUTBOX_CLAIM_TTL` | `2m` | How long a claim stays exclusive |
| `WORKERS_NUM` | `5` | River workers on the default queue |
| `KAFKA_BROKERS` | `kafka:9092` | Bootstrap servers |
| `KAFKA_TOPIC` | `order.created` | Topic the publisher writes and the consumer reads, and the topic Compose creates |
| `KAFKA_CONSUMER_GROUP` | `outbox-event-consumer` | Consumer group the consumer joins |

`DB_MAX_CONNS` (20), `DB_MIN_CONNS` (5), and `DB_MAX_CONN_LIFETIME` (30m) size the Postgres pools. `SHUTDOWN_TIMEOUT` (20s), `READ_TIMEOUT` (5s), `WRITE_TIMEOUT` (10s), and `IDLE_TIMEOUT` (30s) belong to the API server.

> Set a Kafka or outbox variable in `.env` only to override it. An empty value counts as set, so `KAFKA_BROKERS=` replaces the default with an empty broker list.

**OpenTelemetry.** Set `OTEL_EXPORTER_OTLP_ENDPOINT` to export traces and metrics over OTLP/HTTP. Leave it unset and both providers are no-ops. HTTP (except `/health`), Bun SQL, the Kafka producer, and River jobs are instrumented. Optional companions in `.env.example`: `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_COMPRESSION`, `OTEL_EXPORTER_OTLP_TIMEOUT`, `OTEL_RESOURCE_ATTRIBUTES`.

## Layout

| Path | Role |
| --- | --- |
| `cmd/api` | HTTP server |
| `cmd/outbox-event-publisher` | River publisher |
| `cmd/outbox-event-consumer` | Kafka consumer |
| `internal/domain` | Order checks and the outbox event |
| `internal/usecase` | Create-order, publish-batch, and handle-consumed-event |
| `internal/adapters/http` | Routes, JSON, middleware |
| `internal/adapters/postgres` | Transactional writes, the claim query, and the idempotent consumed-event insert |
| `internal/adapters/kafka` | Sync produce and the group consumer |
| `internal/adapters/river` | Periodic job |
| `internal/config`, `internal/observability` | Env config, zerolog, OTLP |
| `migrations/` | Goose migrations |

Uber Fx wires each process.

## Develop

Integration tests start Postgres 18.4 and Kafka with Testcontainers, so Docker has to be running.

```bash
make test
make lint
make build
make hooks
```

`make hooks` installs a lefthook pre-commit hook that runs `golangci-lint fmt` on staged Go files.

Pull requests and pushes to `main` run `golangci-lint fmt --diff`, `golangci-lint run`, `go build ./...`, and `go test ./...`.
