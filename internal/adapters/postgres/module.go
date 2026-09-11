package postgres

import (
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var PoolModule = fx.Module(
	"postgres-pool",

	fx.Provide(NewPool),
)

var RepositoryModule = fx.Module(
	"postgres-repository",

	fx.Provide(
		fx.Annotate(
			NewOrderRepository,
			fx.As(new(usecase.OrderRepository)),
		),
	),
)

var Module = fx.Module(
	"postgres",

	PoolModule,
	RepositoryModule,
)
