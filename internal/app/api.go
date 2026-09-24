package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	httpadapter "github.com/ikaelfess/transactional-outbox/internal/adapters/http"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/observability"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var ApiModule = fx.Module(
	"api",

	config.Module,
	observability.Module,
	httpadapter.Module,
	usecase.Module,
	postgres.Module,

	fx.Provide(
		NewLoggerConfig,

		fx.Annotate(postgres.NewOrderStore, fx.As(new(usecase.OrderStore))),
		fx.Annotate(usecase.NewOrderUsecase, fx.As(new(httpadapter.OrderUsecase))),
	),

	fx.Invoke(NewHTTPServer),
)

func NewLoggerConfig() observability.LoggerConfig {
	return observability.LoggerConfig{
		ServiceName: "api",
		Level:       "debug",
	}
}

func NewHTTPServer(
	lifecycle fx.Lifecycle,
	logger zerolog.Logger,
	handler http.Handler,
	config config.Config,
) *http.Server {
	server := &http.Server{
		Handler:      handler,
		Addr:         config.ServerAddress,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info().
					Str("address", server.Addr).
					Msg("http server started")

				err := server.ListenAndServe()
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error().
						Err(err).
						Msg("http server failed")
				}
			}()

			return nil
		},

		OnStop: func(ctx context.Context) error {
			logger.Info().Msg("http server stopping")

			shutdownCtx, cancel := context.WithTimeout(ctx, config.ShutdownTimeout)
			defer cancel()

			return server.Shutdown(shutdownCtx)
		},
	})

	return server
}
