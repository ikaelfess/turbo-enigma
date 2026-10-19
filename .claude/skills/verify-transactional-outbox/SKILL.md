---
name: verify-transactional-outbox
description: Launch and drive the transactional-outbox Go service (HTTP API + River publisher + Kafka consumer + Postgres + Kafka) in an isolated Docker Compose project, and prove behavior with curl, psql and a Kafka consumer. Use when a change touches POST /orders, the outbox_events table, the publisher, the consumer, or Kafka output and tests alone are not enough.
---

# Verify transactional-outbox

Surface: an HTTP API (`POST /orders`, `GET /health`) whose side effect is an `outbox_events` row, later published to Kafka topic `order.created` by a separate publisher process and stored in `consumed_events` by a separate consumer process. There is no UI. Read `features/README.md` before driving, then the matching feature file.

All scripts live in `.claude/skills/verify-transactional-outbox/scripts/` and `source env.sh`, which defines `$API`, `dc` (docker compose with the verify overlay), `psql_q "<sql>"` and `$VERIFY_OUT`. In a driving shell run `source .claude/skills/verify-transactional-outbox/scripts/env.sh` first.

## Isolation

The stack runs as compose project `outbox-verify` with its own volumes, so every launch starts from an empty database and an empty Kafka log. `compose.verify.yml` moves the API to host port `13000` (override with `VERIFY_API_PORT`), hides Postgres from the host and moves Kafka UI to `18080`. The repo's default ports (3000, 5432, 8080) are often taken by other local apps; never stop what holds them. If `13000` is held by a process that is not this project, `launch.sh` refuses to start: choose another `VERIFY_API_PORT`. Do not drive a stack you did not start in this run.

## Launch

The containers build from the checked-out tree. Check out the branch under test first: on a branch without the `consumed_events` migration, doctor reports `FAIL migrations applied (consumed_events exists)`.

```bash
.claude/skills/verify-transactional-outbox/scripts/launch.sh
```

Runs `make env` (creates `.env` if missing), then `up --build -d api kafka kafka_topics` (Postgres and goose migrations come in as dependencies). Ready means: `/health` returns `OK`, Kafka is healthy and the `kafka_topics` one-shot exited 0. First run compiles Go inside the container and pulls images; allow a few minutes. The publisher and consumer are deliberately not started; see below.

## Doctor

```bash
.claude/skills/verify-transactional-outbox/scripts/doctor.sh
```

Read-only. Checks the api container belongs to `outbox-verify` and owns the host port, health, the latest migration, and the 3-partition topic; prints publisher and consumer state. Exit 1 on any FAIL. Run it first whenever anything looks off.

## Drive

- **Create an order:** `curl -sS -i -X POST $API/orders -H 'Content-Type: application/json' -d '{"items":[...]}'`
- **Inspect the outbox:** `psql_q "select event_type, published_at is null, payload from outbox_events order by created_at desc"`
- **Start the publisher:** `scripts/start-publisher.sh`. River runs the job once at process start and then every minute, so create rows first and start the publisher after: they publish within seconds instead of up to 60s. Stop it with `dc stop outbox_event_publisher`.
- **Start the consumer:** `scripts/start-consumer.sh`. It reads the topic from the start, so start it after the publisher has sent rows. Stop it with `dc stop outbox_event_consumer`.
- **Inspect consumed events:** `psql_q "select event_id, aggregate_id, event_type from consumed_events"`.
- **Read Kafka:** `dc exec -T kafka kafka-console-consumer --bootstrap-server kafka:9092 --topic order.created --from-beginning --max-messages N --timeout-ms 15000 --property print.key=true --property print.headers=true` (stderr carries Java warnings; ignore them).

Feature recipes are in `features/`.

## Evidence

Written to `$VERIFY_OUT` (default `.verify/artifacts/`, git-ignored), one folder per feature. It survives cleanup. Standards:

- Use the real HTTP path, never insert into `outbox_events` by hand to simulate an order.
- Capture the request and response (`curl -i`), the database state before and after, and the Kafka record with key and headers. A `201` alone is not proof; the outbox row and the Kafka record are.
- Kafka is real; nothing is mocked. The only unrealistic part is that the publisher and consumer are started on demand.

## Cleanup

```bash
.claude/skills/verify-transactional-outbox/scripts/cleanup.sh
```

Runs `docker compose -p outbox-verify down -v --remove-orphans`: removes only this project's containers and volumes. It never touches `$VERIFY_OUT`. Run it after failed attempts too.
