-- +goose Up
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE orders;
