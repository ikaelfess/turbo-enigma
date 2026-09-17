package domain

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEventType string

const (
	OrderCreatedEventType OutboxEventType = "order.created"
)

type OutboxEventItem struct {
	ID             uuid.UUID `json:"id"`
	ItemName       string    `json:"item_name"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
}

type OutboxEventPayload struct {
	ID         uuid.UUID         `json:"id"`
	TotalCents int64             `json:"total_cents"`
	Items      []OutboxEventItem `json:"items"`
}

type OutboxEvent struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	EventType   OutboxEventType
	Payload     OutboxEventPayload
	CreatedAt   time.Time
	PublishedAt *time.Time
}
