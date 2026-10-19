package usecase

import (
	"context"
	"fmt"

	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

var _ ConsumedEventRepo = (*postgres.ConsumedEventRepo)(nil)

type ConsumedEventRepo interface {
	Store(ctx context.Context, event domain.OutboxEvent) error
}

type ConsumedEventUsecase struct {
	consumedEvents ConsumedEventRepo
}

func NewConsumedEventUsecase(consumedEvents ConsumedEventRepo) *ConsumedEventUsecase {
	return &ConsumedEventUsecase{consumedEvents: consumedEvents}
}

// Handle is safe to call again for the same event, so at-least-once redelivery
// leaves one stored copy.
func (usecase *ConsumedEventUsecase) Handle(ctx context.Context, event domain.OutboxEvent) error {
	if err := usecase.consumedEvents.Store(ctx, event); err != nil {
		return fmt.Errorf("handle consumed event %s: %w", event.ID, err)
	}

	return nil
}
