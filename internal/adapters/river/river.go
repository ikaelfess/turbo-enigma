package riveradapter

import (
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/ikaelfess/transactional-outbox/internal/config"
)

func NewRiverClient(
	config config.Config,
	db *pgxpool.Pool,
	workers *river.Workers,
	periodicJobs []*river.PeriodicJob,
) (*river.Client[pgx.Tx], error) {
	return river.NewClient(
		riverpgxv5.New(db),
		&river.Config{
			Queues: map[string]river.QueueConfig{
				river.QueueDefault: {MaxWorkers: config.WorkersNum},
			},
			Workers:      workers,
			PeriodicJobs: periodicJobs,
			Logger:       slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		},
	)
}
