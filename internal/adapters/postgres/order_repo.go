package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

type OrderRepo struct {
	db bun.IDB
}

func NewOrderRepo(db bun.IDB) *OrderRepo {
	return &OrderRepo{db: db}
}

func (o *OrderRepo) Create(ctx context.Context, order domain.Order) (domain.Order, error) {
	orderModel := Order{TotalCents: order.TotalCents}
	_, err := o.db.NewInsert().
		Model(&orderModel).
		Exec(ctx)
	if err != nil {
		return domain.Order{}, fmt.Errorf("create order: %w", err)
	}

	return domain.Order{
		ID:         orderModel.ID,
		TotalCents: orderModel.TotalCents,
	}, nil
}
