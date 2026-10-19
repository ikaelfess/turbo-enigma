# Consume outbox

The consumer joins group `outbox-event-consumer` on topic `order.created`, decodes each record and stores it once in `consumed_events` (`event_id` primary key, `ON CONFLICT DO NOTHING`). It commits the offset only after the write succeeds.

## Sub-features

- `consume-stored` every published event appears in `consumed_events` with the same `event_id` and `aggregate_id`.
- `consume-committed` after the consumer stops, the group shows zero lag.

## How to get to it (user POV)

- A client creates orders via `POST /orders` and the publisher sends them ([publish-outbox](./publish-outbox.md)); an operator runs the `outbox_event_consumer` process.

## Driving it with curl, psql and kafka-consumer-groups

Preconditions: baseline; orders created and published per [publish-outbox](./publish-outbox.md); consumer stopped; `select count(*) from consumed_events` is `0`.

- **Start.** `scripts/start-consumer.sh`. Allow about 40s the first time, because `go run` compiles inside the container.
- **Wait.** Poll `psql_q "select count(*) from consumed_events"` until it equals the number of orders (give up after 90s).
- **Stored.** `psql_q "select count(*) from outbox_events o join consumed_events c on c.event_id = o.id and c.aggregate_id = o.aggregate_id"` equals the order count.
- **Lag.** `dc stop outbox_event_consumer`, then `dc exec -T kafka kafka-consumer-groups --bootstrap-server kafka:9092 --describe --group outbox-event-consumer`. Expect `LAG` `0` on every partition that holds records.
- **Proof.** Save the outputs to `$VERIFY_OUT/consume-outbox/`.

## Gotchas

- The consumer reads from the start of the topic, so it does not need to start before the publisher.
- `dc stop` shows exit code 143, because `go run` wraps the process. It says nothing about graceful shutdown.
- Not driven here: a failing write, a duplicate record and an undecodable record. The integration test under `test/integration/consumer` covers those.
