package helpers

import (
	"context"
	"testing"

	"github.com/andrei-polukhin/pgdbtemplate"
	pgdbtemplatepgx "github.com/andrei-polukhin/pgdbtemplate-pgx"
	"github.com/go-openapi/testify/v2/require"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func WithDatabase(
	t *testing.T,
	tm *pgdbtemplate.TemplateManager,
	fn func(t *testing.T, db *bun.DB),
) {
	t.Helper()

	database, databaseName, err := tm.CreateTestDatabase(t.Context())
	require.NoError(t, err, "template manager create test database")

	t.Cleanup(func() {
		require.NoError(
			t,
			tm.DropTestDatabase(context.Background(), databaseName),
			"template manager drop test database",
		)
	})

	t.Cleanup(func() {
		require.NoError(
			t,
			database.Close(),
			"database close",
		)
	})

	db, ok := database.(*pgdbtemplatepgx.DatabaseConnection)
	require.True(t, ok, "expected pgx database connection")
	bunDB := bun.NewDB(stdlib.OpenDBFromPool(db.Pool), pgdialect.New())
	t.Cleanup(func() { bunDB.Close(); db.Close() })

	fn(t, bunDB)
}
