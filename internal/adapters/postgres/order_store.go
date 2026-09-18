package postgres

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OrderStore struct {
	db *bun.DB
}

func NewOrderStore(db *bun.DB) *OrderStore {
	return &OrderStore{db: db}
}

func (s *OrderStore) Create(
	ctx context.Context,
	order domain.Order,
	items []domain.OrderItem,
) (domain.Order, error) {
	var err error
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		orderRepo := NewOrderRepo(tx)
		orderItemRepo := NewOrderItemRepo(tx)
		outboxEventRepo := NewOutboxEventRepo(tx)

		order, err = orderRepo.Create(ctx, order)
		if err != nil {
			return err
		}

		items, err = orderItemRepo.InsertBatch(ctx, order.ID, items)
		if err != nil {
			return err
		}

		payloadItems := make([]domain.OutboxEventItem, 0, len(items))
		for _, item := range items {
			payloadItems = append(payloadItems, domain.OutboxEventItem{
				ID:             item.ID,
				ItemName:       item.ItemName,
				Quantity:       item.Quantity,
				UnitPriceCents: item.UnitPriceCents,
			})
		}

		outboxEvent := domain.OutboxEvent{
			AggregateID: order.ID,
			EventType:   domain.OrderCreatedEventType,
			Payload: domain.OutboxEventPayload{
				ID:         order.ID,
				TotalCents: order.TotalCents,
				Items:      payloadItems,
			},
		}

		_, err = outboxEventRepo.Create(ctx, outboxEvent)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return domain.Order{}, err
	}

	return order, nil
}
