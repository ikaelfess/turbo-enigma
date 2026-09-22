package river

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
)

func NewRiverClient(
	lifecycle fx.Lifecycle,
	cfg config.Config,
	workers *river.Workers,
	periodicJobs []*river.PeriodicJob,
) (*river.Client[pgx.Tx], error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseUrl)
	if err != nil {
		return nil, fmt.Errorf("parse river database url: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.DBMaxConns)
	poolConfig.MinConns = int32(cfg.DBMinConns)
	poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("river pgx pool: %w", err)
	}

	client, err := river.NewClient(
		riverpgxv5.New(pool),
		&river.Config{
			Queues: map[string]river.QueueConfig{
				river.QueueDefault: {MaxWorkers: cfg.WorkersNum},
			},
			Workers:      workers,
			PeriodicJobs: periodicJobs,
			Logger:       slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		},
	)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("river client: %w", err)
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return client, nil
}
