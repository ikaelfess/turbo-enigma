package app

import (
	"go.uber.org/fx"

	kafkaadapter "github.com/ikaelfess/transactional-outbox/internal/adapters/kafka"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/observability"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var OutboxEventConsumerModule = fx.Module(
	"outbox-event-consumer",

	config.Module,
	postgres.Module,
	observability.Module,

	fx.Provide(
		ConsumerLoggerConfig,

		fx.Annotate(postgres.NewConsumedEventRepo, fx.As(new(usecase.ConsumedEventRepo))),
		fx.Annotate(usecase.NewConsumedEventUsecase, fx.As(new(kafkaadapter.Handler))),
	),

	fx.Invoke(kafkaadapter.RegisterConsumer),
)

func ConsumerLoggerConfig() observability.LoggerConfig {
	return observability.LoggerConfig{
		ServiceName: "outbox-event-consumer",
		Level:       "debug",
	}
}
