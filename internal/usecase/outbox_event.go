package usecase

import (
	"context"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OutboxEventRepo interface {
	FindUnpublishedBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []int64) error
}

type OutboxEventUsecase struct {
	outboxEvents OutboxEventRepo
}

func NewOutboxEventUsecase(outboxEvents OutboxEventRepo) *OutboxEventUsecase {
	return &OutboxEventUsecase{
		outboxEvents: outboxEvents,
	}
}

func (usecase *OutboxEventUsecase) PublishBatch(ctx context.Context) error {
	// TODO: implement
	return nil
}
