package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfig_requiresUptraceDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://outbox:outbox@postgres:5432/outbox?sslmode=disable")
	require.NoError(t, os.Unsetenv("UPTRACE_DSN"))

	_, err := NewConfig()

	require.Error(t, err)
	require.ErrorContains(t, err, "is required")
}

func TestNewConfig_readsUptraceDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://outbox:outbox@postgres:5432/outbox?sslmode=disable")
	t.Setenv("UPTRACE_DSN", "http://project1_secret@uptrace:80?grpc=4317")

	cfg, err := NewConfig()

	require.NoError(t, err)
	require.Equal(t, "http://project1_secret@uptrace:80?grpc=4317", cfg.UptraceDSN)
}
