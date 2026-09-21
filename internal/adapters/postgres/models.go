package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type Order struct {
	bun.BaseModel `bun:"table:orders"`

	ID         uuid.UUID `bun:"id,pk,type:uuid,default:uuidv7()"`
	TotalCents int64     `bun:"total_cents,notnull"`
	CreatedAt  time.Time `bun:"created_at,notnull,default:now()"`

	Items []*OrderItem `bun:"rel:has-many,join:id=order_id"`
}

func NewOrder(order *domain.Order) *Order {
	o := &Order{}
	_ = copier.CopyWithOption(o, order, copier.Option{IgnoreEmpty: true, DeepCopy: true})

	return o
}

func (o *Order) ToDomain() *domain.Order {
	order := &domain.Order{}
	_ = copier.CopyWithOption(order, o, copier.Option{IgnoreEmpty: true, DeepCopy: true})

	return order
}

type OrderItem struct {
	bun.BaseModel `bun:"table:order_items,alias:oi"`

	ID             uuid.UUID `bun:"id,pk,type:uuid,default:uuidv7()"`
	OrderID        uuid.UUID `bun:"order_id,type:uuid,notnull"`
	ItemName       string    `bun:"item_name,notnull"`
	Quantity       int       `bun:"quantity,notnull"`
	UnitPriceCents int64     `bun:"unit_price_cents,notnull"`
	CreatedAt      time.Time `bun:"created_at,notnull,default:now()"`

	Order *Order `bun:"rel:belongs-to,join:order_id=id"`
}

func NewOrderItem(orderItem *domain.OrderItem) *OrderItem {
	i := &OrderItem{}
	_ = copier.CopyWithOption(i, orderItem, copier.Option{IgnoreEmpty: true})

	return i
}

func (i *OrderItem) ToDomain() *domain.OrderItem {
	orderItem := &domain.OrderItem{}
	_ = copier.CopyWithOption(orderItem, i, copier.Option{IgnoreEmpty: true})

	return orderItem
}

type OutboxEvent struct {
	bun.BaseModel `bun:"table:outbox_events"`

	ID           uuid.UUID       `bun:"id,pk,type:uuid,default:uuidv7()"`
	AggregateID  uuid.UUID       `bun:"aggregate_id,type:uuid,notnull"`
	EventType    string          `bun:"event_type,notnull"`
	Payload      json.RawMessage `bun:"payload,type:jsonb,notnull"`
	CreatedAt    time.Time       `bun:"created_at,notnull,default:now()"`
	PublishedAt  *time.Time      `bun:"published_at"`
	ClaimedUntil *time.Time      `bun:"claimed_until"`
}

func NewOutboxEvent(outboxEvent *domain.OutboxEvent) *OutboxEvent {
	event := &OutboxEvent{}
	_ = copier.CopyWithOption(
		event,
		outboxEvent,
		copier.Option{
			IgnoreEmpty: true,
			Converters: []copier.TypeConverter{
				{
					SrcType: domain.OutboxEventPayload{},
					DstType: json.RawMessage{},
					Fn: func(src any) (dst any, err error) {
						return json.Marshal(src)
					},
				},
			},
		},
	)

	return event
}

func (e *OutboxEvent) ToDomain() *domain.OutboxEvent {
	outboxEvent := &domain.OutboxEvent{}
	_ = copier.CopyWithOption(
		outboxEvent,
		e,
		copier.Option{
			IgnoreEmpty: true,
			Converters: []copier.TypeConverter{
				{
					SrcType: json.RawMessage{},
					DstType: domain.OutboxEventPayload{},
					Fn: func(src any) (any, error) {
						var payload domain.OutboxEventPayload
						err := json.Unmarshal(src.(json.RawMessage), &payload)

						return payload, err
					},
				},
			},
		},
	)

	return outboxEvent
}
