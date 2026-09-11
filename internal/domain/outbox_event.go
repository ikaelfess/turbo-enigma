package domain

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEventType string

const (
	OrderCreatedEventType OutboxEventType = "order.created"
)

type OutboxEventPayload struct {
	ID         uuid.UUID   `json:"id"`
	TotalCents int64       `json:"total_cents"`
	Items      []OrderItem `json:"items"`
}

type OutboxEvent struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	EventType   OutboxEventType
	Payload     OutboxEventPayload
	CreatedAt   time.Time
	PublishedAt *time.Time
}
