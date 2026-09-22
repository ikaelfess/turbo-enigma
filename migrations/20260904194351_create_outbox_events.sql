-- +goose Up
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    claimed_until TIMESTAMPTZ
);

CREATE INDEX idx_outbox_events_unpublished
    ON outbox_events (created_at)
    WHERE published_at IS NULL;

-- +goose Down
DROP TABLE outbox_events;
