package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

func TestNewOutboxEvent(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	aggregateID := uuid.New()
	itemID := uuid.New()

	createdAt := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	payload := domain.OutboxEventPayload{
		ID:         id,
		TotalCents: 12500,
		Items: []domain.OutboxEventItem{
			{
				ID:             itemID,
				ItemName:       "Coffee",
				Quantity:       2,
				UnitPriceCents: 6250,
			},
		},
	}

	tests := []struct {
		name string
		src  *domain.OutboxEvent
		want *OutboxEvent
	}{
		{
			name: "copies all fields",
			src: &domain.OutboxEvent{
				ID:          id,
				AggregateID: aggregateID,
				EventType:   domain.OutboxEventType("order.created"),
				Payload:     payload,
				CreatedAt:   createdAt,
				PublishedAt: &publishedAt,
			},
			want: &OutboxEvent{
				ID:          id,
				AggregateID: aggregateID,
				EventType:   "order.created",
				Payload: json.RawMessage(`{
					"id":"` + id.String() + `",
					"total_cents":12500,
					"items":[{
						"id":"` + itemID.String() + `",
						"item_name":"Coffee",
						"quantity":2,
						"unit_price_cents":6250
					}]
				}`),
				CreatedAt:   createdAt,
				PublishedAt: &publishedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewOutboxEvent(tt.src)

			require.Equal(t, tt.want.ID, got.ID)
			require.Equal(t, tt.want.AggregateID, got.AggregateID)
			require.Equal(t, tt.want.EventType, got.EventType)
			require.Equal(t, tt.want.CreatedAt, got.CreatedAt)
			require.Equal(t, tt.want.PublishedAt, got.PublishedAt)

			var gotPayload domain.OutboxEventPayload
			require.NoError(t, json.Unmarshal(got.Payload, &gotPayload))
			require.Equal(t, tt.src.Payload, gotPayload)
		})
	}
}

func TestOutboxEvent_ToDomain(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	aggregateID := uuid.New()
	itemID := uuid.New()

	createdAt := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	payload := domain.OutboxEventPayload{
		ID:         id,
		TotalCents: 12500,
		Items: []domain.OutboxEventItem{
			{
				ID:             itemID,
				ItemName:       "Coffee",
				Quantity:       2,
				UnitPriceCents: 6250,
			},
		},
	}

	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	src := &OutboxEvent{
		ID:          id,
		AggregateID: aggregateID,
		EventType:   "order.created",
		Payload:     payloadJSON,
		CreatedAt:   createdAt,
		PublishedAt: &publishedAt,
	}

	want := &domain.OutboxEvent{
		ID:          id,
		AggregateID: aggregateID,
		EventType:   domain.OutboxEventType("order.created"),
		Payload:     payload,
		CreatedAt:   createdAt,
		PublishedAt: &publishedAt,
	}

	got := src.ToDomain()

	require.Equal(t, want, got)
}
