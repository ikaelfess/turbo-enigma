package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ikaelfess/transactional-outbox/internal/adapters/kafka"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

var _ OutboxEventRepo = (*postgres.OutboxEventRepo)(nil)
var _ EventPublisher = (*kafka.Producer)(nil)

type OutboxEventRepo interface {
	FindUnpublishedBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []uuid.UUID) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type OutboxEventUsecase struct {
	outboxEvents OutboxEventRepo
	publisher    EventPublisher
	batchSize    int
}

func NewOutboxEventUsecase(
	outboxEvents OutboxEventRepo,
	publisher EventPublisher,
	batchSize int,
) *OutboxEventUsecase {
	return &OutboxEventUsecase{
		outboxEvents: outboxEvents,
		publisher:    publisher,
		batchSize:    batchSize,
	}
}

func (usecase *OutboxEventUsecase) PublishBatch(ctx context.Context) error {
	events, err := usecase.outboxEvents.FindUnpublishedBatch(ctx, usecase.batchSize)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	var publishErr error
	published := make([]uuid.UUID, 0, len(events))
	for _, event := range events {
		if err := usecase.publisher.Publish(ctx, event); err != nil {
			publishErr = fmt.Errorf("publish outbox event: %w", err)
			break
		}

		published = append(published, event.ID)
	}

	if len(published) == 0 {
		return publishErr
	}

	if err := usecase.outboxEvents.MarkPublished(ctx, published); err != nil {
		if publishErr != nil {
			return errors.Join(publishErr, err)
		}

		return err
	}

	return publishErr
}
