package usecase

import (
	"context"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
}

type OrderService struct {
	orders OrderRepository
}

func NewOrderService(orders OrderRepository) *OrderService {
	return &OrderService{orders: orders}
}

func (s *OrderService) CreateOrder(
	ctx context.Context,
	items []domain.OrderItem,
) (domain.Order, error) {
	order, err := domain.NewOrder(items)
	if err != nil {
		return domain.Order{}, err
	}

	if err := s.orders.Create(ctx, &order); err != nil {
		return domain.Order{}, err
	}

	return order, nil
}
