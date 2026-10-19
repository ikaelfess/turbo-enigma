# Create order

A client posts line items to `POST /orders`; the service stores the order, its items and an `order.created` outbox row in one transaction and answers with the order id and total.

## Sub-features

- `order-create` valid request returns `201` with `id` and `total_cents`.
- `order-outbox-row` the same request leaves one unpublished `outbox_events` row.
- `order-invalid` bad input returns `400` or `422` and writes nothing.
- `health` `GET /health` returns `OK`.

## How to get to it (user POV)

- `POST $API/orders` with a JSON body `{"items":[{"item_name","quantity","unit_price_cents"}]}`.
- `GET $API/health`.

## Driving it with curl and psql

Preconditions: baseline; `select count(*) from orders` is `0`.

- **Create.** `curl -sS -i -X POST $API/orders -H 'Content-Type: application/json' -d '{"items":[{"item_name":"Wireless Mouse","quantity":1,"unit_price_cents":100},{"item_name":"USB-C Cable","quantity":2,"unit_price_cents":400}]}'`. Expect `201` and `{"id":"<uuid>","total_cents":900}`.
- **Outbox row.** `psql_q "select event_type, published_at is null from outbox_events"`. Expect one row `order.created|t` whose payload `id` equals the response `id`.
- **Rows agree.** `psql_q "select count(*) from order_items"` is `2`.
- **Invalid JSON.** Post `not json`; expect `400` and `invalid request body`.
- **Domain errors.** Post `{"items":[]}`, a blank `item_name`, `"quantity":0`, and `"unit_price_cents":-1`; expect `422` with `order must contain at least one item`, `item name must not be empty`, `quantity must be greater than zero`, `unit price must not be negative`. Row counts stay unchanged.
- **Health.** `curl -sS $API/health` prints `OK`.
- **Proof.** Save responses and psql output to `$VERIFY_OUT/create-order/`.

## Gotchas

- The `Content-Type` header is required for the documented behavior; keep it.
- `GET /health` is not traced; do not use it to prove telemetry.
- A `201` without the outbox row means the transaction is broken even though the client sees success.
