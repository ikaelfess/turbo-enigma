package postgres

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"postgres",

	StoreModule,
	fx.Provide(
		NewDatabase,
	),
)

var StoreModule = fx.Module(
	"postgres-store",

	fx.Provide(
		NewOrderStore,
	),
)
