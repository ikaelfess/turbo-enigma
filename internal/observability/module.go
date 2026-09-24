package observability

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"observability",

	fx.Provide(
		NewLogger,
		NewTelemetry,
	),

	fx.Invoke(func(*Telemetry) {}),
)
