package observability

import (
	"github.com/uptrace/uptrace-go/uptrace"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
)

type Telemetry struct{}

func NewTelemetry(lc fx.Lifecycle, cfg config.Config, loggerConfig LoggerConfig) *Telemetry {
	uptrace.ConfigureOpentelemetry(
		uptrace.WithDSN(cfg.UptraceDSN),
		uptrace.WithServiceName(loggerConfig.ServiceName),
		uptrace.WithDeploymentEnvironment("local"),
		uptrace.WithMetricsDisabled(),
		uptrace.WithLoggingDisabled(),
	)

	lc.Append(fx.Hook{OnStop: uptrace.Shutdown})

	return &Telemetry{}
}
