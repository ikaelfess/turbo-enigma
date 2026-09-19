package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/extra/bunotel"
	"github.com/uptrace/bun/extra/bunzerolog"
	"go.opentelemetry.io/otel"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/observability"
)

const (
	SlowQueryThreshold = 2 * time.Second
)

type databaseParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    config.Config
	Logger    zerolog.Logger
	Telemetry *observability.Telemetry `optional:"true"`
}

func NewDatabase(p databaseParams) *bun.DB {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(p.Config.DatabaseUrl)))
	sqldb.SetMaxOpenConns(p.Config.DBMaxConns)
	sqldb.SetConnMaxLifetime(p.Config.DBMaxConnLifetime)

	loggerHook := bunzerolog.NewQueryHook(
		bunzerolog.WithLogger(&p.Logger),
		bunzerolog.WithQueryLogLevel(zerolog.DebugLevel),
		bunzerolog.WithSlowQueryLogLevel(zerolog.WarnLevel),
		bunzerolog.WithErrorQueryLogLevel(zerolog.ErrorLevel),
		bunzerolog.WithSlowQueryThreshold(SlowQueryThreshold),
	)

	otelHookOpts := []bunotel.Option{
		bunotel.WithDBName(databaseName(p.Config.DatabaseUrl)),
	}
	if p.Telemetry != nil {
		otelHookOpts = append(otelHookOpts, bunotel.WithTracerProvider(otel.GetTracerProvider()))
	}

	db := bun.
		NewDB(sqldb, pgdialect.New()).
		WithQueryHook(loggerHook).
		WithQueryHook(bundebug.NewQueryHook(bundebug.FromEnv("BUNDEBUG"))).
		WithQueryHook(bunotel.NewQueryHook(otelHookOpts...))

	p.Lifecycle.Append(fx.Hook{
		OnStart: db.PingContext,
		OnStop: func(_ context.Context) error {
			return db.Close()
		},
	})

	return db
}

func databaseName(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return ""
	}

	return strings.TrimPrefix(parsed.Path, "/")
}
