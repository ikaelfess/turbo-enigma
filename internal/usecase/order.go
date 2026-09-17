package usecase

import (
	"context"
	"fmt"

	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

var _ OrderStore = (*postgres.OrderStore)(nil)

type OrderStore interface {
	Create(ctx context.Context, order domain.Order, items []domain.OrderItem) (domain.Order, error)
}

type OrderUsecase struct {
	orders OrderStore
}

func NewOrderUsecase(orders OrderStore) *OrderUsecase {
	return &OrderUsecase{orders: orders}
}

func (s *OrderUsecase) CreateOrder(
	ctx context.Context,
	items []domain.OrderItem,
) (domain.Order, error) {
	order, err := domain.NewOrder(items)
	if err != nil {
		return domain.Order{}, err
	}

	order, err = s.orders.Create(ctx, order, items)
	if err != nil {
		return domain.Order{}, fmt.Errorf("%w: %w", domain.ErrOrderNotSaved, err)
	}

	return order, nil
}
