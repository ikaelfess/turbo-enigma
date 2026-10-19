package kafka

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

// ErrUndecodable marks a record that can never succeed on retry.
var ErrUndecodable = errors.New("undecodable record")

// DecodeRecord is the inverse of Producer.Publish.
func DecodeRecord(record *kgo.Record) (domain.OutboxEvent, error) {
	eventID, err := uuid.Parse(headerValue(record, "event_id"))
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("%w: event_id header: %w", ErrUndecodable, err)
	}

	eventType := headerValue(record, "event_type")
	if eventType == "" {
		return domain.OutboxEvent{}, fmt.Errorf("%w: missing event_type header", ErrUndecodable)
	}

	aggregateID, err := uuid.ParseBytes(record.Key)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("%w: key: %w", ErrUndecodable, err)
	}

	var payload domain.OutboxEventPayload
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("%w: value: %w", ErrUndecodable, err)
	}

	return domain.OutboxEvent{
		ID:          eventID,
		AggregateID: aggregateID,
		EventType:   domain.OutboxEventType(eventType),
		Payload:     payload,
		CreatedAt:   record.Timestamp,
	}, nil
}

func headerValue(record *kgo.Record, key string) string {
	for _, header := range record.Headers {
		if header.Key == key {
			return string(header.Value)
		}
	}

	return ""
}
