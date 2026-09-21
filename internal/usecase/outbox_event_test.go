package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

func TestOutboxEventUsecase_PublishBatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	batch := make([]domain.OutboxEvent, 10)
	ids := make([]uuid.UUID, 10)
	for i := range batch {
		event := testOutboxEvent(byte(i + 1))
		batch[i] = event
		ids[i] = event.ID
	}

	publishErr := errors.New("broker unavailable")
	markErr := errors.New("update failed")

	tests := []struct {
		name          string
		batch         []domain.OutboxEvent
		failPublishID uuid.UUID
		publishErr    error
		markErr       error
		wantErr       error
		wantPublished []uuid.UUID
		wantMarked    []uuid.UUID
		wantFindLimit int
		batchSize     int
	}{
		{
			name:          "publishes batch and marks all ids",
			batch:         []domain.OutboxEvent{batch[0], batch[1]},
			wantPublished: []uuid.UUID{ids[0], ids[1]},
			wantMarked:    []uuid.UUID{ids[0], ids[1]},
			wantFindLimit: 10,
			batchSize:     10,
		},
		{
			name:          "empty batch is success",
			wantFindLimit: 10,
			batchSize:     10,
		},
		{
			name:          "stops on first publish error and marks prefix",
			batch:         batch,
			failPublishID: ids[2],
			publishErr:    publishErr,
			wantErr:       publishErr,
			wantPublished: []uuid.UUID{ids[0], ids[1]},
			wantMarked:    []uuid.UUID{ids[0], ids[1]},
			wantFindLimit: 10,
			batchSize:     10,
		},
		{
			name:          "returns mark error after successful produces",
			batch:         []domain.OutboxEvent{batch[0], batch[1]},
			markErr:       markErr,
			wantErr:       markErr,
			wantPublished: []uuid.UUID{ids[0], ids[1]},
			wantMarked:    []uuid.UUID{ids[0], ids[1]},
			wantFindLimit: 10,
			batchSize:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeOutboxEventRepo{batch: tt.batch, markErr: tt.markErr}
			publisher := &fakeEventPublisher{failID: tt.failPublishID, err: tt.publishErr}
			usecase := NewOutboxEventUsecase(repo, publisher, tt.batchSize)

			err := usecase.PublishBatch(ctx)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.wantFindLimit, repo.findLimit)
			require.Equal(t, tt.wantPublished, publisher.publishedIDs)
			if tt.wantMarked == nil {
				require.Empty(t, repo.marked)
			} else {
				require.Equal(t, [][]uuid.UUID{tt.wantMarked}, repo.marked)
			}
		})
	}
}

func testOutboxEvent(n byte) domain.OutboxEvent {
	var id uuid.UUID
	id[15] = n

	return domain.OutboxEvent{ID: id}
}

type fakeOutboxEventRepo struct {
	batch     []domain.OutboxEvent
	markErr   error
	marked    [][]uuid.UUID
	findLimit int
}

func (f *fakeOutboxEventRepo) FindUnpublishedBatch(
	_ context.Context,
	limit int,
) ([]domain.OutboxEvent, error) {
	f.findLimit = limit

	return f.batch, nil
}

func (f *fakeOutboxEventRepo) MarkPublished(_ context.Context, ids []uuid.UUID) error {
	copied := make([]uuid.UUID, len(ids))
	copy(copied, ids)
	f.marked = append(f.marked, copied)

	return f.markErr
}

type fakeEventPublisher struct {
	failID       uuid.UUID
	err          error
	publishedIDs []uuid.UUID
}

func (f *fakeEventPublisher) Publish(_ context.Context, event domain.OutboxEvent) error {
	if f.err != nil && event.ID == f.failID {
		return f.err
	}

	f.publishedIDs = append(f.publishedIDs, event.ID)

	return nil
}
