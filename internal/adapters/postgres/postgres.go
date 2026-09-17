package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/extra/bunzerolog"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
)

const (
	SlowQueryThreshold = 2 * time.Second
)

func NewDatabase(lc fx.Lifecycle, c config.Config, logger zerolog.Logger) *bun.DB {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(c.DatabaseUrl)))
	sqldb.SetMaxOpenConns(c.DBMaxConns)
	sqldb.SetConnMaxLifetime(c.DBMaxConnLifetime)

	loggerHook := bunzerolog.NewQueryHook(
		bunzerolog.WithLogger(&logger),
		bunzerolog.WithQueryLogLevel(zerolog.DebugLevel),
		bunzerolog.WithSlowQueryLogLevel(zerolog.WarnLevel),
		bunzerolog.WithErrorQueryLogLevel(zerolog.ErrorLevel),
		bunzerolog.WithSlowQueryThreshold(SlowQueryThreshold),
	)

	db := bun.
		NewDB(sqldb, pgdialect.New()).
		WithQueryHook(loggerHook).
		WithQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))

	lc.Append(fx.Hook{
		OnStart: db.PingContext,
		OnStop: func(_ context.Context) error {
			return db.Close()
		},
	})

	return db
}
