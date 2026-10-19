package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

func TestConsumedEventUsecase_Handle(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("database unavailable")
	first := testOutboxEvent(1)
	second := testOutboxEvent(2)

	tests := []struct {
		name      string
		events    []domain.OutboxEvent
		storeErr  error
		wantErr   error
		wantStore []uuid.UUID
	}{
		{
			name:      "stores the event",
			events:    []domain.OutboxEvent{first},
			wantStore: []uuid.UUID{first.ID},
		},
		{
			name:      "duplicate event is stored once",
			events:    []domain.OutboxEvent{first, first, second},
			wantStore: []uuid.UUID{first.ID, second.ID},
		},
		{
			name:     "store error is returned",
			events:   []domain.OutboxEvent{first},
			storeErr: storeErr,
			wantErr:  storeErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeConsumedEventRepo{err: tt.storeErr}
			usecase := NewConsumedEventUsecase(repo)

			var err error
			for _, event := range tt.events {
				if err = usecase.Handle(t.Context(), event); err != nil {
					break
				}
			}

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantStore, repo.stored)
		})
	}
}

type fakeConsumedEventRepo struct {
	err    error
	stored []uuid.UUID
}

func (f *fakeConsumedEventRepo) Store(_ context.Context, event domain.OutboxEvent) error {
	if f.err != nil {
		return f.err
	}

	for _, id := range f.stored {
		if id == event.ID {
			return nil
		}
	}

	f.stored = append(f.stored, event.ID)
	return nil
}
