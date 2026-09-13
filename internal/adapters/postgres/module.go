package postgres

import (
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var PoolModule = fx.Module(
	"postgres-pool",

	fx.Provide(NewPool),
)

var OrderRepositoryModule = fx.Module(
	"postgres-order-repository",

	fx.Provide(
		fx.Annotate(
			NewOrderRepository,
			fx.As(new(usecase.OrderRepository)),
		),
	),
)

var OutboxEventRepositoryModule = fx.Module(
	"postgres-outbox-event-repository",

	fx.Provide(
		NewOutboxEventRepository,
	),
)
