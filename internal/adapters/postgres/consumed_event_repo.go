package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type ConsumedEventRepo struct {
	db bun.IDB
}

func NewConsumedEventRepo(db *bun.DB) *ConsumedEventRepo {
	return &ConsumedEventRepo{db: db}
}

// Store is idempotent: a redelivered event leaves the existing row untouched.
func (c *ConsumedEventRepo) Store(ctx context.Context, event domain.OutboxEvent) error {
	model, err := NewConsumedEvent(event)
	if err != nil {
		return err
	}

	_, err = c.db.NewInsert().
		Model(model).
		On("CONFLICT (event_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("store consumed event: %w", err)
	}

	return nil
}
