package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	httpadapter "github.com/ikaelfess/transactional-outbox/internal/adapters/http"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

func WithApiServer(
	t *testing.T,
	db *bun.DB,
	fn func(t *testing.T, apiServerUrl string),
) {
	var handler http.Handler
	app := fxtest.New(
		t,
		fx.NopLogger,

		postgres.StoreModule,
		usecase.Module,
		httpadapter.Module,

		fx.Provide(
			func() zerolog.Logger { return zerolog.Nop() },
			func() *bun.DB { return db },
			fx.Annotate(postgres.NewOrderStore, fx.As(new(usecase.OrderStore))),
			fx.Annotate(usecase.NewOrderUsecase, fx.As(new(httpadapter.OrderUsecase))),
		),

		fx.Populate(&handler),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	fn(t, server.URL)
}
