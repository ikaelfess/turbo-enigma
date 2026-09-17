package usecase

import (
	"context"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OutboxEventRepository interface {
	FindUnpublishedBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []int64) error
}

type OutboxEventUsecase struct {
	outboxEvents OutboxEventRepository
}

func NewOutboxEventUsecase(outboxEvents OutboxEventRepository) *OutboxEventUsecase {
	return &OutboxEventUsecase{
		outboxEvents: outboxEvents,
	}
}

func (usecase *OutboxEventUsecase) PublishBatch(ctx context.Context) error {
	// TODO: implement
	return nil
}
