package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

const (
	createOrderQuery = `
		INSERT INTO orders (total_cents) VALUES ($1) RETURNING id
	`
	createOrderItemQuery = `
		INSERT INTO order_items (
			order_id,
			item_name,
			quantity,
			unit_price_cents
		)
		VALUES ($1, $2, $3, $4)
	`
	createOutboxEventQuery = `
		INSERT INTO outbox_events (
			aggregate_id,
			event_type,
			payload
		)
		VALUES ($1, $2, $3)
	`
)

type OrderRepository struct {
	db *pgxpool.Pool
}

var _ usecase.OrderRepository = (*OrderRepository)(nil)

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(ctx, createOrderQuery, order.TotalCents).Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	for _, item := range order.Items {
		_, err = tx.Exec(
			ctx,
			createOrderItemQuery,
			order.ID,
			item.ItemName,
			item.Quantity,
			item.UnitPriceCents,
		)
		if err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}
	}

	payload, err := json.Marshal(domain.OutboxEventPayload{
		ID:         order.ID,
		TotalCents: order.TotalCents,
		Items:      order.Items,
	})
	if err != nil {
		return fmt.Errorf("marshal order created event: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		createOutboxEventQuery,
		order.ID,
		domain.OrderCreatedEventType,
		payload,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
