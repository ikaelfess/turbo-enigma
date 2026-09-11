package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
)

type Config struct {
	URL             string
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
}

func NewPool(
	lifecycle fx.Lifecycle,
	config config.Config,
	logger zerolog.Logger,
) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseUrl)
	if err != nil {
		return nil, fmt.Errorf("postgres config: %w", err)
	}

	poolConfig.ConnConfig.Tracer = &queryTracer{logger: logger}
	poolConfig.MaxConns = int32(config.DBMaxConns)
	poolConfig.MinConns = int32(config.DBMinConns)
	poolConfig.MaxConnLifetime = config.DBMaxConnLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

type queryTracer struct {
	logger zerolog.Logger
}

var _ pgx.QueryTracer = (*queryTracer)(nil)

type queryTraceData struct {
	start time.Time
	sql   string
	args  []any
}

type queryTraceKey struct{}

func (t *queryTracer) TraceQueryStart(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	return context.WithValue(
		ctx,
		queryTraceKey{},
		queryTraceData{
			start: time.Now(),
			sql:   data.SQL,
			args:  data.Args,
		},
	)
}

func (t *queryTracer) TraceQueryEnd(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryEndData,
) {
	traceData, ok := ctx.Value(queryTraceKey{}).(queryTraceData)
	if !ok {
		return
	}

	event := t.logger.Debug().
		Str("sql", strings.Join(strings.Fields(traceData.sql), " ")).
		Float64("duration_ms", float64(time.Since(traceData.start).Microseconds())/1000)

	if data.Err != nil {
		event = t.logger.Error().
			Str("sql", strings.Join(strings.Fields(traceData.sql), " ")).
			Float64("duration_ms", float64(time.Since(traceData.start).Microseconds())/1000).
			Err(data.Err)
	}

	event.Msg("postgres query")
}
