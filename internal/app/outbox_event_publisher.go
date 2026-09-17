package app

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	riveradapter "github.com/ikaelfess/transactional-outbox/internal/adapters/river"
	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/observability"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var OutboxEventPublisherModule = fx.Module(
	"outbox-event-publisher",

	config.Module,
	postgres.Module,
	usecase.Module,
	observability.Module,
	riveradapter.Module,
	fx.Provide(
		LoggerConfig,
		RegisterWorkers,
	),

	fx.Invoke(StartRiverClient),
)

func LoggerConfig() observability.LoggerConfig {
	return observability.LoggerConfig{
		ServiceName: "outbox-event-publisher",
		Level:       "debug",
	}
}

func RegisterWorkers(worker *riveradapter.OutboxEventPublisherWorker) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)

	return workers
}

func StartRiverClient(
	lifecycle fx.Lifecycle,
	client *river.Client[pgx.Tx],
) {
	lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			if err := client.Start(context.Background()); err != nil {
				return err
			}

			return nil
		},

		OnStop: client.Stop,
	})
}
