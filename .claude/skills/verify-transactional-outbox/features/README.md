# Transactional outbox verification map

Maintained source for verifying user-facing behavior. Read this index, then the feature file.

## Baseline preconditions

- Stack launched with `scripts/launch.sh` in a fresh `outbox-verify` project (empty DB, empty topic).
- `scripts/doctor.sh` exits 0. API is `http://127.0.0.1:13000` unless `VERIFY_API_PORT` was set.
- Publisher and consumer are stopped until a recipe starts them.
- Never drive a stack this run did not start.

## Driving conventions

- Source `scripts/env.sh` for `$API`, `dc`, `psql_q`, `$VERIFY_OUT`.
- Start every recipe from baseline unless its preconditions say otherwise; for a clean slate rerun `cleanup.sh` then `launch.sh`.
- Treat commands literally; payloads are JSON with `Content-Type: application/json`.

## Proof and skip reporting

- Capture action and resulting state: HTTP response, outbox rows, Kafka record.
- Record the feature ID with every artifact under `$VERIFY_OUT/<feature>/`.
- Report an unreachable path with the attempted command and unmet precondition; do not substitute another path.

## Features

- [Create order](./create-order.md) covers `POST /orders` success, validation errors, and health.
- [Publish outbox](./publish-outbox.md) covers the publisher delivering `order.created` records to Kafka.
- [Consume outbox](./consume-outbox.md) covers the consumer storing `order.created` records in `consumed_events` and committing offsets.
