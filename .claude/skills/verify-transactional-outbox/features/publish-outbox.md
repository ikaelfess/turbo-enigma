# Publish outbox

The publisher claims unpublished outbox rows, produces them to Kafka topic `order.created` (key = order id, headers `event_id`, `event_type`) and marks them published only after Kafka acks.

## Sub-features

- `publish-startup` rows created before the publisher starts are sent by its startup run.
- `publish-record` Kafka record has the right key, headers and JSON value.
- `publish-marked` `published_at` is set and `claimed_until` lease is recorded.

## How to get to it (user POV)

- A client creates orders via `POST /orders`; an operator runs the `outbox_event_publisher` process.
- A consumer reads topic `order.created`.

## Driving it with curl, psql and kafka-console-consumer

Preconditions: baseline; publisher stopped; at least one order created per [create-order](./create-order.md), `published_at` still null.

- **Start.** `scripts/start-publisher.sh`.
- **Wait.** Poll `psql_q "select count(*) from outbox_events where published_at is null"` until `0` (seconds; give up after 60s, the next scheduled run).
- **Marked.** `psql_q "select event_type, published_at is not null from outbox_events"` shows `order.created|t`.
- **Record.** `dc exec -T kafka kafka-console-consumer --bootstrap-server kafka:9092 --topic order.created --from-beginning --max-messages 1 --timeout-ms 15000 --property print.key=true --property print.headers=true`. Expect headers `event_id:<uuid>,event_type:order.created`, key equal to the order id, value `{"id","total_cents","items":[...]}` matching the response.
- **Record count.** Message count equals the number of orders created; no duplicates in a single clean run.
- **Proof.** Save the outputs to `$VERIFY_OUT/publish-outbox/`.

## Gotchas

- Publishing is at least once; duplicates after a crash or lease expiry are legitimate, but not in a clean run.
- The record also carries a `traceparent` header; ignore it.
- Kafka is only reachable inside the Compose network; use `dc exec kafka`, not a host client.
- The scheduled job runs every minute; the startup run is the fast path, so start the publisher after creating rows.
- Stale volumes from an old schema without `claimed_until` break migrations; the verify project uses its own volumes and `cleanup.sh` removes them.
