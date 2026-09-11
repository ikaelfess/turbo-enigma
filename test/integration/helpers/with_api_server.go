package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	apphttp "github.com/ikaelfess/transactional-outbox/internal/adapters/http"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

func WithApiServer(
	t *testing.T,
	db *pgxpool.Pool,
	fn func(t *testing.T, apiServerUrl string),
) {
	var handler http.Handler
	app := fxtest.New(
		t,
		fx.NopLogger,

		postgres.RepositoryModule,
		usecase.Module,
		apphttp.Module,

		fx.Provide(
			func() zerolog.Logger { return zerolog.Nop() },
			func() *pgxpool.Pool { return db },
		),

		fx.Populate(&handler),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	fn(t, server.URL)
}
