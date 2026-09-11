package helpers

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	DatabaseImage    = "postgres:18.4-alpine"
	DatabaseName     = "transactional_outbox_test"
	DatabaseUser     = "test"
	DatabasePassword = "password"
)

func WithDatabaseContainer(t *testing.T, fn func(t *testing.T, connString string)) {
	t.Helper()

	container, err := postgres.Run(
		t.Context(),
		DatabaseImage,
		postgres.WithDatabase(DatabaseName),
		postgres.WithUsername(DatabaseUser),
		postgres.WithPassword(DatabasePassword),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err, "postgres container run")

	t.Cleanup(func() {
		require.NoError(
			t,
			testcontainers.TerminateContainer(container),
			"postgres container terminate",
		)
	})

	connectionString, err := container.ConnectionString(t.Context(), "sslmode=disable")
	require.NoError(t, err, "database container connection string")

	fn(t, connectionString)
}
