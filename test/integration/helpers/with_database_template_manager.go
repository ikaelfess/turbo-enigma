package helpers

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrei-polukhin/pgdbtemplate"
	pgdbtemplategoose "github.com/andrei-polukhin/pgdbtemplate-goose"
	pgdbtemplatepgx "github.com/andrei-polukhin/pgdbtemplate-pgx"
	"github.com/go-openapi/testify/v2/require"
	"github.com/pressly/goose/v3"
)

const (
	MigrationsDir = "migrations"
)

func WithDatabaseTemplateManager(
	t *testing.T,
	connectionString string,
	fn func(t *testing.T, tm *pgdbtemplate.TemplateManager),
) {
	t.Helper()

	connectionStringFunc := func(dbName string) string {
		return pgdbtemplate.ReplaceDatabaseInConnectionString(connectionString, dbName)
	}

	connectionProvider := pgdbtemplatepgx.NewConnectionProvider(connectionStringFunc)
	t.Cleanup(connectionProvider.Close)

	migrationRunner := pgdbtemplategoose.NewMigrationRunner(
		migrationsFS(t),
		pgdbtemplategoose.WithDialect(goose.DialectPostgres),
	)
	config := pgdbtemplate.Config{
		ConnectionProvider: connectionProvider,
		MigrationRunner:    migrationRunner,
	}

	tm, err := pgdbtemplate.NewTemplateManager(config)
	require.NoError(t, err, "template manager new")
	require.NoError(t, tm.Initialize(t.Context()), "template manager initialize")

	t.Cleanup(func() {
		require.NoError(
			t,
			tm.Cleanup(context.Background()),
			"template manager cleanup",
		)
	})

	fn(t, tm)
}

func migrationsFS(t *testing.T) fs.FS {
	t.Helper()

	return os.DirFS(filepath.Join(FindProjectRoot(t), MigrationsDir))
}
