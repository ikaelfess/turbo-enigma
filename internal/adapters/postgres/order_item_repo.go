package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OrderItemRepo struct {
	db bun.IDB
}

func NewOrderItemRepo(db bun.IDB) *OrderItemRepo {
	return &OrderItemRepo{db: db}
}

func (i *OrderItemRepo) InsertBatch(
	ctx context.Context,
	orderID uuid.UUID,
	items []domain.OrderItem,
) ([]domain.OrderItem, error) {
	if len(items) == 0 {
		return []domain.OrderItem{}, nil
	}

	itemModels := make([]OrderItem, 0, len(items))
	for _, item := range items {
		item.OrderID = orderID
		itemModels = append(itemModels, *NewOrderItem(&item))
	}

	_, err := i.db.
		NewInsert().
		Model(&itemModels).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert order items: %w", err)
	}

	for i, itemModel := range itemModels {
		items[i] = *itemModel.ToDomain()
	}

	return items, nil
}
