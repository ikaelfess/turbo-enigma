package riveradapter

import "go.uber.org/fx"

var Module = fx.Module(
	"river-client",

	fx.Provide(
		NewRiverClient,
		NewOutboxEventPublisherWorker,
		CreatePeriodicJobs,
	),
)
