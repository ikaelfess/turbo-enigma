package river

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var _ OutboxEventUsecase = (*usecase.OutboxEventUsecase)(nil)

type OutboxEventUsecase interface {
	PublishBatch(context.Context) error
}

type OutboxEventPublisherJobArgs struct{}

func (OutboxEventPublisherJobArgs) Kind() string {
	return "OutboxEventPublisherJobArgs"
}

type OutboxEventPublisherWorker struct {
	river.WorkerDefaults[OutboxEventPublisherJobArgs]

	usecase OutboxEventUsecase
}

func NewOutboxEventPublisherWorker(usecase OutboxEventUsecase) *OutboxEventPublisherWorker {
	return &OutboxEventPublisherWorker{
		usecase: usecase,
	}
}

func (w *OutboxEventPublisherWorker) Work(
	ctx context.Context,
	_ *river.Job[OutboxEventPublisherJobArgs],
) error {
	return w.usecase.PublishBatch(ctx)
}
