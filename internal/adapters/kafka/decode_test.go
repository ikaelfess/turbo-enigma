package kafka

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

func TestDecodeRecord(t *testing.T) {
	t.Parallel()

	eventID := uuid.New()
	aggregateID := uuid.New()
	itemID := uuid.New()
	timestamp := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	valid := func() *kgo.Record {
		return &kgo.Record{
			Key:       []byte(aggregateID.String()),
			Value:     []byte(`{"id":"` + aggregateID.String() + `","total_cents":300,"items":[{"id":"` + itemID.String() + `","item_name":"Widget","quantity":3,"unit_price_cents":100}]}`),
			Timestamp: timestamp,
			Headers: []kgo.RecordHeader{
				{Key: "event_id", Value: []byte(eventID.String())},
				{Key: "event_type", Value: []byte("order.created")},
			},
		}
	}

	t.Run("valid record", func(t *testing.T) {
		t.Parallel()

		event, err := DecodeRecord(valid())
		require.NoError(t, err)
		require.Equal(t, domain.OutboxEvent{
			ID:          eventID,
			AggregateID: aggregateID,
			EventType:   domain.OrderCreatedEventType,
			CreatedAt:   timestamp,
			Payload: domain.OutboxEventPayload{
				ID:         aggregateID,
				TotalCents: 300,
				Items: []domain.OutboxEventItem{{
					ID:             itemID,
					ItemName:       "Widget",
					Quantity:       3,
					UnitPriceCents: 100,
				}},
			},
		}, event)
	})

	tests := []struct {
		name   string
		mutate func(*kgo.Record)
	}{
		{"missing event_id", func(r *kgo.Record) { r.Headers = r.Headers[1:] }},
		{"bad event_id", func(r *kgo.Record) { r.Headers[0].Value = []byte("nope") }},
		{"missing event_type", func(r *kgo.Record) { r.Headers = r.Headers[:1] }},
		{"bad key", func(r *kgo.Record) { r.Key = []byte("nope") }},
		{"nil key", func(r *kgo.Record) { r.Key = nil }},
		{"bad json", func(r *kgo.Record) { r.Value = []byte("{") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			record := valid()
			tt.mutate(record)

			_, err := DecodeRecord(record)
			require.ErrorIs(t, err, ErrUndecodable)
		})
	}
}
