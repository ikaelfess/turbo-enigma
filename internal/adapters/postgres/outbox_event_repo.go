package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OutboxEventRepo struct {
	db bun.IDB
}

func NewOutboxEventRepo(db bun.IDB) *OutboxEventRepo {
	return &OutboxEventRepo{db: db}
}

func (o *OutboxEventRepo) Create(
	ctx context.Context,
	event domain.OutboxEvent,
) (domain.OutboxEvent, error) {
	eventModel := NewOutboxEvent(&event)
	_, err := o.db.NewInsert().
		Model(eventModel).
		Exec(ctx)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("create outbox event: %w", err)
	}

	event.ID = eventModel.ID
	return event, nil
}
